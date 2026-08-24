package store

import (
	"errors"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) studentHomeUnlocked(principal learning.Principal) (learning.StudentHome, error) {
	if principal.StudentID == "" {
		return learning.StudentHome{}, errors.New("student account is not bound")
	}
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return learning.StudentHome{}, errors.New("student not found")
	}
	if student.AccountStatus == "停用" {
		return learning.StudentHome{}, errors.New("账号已停用，请联系老师或管理员")
	}
	courses := s.coursesForStudent(student.ID)
	materials := s.studentMaterialsForPrincipal(principal)
	pendingHomework := s.pendingHomeworkForPrincipal(principal)
	continueCourse := learning.Course{}
	if len(courses) > 0 {
		continueCourse = courses[0]
	}
	if len(materials) == 0 {
		materials = []learning.Material{}
	}
	if len(pendingHomework) == 0 {
		pendingHomework = []learning.Homework{}
	}
	notices := s.noticesForStudent(student)
	feedback := s.classroomFeedbackForStudent(student.ID)
	subscriptionReminder := s.subscriptionReminder(student.ID)
	return learning.StudentHome{
		Student:              student,
		ContinueCourse:       continueCourse,
		ContinueProgress:     s.courseProgress(student.ID, continueCourse.ID),
		PendingHomework:      pendingHomework,
		Notices:              notices,
		Materials:            materials,
		TodayTodos:           s.todayTodosForStudent(student, pendingHomework, feedback, subscriptionReminder),
		ClassroomFeedback:    feedback,
		SubscriptionReminder: subscriptionReminder,
	}, nil
}

// StudentRecommendations 返回学生当前可了解、但尚未有效开通的套餐。
func (s *MemoryStore) studentRecommendationsUnlocked(principal learning.Principal) ([]learning.StudentPackageRecommendation, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return nil, errors.New("student not found")
	}
	if student.AccountStatus == "停用" {
		return nil, errors.New("账号已停用，请联系老师或管理员")
	}

	activeSpaceIDs := s.studentAccessibleSpaceIDs(student.ID)
	activeSpaces := make([]learningSpace, 0, len(activeSpaceIDs))
	for _, spaceID := range activeSpaceIDs {
		for _, space := range s.learningSpaces {
			if space.ID == spaceID {
				activeSpaces = append(activeSpaces, space)
				break
			}
		}
	}
	if len(activeSpaces) == 0 {
		return []learning.StudentPackageRecommendation{}, nil
	}
	openedCourseIDs := map[string]bool{}
	for _, course := range s.coursesForStudent(student.ID) {
		openedCourseIDs[course.ID] = true
	}
	openedMaterialIDs := map[string]bool{}
	for _, material := range s.materialsForStudent(student.ID) {
		openedMaterialIDs[material.ID] = true
	}

	out := make([]learning.StudentPackageRecommendation, 0)
	for _, pkg := range s.packages {
		if pkg.Status != learning.StatusEnabled || !containsRecommendationContent(s.contentTypesForPackage(pkg.ID)) {
			continue
		}
		if opened, _, _ := s.activeGrantState(student.ID, pkg.ID); opened {
			continue
		}
		courses, materials := s.recommendationContentForPackage(pkg, openedCourseIDs, openedMaterialIDs)
		if len(courses) == 0 && len(materials) == 0 {
			continue
		}
		sameSpace := overlaps(s.learningSpaceIDsForPackage(pkg.ID), activeSpaceIDs)
		sameTerm := sameRecommendationTerm(pkg, activeSpaces)
		if !sameSpace && !sameTerm {
			continue
		}
		reason := "同年级同学期推荐"
		if sameSpace {
			reason = "同学习空间推荐"
		}
		out = append(out, learning.StudentPackageRecommendation{
			PackageID:            pkg.ID,
			PackageName:          pkg.Name,
			AcademicYear:         pkg.AcademicYear,
			Grade:                pkg.Grade,
			Semester:             pkg.Semester,
			Subject:              pkg.Subject,
			Summary:              pkg.Summary,
			LearningSpaces:       s.learningSpaceNamesForPackage(pkg.ID),
			CourseCount:          len(courses),
			MaterialCount:        len(materials),
			ContentSamples:       recommendationSamples(courses, materials),
			RecommendationReason: reason,
			SameLearningSpace:    sameSpace,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SameLearningSpace != out[j].SameLearningSpace {
			return out[i].SameLearningSpace
		}
		leftCount := out[i].CourseCount + out[i].MaterialCount
		rightCount := out[j].CourseCount + out[j].MaterialCount
		if leftCount != rightCount {
			return leftCount > rightCount
		}
		return out[i].PackageName < out[j].PackageName
	})
	if len(out) > 3 {
		out = out[:3]
	}
	return out, nil
}

