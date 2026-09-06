package store

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) studentScheduleUnlocked(principal learning.Principal) ([]learning.ScheduleClass, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	out := make([]learning.ScheduleClass, 0)
	for _, item := range s.scheduleClasses {
		if !scheduleVisibleToStudent(item) {
			continue
		}
		for _, student := range item.Students {
			if student.ID == principal.StudentID {
				out = append(out, cloneScheduleClass(item))
				break
			}
		}
	}
	return out, nil
}

// scheduleVisibleToStudent 是学生端课表的唯一闸门。
// 老师提交但管理员还没通过的课，学生和家长一律看不到——否则老师随手排一节，
// 家长就当成板上钉钉的安排了。已取消的课同理不再露出。
func scheduleVisibleToStudent(item learning.ScheduleClass) bool {
	return item.AuditStatus == learning.AuditApproved && item.Status != "已取消"
}

func (s *MemoryStore) studentCourseDetailUnlocked(principal learning.Principal, courseID string) (learning.StudentCourseDetail, error) {
	if principal.StudentID == "" {
		return learning.StudentCourseDetail{}, errors.New("student account is not bound")
	}
	courseID = strings.TrimSpace(courseID)
	var course learning.Course
	found := false
	for _, item := range s.coursesForStudent(principal.StudentID) {
		if item.ID == courseID {
			course = item
			found = true
			break
		}
	}
	if !found {
		for _, item := range s.previewCoursesForStudent(principal.StudentID) {
			if item.ID == courseID {
				course = item
				found = true
				break
			}
		}
	}
	if !found {
		return learning.StudentCourseDetail{}, errors.New("课程不存在或未开通")
	}
	materials := make([]learning.Material, 0)
	for _, material := range s.studentMaterialsForPrincipal(principal) {
		if material.CourseID == courseID {
			materials = append(materials, material)
		}
	}
	if len(materials) == 0 {
		if lessonID, ok := s.previewLessonForCourse(course); ok {
			for _, material := range s.materials {
				if (material.CourseID == courseID || material.LearningSpaceID == course.LearningSpaceID) && material.LessonID == lessonID && materialPublished(material.Status) {
					materials = append(materials, material)
				}
			}
		}
	}
	homework := make([]learning.Homework, 0)
	for _, item := range s.studentHomeworkForPrincipal(principal) {
		if item.CourseID == courseID {
			homework = append(homework, item)
		}
	}
	stations := s.buildStations(principal.StudentID, materials, homework)
	if !s.hasActiveSubjectContent(principal.StudentID, course.Grade, course.Subject) {
		stations = append(stations, s.lockedPreviewStations(course)...)
	}
	return learning.StudentCourseDetail{
		Course:    course,
		Materials: materials,
		Homework:  homework,
		Stations:  stations,
		Progress:  stationProgress(stations),
	}, nil
}

// lockedPreviewStations 只返回目录标题和锁定状态，不返回任何资料或习题 ID，
// 让学生知道后续内容存在，同时避免客户端通过拼接 ID 绕过后端权限。
func (s *MemoryStore) lockedPreviewStations(course learning.Course) []learning.Station {
	previewLessonID, hasPreview := s.previewLessonForCourse(course)
	lessons := make([]learning.CurriculumNode, 0)
	for _, node := range course.Curriculum {
		if node.Type == learning.CurriculumLesson && (!hasPreview || node.ID != previewLessonID) {
			lessons = append(lessons, node)
		}
	}
	sort.SliceStable(lessons, func(i, j int) bool { return lessons[i].SortOrder < lessons[j].SortOrder })
	out := make([]learning.Station, 0, len(lessons))
	for _, lesson := range lessons {
		out = append(out, learning.Station{Icon: "🔒", Title: lesson.Name, Desc: "开通后可查看讲义和练习", Status: "未开通"})
	}
	return out
}