func (s *MemoryStore) recommendationContentForPackage(pkg learning.Package, openedCourseIDs, openedMaterialIDs map[string]bool) ([]string, []string) {
	courses := make([]string, 0)
	materials := make([]string, 0)
	spaceIDs := s.learningSpaceIDsForPackage(pkg.ID)
	contentTypes := s.contentTypesForPackage(pkg.ID)
	if containsString(contentTypes, "course") {
		for _, course := range s.courses {
			if course.Status == learning.StatusEnabled && containsString(spaceIDs, course.LearningSpaceID) && !openedCourseIDs[course.ID] {
				courses = appendUnique(courses, course.Name)
			}
		}
	}
	if containsString(contentTypes, "handout") {
		for _, material := range s.materials {
			if materialPublished(material.Status) && containsString(spaceIDs, material.LearningSpaceID) && !openedMaterialIDs[material.ID] {
				materials = appendUnique(materials, material.Title)
			}
		}
	}
	return courses, materials
}

func containsRecommendationContent(contentTypes []string) bool {
	return containsString(contentTypes, "course") || containsString(contentTypes, "handout")
}

// sameRecommendationTerm 不比较学年，理由同 learningSpaceMatches：
// 学习空间是跨学年复用的目录，学年只属于套餐。
func sameRecommendationTerm(pkg learning.Package, spaces []learningSpace) bool {
	for _, space := range spaces {
		if space.Grade == pkg.Grade && space.Semester == pkg.Semester {
			return true
		}
	}
	return false
}

func overlaps(left, right []string) bool {
	for _, value := range left {
		if containsString(right, value) {
			return true
		}
	}
	return false
}

func recommendationSamples(courses, materials []string) []string {
	samples := append([]string{}, courses...)
	samples = append(samples, materials...)
	if len(samples) > 3 {
		return samples[:3]
	}
	return samples
}

func (s *MemoryStore) todayTodosForStudent(student learning.Student, pendingHomework []learning.Homework, feedback []learning.ClassroomFeedback, subscriptionReminder learning.SubscriptionReminder) []learning.StudentTodo {
	out := make([]learning.StudentTodo, 0)
	for index, item := range pendingHomework {
		if index >= 3 {
			break
		}
		out = append(out, learning.StudentTodo{
			ID:         "todo-homework-" + item.ID,
			Type:       "homework",
			Title:      item.Title,
			Summary:    homeworkTodoSummary(item),
			ActionText: "去完成",
			Path:       "/pages/answer/index?id=" + item.ID,
			Priority:   100 - index,
			Status:     "待完成",
		})
	}
	if class, ok := s.nextScheduleTodoClass(student.ID); ok {
		out = append(out, learning.StudentTodo{
			ID:         "todo-schedule-" + class.ID,
			Type:       "schedule",
			Title:      "下一节课",
			Summary:    class.CourseName + " · " + weekLabelCN(class.DayOfWeek) + " " + class.StartTime + "-" + class.EndTime + " · " + class.TeacherName,
			ActionText: "看课表",
			Path:       "/pages/schedule/index",
			Priority:   80,
			Status:     class.Status,
		})
	}
	if len(feedback) > 0 {
		item := feedback[0]
		out = append(out, learning.StudentTodo{
			ID:         "todo-feedback-" + item.ID,
			Type:       "feedback",
			Title:      "查看课堂反馈",
			Summary:    item.CourseName + " · " + item.Performance,
			ActionText: "查看反馈",
			Path:       "/pages/result/index?id=" + item.RelatedSubmissionID,
			Priority:   70,
			Status:     "待查看",
		})
	}
	if !subscriptionReminder.Enabled {
		out = append(out, learning.StudentTodo{
			ID:         "todo-subscribe-learning",
			Type:       "subscribe",
			Title:      subscriptionReminder.Title,
			Summary:    subscriptionReminder.Summary,
			ActionText: subscriptionReminder.ActionText,
			Priority:   50,
			Status:     subscriptionTodoStatus(subscriptionReminder),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Priority > out[j].Priority })
	return out
}

func subscriptionTodoStatus(reminder learning.SubscriptionReminder) string {
	if reminder.TemplateConfigured {
		return "建议开启"
	}
	return "开通中"
}

func homeworkTodoSummary(item learning.Homework) string {
	parts := []string{item.Course}
	if item.Deadline != "" {
		parts = append(parts, "截止 "+item.Deadline)
	}
	if item.QuestionNum > 0 {
		parts = append(parts, itoa(item.QuestionNum)+" 道题")
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, " · ")
}

func (s *MemoryStore) nextScheduleTodoClass(studentID string) (learning.ScheduleClass, bool) {
	for _, item := range s.scheduleClasses {
		// 与学生课表同一道闸门：待审核的课不能从「下一节课」待办漏出去。
		if !scheduleVisibleToStudent(item) {
			continue
		}
		for _, student := range item.Students {
			if student.ID == studentID {
				return item, true
			}
		}
	}
	return learning.ScheduleClass{}, false
}

func (s *MemoryStore) classroomFeedbackForStudent(studentID string) []learning.ClassroomFeedback {
	out := make([]learning.ClassroomFeedback, 0)
	for _, submission := range s.submissions {
		if submission.StudentID != studentID || submission.Status != "已批改" {
			continue
		}
		homework, ok := s.findHomework(submission.HomeworkID)
		if !ok {
			continue
		}
		out = append(out, learning.ClassroomFeedback{
			ID:                  "feedback-" + submission.ID,
			CourseName:          homework.Course,
			LessonTitle:         homework.Title,
			TeacherName:         homework.OwnerTeacherName,
			Performance:         performanceForScore(submission.FinalScore),
			Focus:               strings.TrimSpace(submission.TeacherComment),
			NextStep:            nextStepForFeedback(submission.FinalScore, homework.Title),
			Score:               submission.FinalScore,
			CreatedAt:           submission.CreatedAt,
			RelatedSubmissionID: submission.ID,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if len(out) > 3 {
		return out[:3]
	}
	return out
}

func (s *MemoryStore) subscriptionReminder(studentID string) learning.SubscriptionReminder {
	templateIDs := compactStrings(s.miniProgramSubscribeTemplateIDs)
	configured := len(templateIDs) > 0
	enabled := false
	if pref, ok := s.subscriptionPreferences[studentID]; ok && pref.Enabled {
		enabled = true
	}
	summary := "开启后可接收上课、作业和批改完成提醒。"
	actionText := "开启提醒"
	if !configured {
		summary = "提醒服务开通中，可先在通知消息查看学习安排。"
		actionText = "查看通知"
	}
	if enabled {
		summary = "已开启学习提醒，上课、作业和批改结果会及时通知你。"
		actionText = "已开启"
	}
	return learning.SubscriptionReminder{
		Enabled:            enabled,
		TemplateConfigured: configured,
		TemplateIDs:        templateIDs,
		Title:              "学习提醒",
		Summary:            summary,
		ActionText:         actionText,
	}
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (s *MemoryStore) confirmStudentSubscriptionUnlocked(operator string, principal learning.Principal, req learning.StudentSubscriptionRequest) (learning.SubscriptionReminder, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.SubscriptionReminder, error) {
			return work.confirmStudentSubscriptionUnlocked(operator, principal, req)
		})
	}
	if principal.StudentID == "" {
		return learning.SubscriptionReminder{}, errors.New("student account is not bound")
	}
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return learning.SubscriptionReminder{}, errors.New("学生不存在")
	}
	configuredIDs := compactStrings(s.miniProgramSubscribeTemplateIDs)
	if len(configuredIDs) == 0 {
		return learning.SubscriptionReminder{}, errors.New("学习提醒服务正在开通，请先查看通知消息")
	}
	acceptedIDs := compactStrings(req.TemplateIDs)
	if len(acceptedIDs) == 0 {
		return learning.SubscriptionReminder{}, errors.New("请先允许学习提醒")
	}
	configuredSet := map[string]bool{}
	for _, id := range configuredIDs {
		configuredSet[id] = true
	}
	for _, id := range acceptedIDs {
		if !configuredSet[id] {
			return learning.SubscriptionReminder{}, errors.New("订阅模板不匹配，请重新进入页面后再试")
		}
	}
	preference := learning.StudentSubscriptionPreference{
		StudentID:   student.ID,
		Enabled:     true,
		TemplateIDs: acceptedIDs,
		UpdatedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}
	s.subscriptionPreferences[student.ID] = preference
	s.prependLog(operator, "开启小程序订阅消息", student.Name)
	return s.subscriptionReminder(student.ID), nil
}

func performanceForScore(score int) string {
	if score >= 90 {
		return "本次掌握扎实，表达和准确率表现稳定。"
	}
	if score >= 75 {
		return "本次完成情况良好，部分细节还需要巩固。"
	}
	return "本次基础点还需要加强，建议按老师反馈完成订正。"
}

func nextStepForFeedback(score int, title string) string {
	if score >= 90 {
		return "保持当前节奏，复盘 " + title + " 中的关键方法。"
	}
	if score >= 75 {
		return "订正易错点，再完成一次同类题巩固。"
	}
	return "先回看本节重点，再请老师确认订正情况。"
}

// StudentStudy 返回学习页聚合数据：可学课程（带真实进度）与资料。
func (s *MemoryStore) studentStudyUnlocked(principal learning.Principal) (learning.StudentStudyBoard, error) {
	if principal.StudentID == "" {
		return learning.StudentStudyBoard{}, errors.New("student account is not bound")
	}
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return learning.StudentStudyBoard{}, errors.New("student not found")
	}
	if student.AccountStatus == "停用" {
		return learning.StudentStudyBoard{}, errors.New("账号已停用，请联系老师或管理员")
	}
	courses := s.coursesForStudent(student.ID)
	cards := make([]learning.StudentCourseCard, 0, len(courses))
	for _, course := range courses {
		cards = append(cards, learning.StudentCourseCard{
			Course:   course,
			Progress: s.courseProgress(student.ID, course.ID),
		})
	}
	materials := s.studentMaterialsForPrincipal(principal)
	if len(materials) == 0 {
		materials = []learning.Material{}
	}
	return learning.StudentStudyBoard{Student: student, Courses: cards, Materials: materials}, nil
}