// StudentGrowth 返回成长轨迹：提交记录 + 已学资料，按时间倒序。
func (s *MemoryStore) studentGrowthUnlocked(principal learning.Principal) ([]learning.StudentLearningRecord, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	records := make([]learning.StudentLearningRecord, 0)
	visibleHomework := map[string]bool{}
	for _, item := range s.homeworkForStudent(principal.StudentID) {
		visibleHomework[item.ID] = true
	}
	for _, sub := range s.submissions {
		if sub.StudentID != principal.StudentID {
			continue
		}
		if !visibleHomework[sub.HomeworkID] {
			continue
		}
		records = append(records, learning.StudentLearningRecord{
			ID: "growth-" + sub.ID, Type: "小挑战", Title: sub.TaskTitle, Status: sub.Status,
			Score: sub.Score, OccurredAt: sub.CreatedAt, Description: sub.TeacherComment,
		})
	}
	student, _ := s.findStudent(principal.StudentID)
	for _, material := range s.materialsForStudent(principal.StudentID) {
		records = append(records, learning.StudentLearningRecord{
			ID: "growth-mat-" + material.ID, Type: "资料", Title: material.Title, Course: material.Course,
			Status: "已学习", OccurredAt: firstNonEmpty(student.LastStudyAt, "2026-05-22 18:20:00"), Description: "查看课件资料",
		})
	}
	for _, feedback := range s.lessonFeedbacks {
		if feedback.StudentID != principal.StudentID {
			continue
		}
		records = append(records, learning.StudentLearningRecord{ID: "growth-lesson-" + feedback.ID, Type: "课后反馈", Title: feedback.CourseName, Course: feedback.TeacherName, Status: "已反馈", OccurredAt: firstNonEmpty(feedback.LessonDate, feedback.UpdatedAt), Description: feedback.Summary})
	}
	for _, summary := range s.scoreSummariesForStudent(principal.StudentID) {
		if summary.LatestRecord == nil {
			continue
		}
		score := *summary.LatestRecord
		records = append(records, learning.StudentLearningRecord{
			ID: "growth-score-" + score.ID, Type: "成绩", Title: score.ExamName, Course: score.Subject,
			Status: "已记录", Score: score.Score, FullScore: score.FullScore, OccurredAt: score.ExamDate, Description: summary.Description,
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].OccurredAt > records[j].OccurredAt })
	return records, nil
}

// StudentBadges 返回徽章墙，是否获得由学生真实学习数据派生。
func (s *MemoryStore) studentBadgesUnlocked(principal learning.Principal) ([]learning.Badge, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return nil, errors.New("student not found")
	}
	materialCount := len(s.materialsForStudent(principal.StudentID))
	submissionCount := 0
	hasSubmission := false
	for _, sub := range s.submissions {
		if sub.StudentID == principal.StudentID {
			submissionCount++
			hasSubmission = true
		}
	}
	return []learning.Badge{
		{ID: "badge-reading", Icon: "⭐", Name: "阅读小星星", Desc: "完成第一次小挑战", Obtained: hasSubmission},
		{ID: "badge-streak", Icon: "🔥", Name: "坚持不懈", Desc: "连续学习满 7 天", Obtained: student.StreakDays >= 7},
		{ID: "badge-expert", Icon: "🏅", Name: "学习小达人", Desc: "平均分达到 90", Obtained: student.AverageScore >= 90},
		{ID: "badge-explorer", Icon: "🧭", Name: "探索者", Desc: "学习满 5 份学习资料", Obtained: materialCount >= 5},
		{ID: "badge-challenger", Icon: "🎯", Name: "挑战王", Desc: "提交满 3 次小挑战", Obtained: submissionCount >= 3},
	}, nil
}

// StudentFavorites 返回当前学生的收藏列表，按收藏时间倒序。
func (s *MemoryStore) studentFavoritesUnlocked(principal learning.Principal) ([]learning.Favorite, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	out := make([]learning.Favorite, 0)
	for _, fav := range s.favorites {
		if fav.StudentID != principal.StudentID {
			continue
		}
		switch fav.TargetType {
		case "material":
			if _, err := s.studentMaterialUnlocked(principal, fav.TargetID); err != nil {
				continue
			}
		case "homework":
			if _, err := s.studentHomeworkUnlocked(principal, fav.TargetID); err != nil {
				continue
			}
		default:
			continue
		}
		out = append(out, fav)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// AddFavorite 收藏一条内容，校验该内容在学生权限范围内，幂等。
func (s *MemoryStore) addFavoriteUnlocked(operator string, principal learning.Principal, req learning.FavoriteRequest) (learning.Favorite, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Favorite, error) {
			return work.addFavoriteUnlocked(operator, principal, req)
		})
	}
	if principal.StudentID == "" {
		return learning.Favorite{}, errors.New("student account is not bound")
	}
	req.TargetType = strings.TrimSpace(req.TargetType)
	req.TargetID = strings.TrimSpace(req.TargetID)
	var title, course string
	switch req.TargetType {
	case "material":
		material, err := s.studentMaterialUnlocked(principal, req.TargetID)
		if err != nil {
			return learning.Favorite{}, err
		}
		title, course = material.Title, material.Course
	case "homework":
		homework, err := s.studentHomeworkUnlocked(principal, req.TargetID)
		if err != nil {
			return learning.Favorite{}, err
		}
		title, course = homework.Title, homework.Course
	default:
		return learning.Favorite{}, errors.New("不支持的收藏类型")
	}
	for _, fav := range s.favorites {
		if fav.StudentID == principal.StudentID && fav.TargetType == req.TargetType && fav.TargetID == req.TargetID {
			return fav, nil
		}
	}
	fav := learning.Favorite{
		ID:         "fav-" + time.Now().Format("20060102150405.000000000"),
		StudentID:  principal.StudentID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Title:      title,
		Course:     course,
		CreatedAt:  time.Now().Format("2006-01-02 15:04:05"),
	}
	s.favorites[fav.ID] = fav
	s.prependLog(operator, "收藏内容", title)
	return fav, nil
}

// RemoveFavorite 取消收藏，仅允许删除本人的收藏。
func (s *MemoryStore) removeFavoriteUnlocked(operator string, principal learning.Principal, id string) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error { return work.removeFavoriteUnlocked(operator, principal, id) })
	}
	if principal.StudentID == "" {
		return errors.New("student account is not bound")
	}
	id = strings.TrimSpace(id)
	fav, ok := s.favorites[id]
	if !ok {
		return errors.New("收藏记录不存在")
	}
	if fav.StudentID != principal.StudentID {
		return errors.New("没有权限取消该收藏")
	}
	delete(s.favorites, id)
	s.prependLog(operator, "取消收藏", fav.Title)
	return nil
}

func (s *MemoryStore) studentMaterialUnlocked(principal learning.Principal, materialID string) (learning.Material, error) {
	if principal.StudentID == "" {
		return learning.Material{}, errors.New("student account is not bound")
	}
	materialID = strings.TrimSpace(materialID)
	for _, material := range s.materialsForStudent(principal.StudentID) {
		if material.ID == materialID {
			return s.decorateStudentMaterial(principal, material), nil
		}
	}
	for _, course := range s.previewCoursesForStudent(principal.StudentID) {
		hadHandoutGrant, hadActiveHandoutGrant := false, false
		for _, grant := range s.grants {
			if grant.StudentID == principal.StudentID && containsString(s.contentTypesForPackage(grant.PackageID), "handout") && containsString(s.learningSpaceIDsForGrant(grant.ID), course.LearningSpaceID) {
				hadHandoutGrant = true
				hadActiveHandoutGrant = hadActiveHandoutGrant || grantActive(grant)
			}
		}
		if hadHandoutGrant && !hadActiveHandoutGrant {
			continue
		}
		previewLessonID, ok := s.previewLessonForCourse(course)
		if !ok {
			continue
		}
		for _, material := range s.materials {
			if material.ID == materialID && material.LessonID == previewLessonID && s.courseContentMatches(course.ID, material.CourseID, material.LearningSpaceID) && materialPublished(material.Status) {
				return s.decorateStudentMaterial(principal, material), nil
			}
		}
	}
	return learning.Material{}, errors.New("资料不存在或未开通")
}

func (s *MemoryStore) studentMaterialPreviewFileUnlocked(principal learning.Principal, materialID string) (learning.FileAsset, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.FileAsset, error) {
			return work.studentMaterialPreviewFileUnlocked(principal, materialID)
		})
	}
	material, err := s.studentMaterialUnlocked(principal, materialID)
	if err != nil {
		return learning.FileAsset{}, err
	}
	if strings.TrimSpace(material.FileID) == "" {
		return learning.FileAsset{}, errors.New("资料暂未上传预览文件")
	}
	asset, ok := s.fileAssets[material.FileID]
	if !ok {
		return learning.FileAsset{}, errors.New("资料预览文件不存在")
	}
	asset.WatermarkText = s.studentWatermarkText(principal)
	if asset.PreviewStatus != "可预览" || strings.TrimSpace(asset.PreviewPath) == "" {
		return asset, nil
	}
	generatedAt := time.Now().Truncate(5 * time.Minute)
	stampText, traceCode := s.studentWatermarkStampText(principal, material.ID, generatedAt)
	asset.WatermarkStampText = stampText
	s.prependLogDetail(studentAuditOperator(principal), "内容防盗版风控", material.Title, "eventType=material_preview; targetType=material; targetId="+material.ID+"; watermarkTrace="+traceCode)
	return asset, nil
}