// StudentTasks 返回任务列表，studentStatus 由提交记录派生（已完成/待完成）。
func (s *MemoryStore) studentTasksUnlocked(principal learning.Principal) ([]learning.StudentTask, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	homework := s.studentHomeworkForPrincipal(principal)
	tasks := make([]learning.StudentTask, 0, len(homework))
	for _, item := range homework {
		task := learning.StudentTask{Homework: item, StudentStatus: "待完成"}
		if sub, ok := s.latestSubmission(principal.StudentID, item.ID); ok {
			if sub.Status == "待批改" {
				task.StudentStatus = "批改中"
			} else {
				task.StudentStatus = "已完成"
			}
			task.Score = sub.Score
			task.SubmissionID = sub.ID
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (s *MemoryStore) pendingHomeworkForStudent(studentID string) []learning.Homework {
	homework := s.homeworkForStudent(studentID)
	out := make([]learning.Homework, 0, len(homework))
	for _, item := range homework {
		if _, ok := s.latestSubmission(studentID, item.ID); ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *MemoryStore) pendingHomeworkForPrincipal(principal learning.Principal) []learning.Homework {
	homework := s.studentHomeworkForPrincipal(principal)
	out := make([]learning.Homework, 0, len(homework))
	for _, item := range homework {
		if _, ok := s.latestSubmission(principal.StudentID, item.ID); ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

// courseProgress 复用学习地图站点，计算某课程对当前学生的真实完成度。
func (s *MemoryStore) courseProgress(studentID, courseID string) int {
	if studentID == "" || courseID == "" {
		return 0
	}
	materials := make([]learning.Material, 0)
	for _, material := range s.materialsForStudent(studentID) {
		if material.CourseID == courseID {
			materials = append(materials, material)
		}
	}
	homework := make([]learning.Homework, 0)
	for _, item := range s.homeworkForStudent(studentID) {
		if item.CourseID == courseID {
			homework = append(homework, item)
		}
	}
	return stationProgress(s.buildStations(studentID, materials, homework))
}

// latestSubmission 返回某学生对某小挑战最近一次提交。
func (s *MemoryStore) latestSubmission(studentID, homeworkID string) (learning.Submission, bool) {
	var latest learning.Submission
	found := false
	for _, sub := range s.submissions {
		if sub.StudentID != studentID || sub.HomeworkID != homeworkID {
			continue
		}
		if !found || sub.CreatedAt > latest.CreatedAt {
			latest = sub
			found = true
		}
	}
	return latest, found
}