func (s *MemoryStore) studentHomeworkPreviewFileUnlocked(principal learning.Principal, homeworkID string) (learning.FileAsset, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.FileAsset, error) {
			return work.studentHomeworkPreviewFileUnlocked(principal, homeworkID)
		})
	}
	homework, err := s.studentHomeworkUnlocked(principal, homeworkID)
	if err != nil {
		return learning.FileAsset{}, err
	}
	if strings.TrimSpace(homework.FileID) == "" {
		return learning.FileAsset{}, errors.New("练习暂未上传下载文件")
	}
	asset, ok := s.fileAssets[homework.FileID]
	if !ok {
		return learning.FileAsset{}, errors.New("练习下载文件不存在")
	}
	asset.WatermarkText = s.studentWatermarkText(principal)
	if asset.PreviewStatus != "可预览" || strings.TrimSpace(asset.PreviewPath) == "" {
		return asset, nil
	}
	generatedAt := time.Now().Truncate(5 * time.Minute)
	stampText, traceCode := s.studentWatermarkStampText(principal, homework.ID, generatedAt)
	asset.WatermarkStampText = stampText
	s.prependLogDetail(studentAuditOperator(principal), "内容防盗版风控", homework.Title, "eventType=homework_download; targetType=homework; targetId="+homework.ID+"; watermarkTrace="+traceCode)
	return asset, nil
}

func (s *MemoryStore) studentHomeworkUnlocked(principal learning.Principal, homeworkID string) (learning.Homework, error) {
	if principal.StudentID == "" {
		return learning.Homework{}, errors.New("student account is not bound")
	}
	homeworkID = strings.TrimSpace(homeworkID)
	for _, item := range s.homeworkForStudent(principal.StudentID) {
		if item.ID == homeworkID {
			return s.decorateStudentHomework(principal, item), nil
		}
	}
	return learning.Homework{}, errors.New("题目不存在或未开通")
}

func (s *MemoryStore) recordStudentSecurityEventUnlocked(operator string, principal learning.Principal, req learning.SecurityEventRequest) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error {
			return work.recordStudentSecurityEventUnlocked(operator, principal, req)
		})
	}
	if principal.StudentID == "" {
		return errors.New("student account is not bound")
	}
	req.EventType = strings.TrimSpace(req.EventType)
	req.TargetType = strings.TrimSpace(req.TargetType)
	req.TargetID = strings.TrimSpace(req.TargetID)
	req.PagePath = strings.TrimSpace(req.PagePath)
	req.Detail = strings.TrimSpace(req.Detail)
	if req.EventType == "" {
		return errors.New("请选择风控事件类型")
	}
	target := studentSecurityTarget(principal, req)
	if req.TargetType == "material" && req.TargetID != "" {
		material, err := s.studentMaterialUnlocked(principal, req.TargetID)
		if err != nil {
			return err
		}
		target = material.Title
	}
	if req.TargetType == "homework" && req.TargetID != "" {
		homework, err := s.studentHomeworkUnlocked(principal, req.TargetID)
		if err != nil {
			return err
		}
		target = homework.Title
	}
	detail := "eventType=" + req.EventType + "; targetType=" + req.TargetType + "; targetId=" + req.TargetID + "; pagePath=" + req.PagePath
	if req.Detail != "" {
		detail += "; detail=" + req.Detail
	}
	if operator == "" {
		operator = studentAuditOperator(principal)
	}
	s.prependLogDetail(operator, "内容防盗版风控", target, detail)
	return nil
}

func (s *MemoryStore) createSubmissionUnlocked(operator string, principal learning.Principal, req learning.SubmissionRequest) (learning.Submission, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Submission, error) {
			return work.createSubmissionUnlocked(operator, principal, req)
		})
	}
	homework, err := s.studentHomeworkUnlocked(principal, req.HomeworkID)
	if err != nil {
		return learning.Submission{}, err
	}
	if homework.DeadlineAt != "" {
		deadline, parseErr := time.Parse(time.RFC3339, homework.DeadlineAt)
		if parseErr == nil && !time.Now().Before(deadline) {
			return learning.Submission{}, errors.New("本次练习已截止，无法提交")
		}
	}
	if len(req.Answers) == 0 {
		return learning.Submission{}, errors.New("请先作答再提交")
	}
	score, hasText := gradeSubmission(homework, req.Answers)
	status := "已批改"
	comment := commentForScore(score, homework.Title)
	reward := rewardForScore(score)
	if hasText {
		status = "待批改"
		comment = "客观题已完成，简答题老师正在批改。"
		reward = ""
	}
	submission := learning.Submission{
		ID:             "sub-" + time.Now().Format("20060102150405.000000000"),
		HomeworkID:     homework.ID,
		StudentID:      principal.StudentID,
		TaskTitle:      homework.Title,
		Score:          score,
		ObjectiveScore: score,
		FinalScore:     score,
		TeacherComment: comment,
		Reward:         reward,
		Status:         status,
		CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		Answers:        cloneSubmissionAnswers(req.Answers),
	}
	s.submissions[submission.ID] = cloneSubmission(submission)
	if hasText {
		student, _ := s.findStudent(principal.StudentID)
		review := learning.Review{
			ID: "rev-" + submission.ID, StudentID: principal.StudentID, HomeworkID: homework.ID, SubmissionID: submission.ID,
			StudentName: student.Name, PackageName: homework.PackageName, Homework: homework.Title, SystemScore: score,
			TeacherComment: "", Status: "待批改",
		}
		if assignment, found := s.primaryTutoringAssignmentForHomework(principal.StudentID, homework); found {
			review.ReviewerTeacherID = assignment.TeacherID
			review.ReviewerTeacherName = assignment.TeacherName
			review.TutoringAssignmentID = assignment.ID
			review.AssignedAt = time.Now().Format("2006-01-02 15:04:05")
		}
		s.reviews = append([]learning.Review{review}, s.reviews...)
	}
	s.prependLog(operator, "提交小挑战", homework.Title)
	return cloneSubmission(submission), nil
}

func (s *MemoryStore) studentSubmissionUnlocked(principal learning.Principal, id string) (learning.Submission, error) {
	if principal.StudentID == "" {
		return learning.Submission{}, errors.New("student account is not bound")
	}
	id = strings.TrimSpace(id)
	submission, ok := s.submissions[id]
	if !ok {
		return learning.Submission{}, errors.New("提交记录不存在")
	}
	if submission.StudentID != principal.StudentID {
		return learning.Submission{}, errors.New("没有权限查看该批改结果")
	}
	return cloneSubmission(submission), nil
}

func (s *MemoryStore) hasSubmission(studentID, homeworkID string) bool {
	for _, submission := range s.submissions {
		if submission.StudentID == studentID && submission.HomeworkID == homeworkID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) latestSubmissionForStudent(studentID, homeworkID string) (learning.Submission, bool) {
	var latest learning.Submission
	found := false
	for _, submission := range s.submissions {
		if submission.StudentID != studentID || submission.HomeworkID != homeworkID {
			continue
		}
		if !found || submission.CreatedAt > latest.CreatedAt {
			latest = submission
			found = true
		}
	}
	return latest, found
}

func (s *MemoryStore) buildStations(studentID string, materials []learning.Material, homework []learning.Homework) []learning.Station {
	type stationSource struct {
		material *learning.Material
		homework *learning.Homework
		order    int
	}
	sources := make([]stationSource, 0, len(materials)+len(homework))
	for index := range materials {
		sources = append(sources, stationSource{material: &materials[index], order: materials[index].SortOrder})
	}
	for index := range homework {
		sources = append(sources, stationSource{homework: &homework[index], order: homework[index].SortOrder})
	}
	sort.SliceStable(sources, func(i, j int) bool {
		left, right := sources[i].order, sources[j].order
		if left == 0 || right == 0 {
			return left != 0 && right == 0
		}
		return left < right
	})
	stations := make([]learning.Station, 0, len(sources))
	for _, source := range sources {
		if source.material != nil {
			material := *source.material
			status := "学习中"
			desc := "正在学习，继续加油"
			if len(stations) > 0 {
				status = "待挑战"
				desc = "完成上一站后继续阅读"
			}
			stations = append(stations, learning.Station{
				Icon:       "📖",
				Title:      "第 " + strconv.Itoa(len(stations)+1) + " 站 " + material.Title,
				Desc:       desc,
				Status:     status,
				TagCode:    material.TagCode,
				MaterialID: material.ID,
			})
			continue
		}
		item := *source.homework
		status := "待挑战"
		desc := "完成小挑战即可解锁奖励"
		if s.hasSubmission(studentID, item.ID) {
			status = "已完成"
			desc = "已提交，等待老师反馈"
		}
		stations = append(stations, learning.Station{
			Icon:       "🎯",
			Title:      "第 " + strconv.Itoa(len(stations)+1) + " 站 " + item.Title,
			Desc:       desc,
			Status:     status,
			TagCode:    item.TagCode,
			HomeworkID: item.ID,
		})
	}
	return stations
}

func stationProgress(stations []learning.Station) int {
	if len(stations) == 0 {
		return 0
	}
	done := 0
	for _, station := range stations {
		if station.Status == "已完成" {
			done++
		}
	}
	return done * 100 / len(stations)
}

func gradeSubmission(homework learning.Homework, answers []learning.SubmissionAnswer) (int, bool) {
	if len(homework.Questions) == 0 {
		return 90, false
	}
	answerMap := make(map[string]learning.SubmissionAnswer, len(answers))
	for _, answer := range answers {
		answerMap[answer.QuestionID] = answer
	}
	totalScore := 0
	gotScore := 0
	hasText := false
	for _, question := range homework.Questions {
		score := question.Score
		if score <= 0 {
			score = 10
		}
		totalScore += score
		answer := answerForQuestion(answerMap, question.ID)
		if question.Type == "single" || question.Type == "judge" {
			if strings.EqualFold(strings.TrimSpace(answer.Choice), strings.TrimSpace(question.Answer)) {
				gotScore += score
			}
			continue
		}
		if question.Type == "multiple" {
			if sameChoiceSet(answer.Choices, normalizedQuestionAnswers(question)) {
				gotScore += score
			}
			continue
		}
		if question.Type == "fill" {
			if sameChoiceSet([]string{answer.Text}, normalizedQuestionAnswers(question)) {
				gotScore += score
			}
			continue
		}
		hasText = true
	}
	if totalScore == 0 {
		return 0, hasText
	}
	return gotScore * 100 / totalScore, hasText
}

func answerForQuestion(answerMap map[string]learning.SubmissionAnswer, questionID string) learning.SubmissionAnswer {
	if answer, ok := answerMap[questionID]; ok {
		return answer
	}
	if index := strings.LastIndex(questionID, "-q"); index >= 0 {
		if answer, ok := answerMap[questionID[index+1:]]; ok {
			return answer
		}
	}
	return learning.SubmissionAnswer{}
}

func commentForScore(score int, title string) string {
	switch {
	case score >= 90:
		return title + "完成得很棒，重点都抓住啦，继续保持！"
	case score >= 60:
		return title + "整体不错，个别地方再细心一点就更好了。"
	default:
		return title + "已经迈出第一步啦，跟着学习资料再复习一遍，下次一定更好。"
	}
}

func rewardForScore(score int) string {
	if score >= 90 {
		return "获得「学习之星」徽章 ⭐"
	}
	if score >= 60 {
		return "获得 10 点能量值 ⚡"
	}
	return "完成即可获得 5 点能量值"
}

func normalizeReviewFinalStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "已批改"
	}
	if status == "待复核" || status == "已批改" {
		return status
	}
	return ""
}

func reviewNoticeTitle(status string) string {
	if status == "待复核" {
		return "批改反馈待复核"
	}
	return "批改完成提醒"
}

func reviewNoticeSummary(homeworkTitle, status string) string {
	if status == "待复核" {
		return homeworkTitle + "已有老师反馈，复核完成后会更新最终结果。"
	}
	return homeworkTitle + "已完成批改，快去查看老师反馈。"
}

func reviewLogAction(status string) string {
	if status == "待复核" {
		return "提交复核"
	}
	return "完成批改"
}
