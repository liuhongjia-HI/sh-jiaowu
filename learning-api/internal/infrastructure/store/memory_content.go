package store

import (
	"errors"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) coursesUnlocked(principal learning.Principal) []learning.Course {
	out := make([]learning.Course, 0, len(s.courses))
	for _, course := range s.courses {
		if canSeeCourse(principal, course) {
			out = append(out, s.decorateCourse(course))
		}
	}
	return out
}

func (s *MemoryStore) createCourseUnlocked(operator string, principal learning.Principal, req learning.CourseUpsertRequest) (learning.Course, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Course, error) {
			return work.createCourseUnlocked(operator, principal, req)
		})
	}
	course, err := s.courseFromRequest(principal, "", req)
	if err != nil {
		return learning.Course{}, err
	}
	if s.courseNameExists("", course.Name) {
		return learning.Course{}, errors.New("课程名称已存在")
	}
	course.ID = "course-custom-" + time.Now().Format("20060102150405.000000000")
	s.courses = append([]learning.Course{course}, s.courses...)
	s.prependLog(operator, "创建课程", course.Name)
	return s.decorateCourse(course), nil
}

func (s *MemoryStore) updateCourseUnlocked(operator string, principal learning.Principal, id string, req learning.CourseUpsertRequest) (learning.Course, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Course, error) {
			return work.updateCourseUnlocked(operator, principal, id, req)
		})
	}
	id = strings.TrimSpace(id)
	course, err := s.courseFromRequest(principal, id, req)
	if err != nil {
		return learning.Course{}, err
	}
	if s.courseNameExists(id, course.Name) {
		return learning.Course{}, errors.New("课程名称已存在")
	}
	for index := range s.courses {
		if s.courses[index].ID != id {
			continue
		}
		before := s.decorateCourse(s.courses[index])
		s.courses[index] = course
		s.syncCourseReferences(course)
		after := s.decorateCourse(course)
		s.prependLogDetail(operator, "编辑课程", course.Name, auditChangeDetail(courseAuditSnapshot(before), courseAuditSnapshot(after)))
		return after, nil
	}
	return learning.Course{}, errors.New("课程不存在")
}

func (s *MemoryStore) deleteCourseUnlocked(operator string, principal learning.Principal, id string) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error { return work.deleteCourseUnlocked(operator, principal, id) })
	}
	id = strings.TrimSpace(id)
	if !(hasRole(principal.Roles, learning.RoleTeacher) || hasRole(principal.Roles, learning.RoleOpsStaff) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleSuperAdmin)) {
		return errors.New("当前账号没有维护课程权限，请联系管理员开通")
	}
	for i, course := range s.courses {
		if course.ID != id {
			continue
		}
		if !canSeeCourse(principal, course) {
			return errors.New("不能删除未负责的课程")
		}
		name := course.Name
		s.courses = append(s.courses[:i:i], s.courses[i+1:]...)
		s.materials = filterMaterialsByCourse(s.materials, id)
		s.homework = filterHomeworkByCourse(s.homework, id)
		s.prependLogDetail(operator, "删除课程", name, "")
		return nil
	}
	return errors.New("课程不存在")
}

func filterMaterialsByCourse(items []learning.Material, courseID string) []learning.Material {
	out := items[:0]
	for _, item := range items {
		if item.CourseID != courseID {
			out = append(out, item)
		}
	}
	return out
}
func filterHomeworkByCourse(items []learning.Homework, courseID string) []learning.Homework {
	out := items[:0]
	for _, item := range items {
		if item.CourseID != courseID {
			out = append(out, item)
		}
	}
	return out
}

func (s *MemoryStore) materialsUnlocked(principal learning.Principal) []learning.Material {
	courses := courseNames(s.coursesUnlocked(principal))
	return orderMaterialsByCourse(s.materialsForCourses(courses))
}

func (s *MemoryStore) materialsFilteredUnlocked(principal learning.Principal, query learning.MaterialQuery) []learning.Material {
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	query.TagCode = strings.TrimSpace(query.TagCode)
	rows := s.materialsUnlocked(principal)
	out := make([]learning.Material, 0, len(rows))
	for _, item := range rows {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Title), keyword) {
			continue
		}
		if query.Subject = strings.TrimSpace(query.Subject); query.Subject != "" && item.Subject != query.Subject {
			continue
		}
		if query.UploaderID = strings.TrimSpace(query.UploaderID); query.UploaderID != "" && item.OwnerTeacherID != query.UploaderID {
			continue
		}
		if query.UploadedFrom = strings.TrimSpace(query.UploadedFrom); query.UploadedFrom != "" && item.CreatedAt < query.UploadedFrom {
			continue
		}
		if query.UploadedTo = strings.TrimSpace(query.UploadedTo); query.UploadedTo != "" && item.CreatedAt > query.UploadedTo+" 23:59:59" {
			continue
		}
		if query.TagCode != "" && item.TagCode != query.TagCode {
			continue
		}
		out = append(out, item)
	}
	return out
}

var contentTagCodes = map[string]bool{"HD": true, "Blank": true, "HW": true, "Exam": true, "Special": true}

var contentTagPrefixes = []struct {
	prefix string
	code   string
}{
	{prefix: "hd", code: "HD"},
	{prefix: "blank", code: "Blank"},
	{prefix: "hw", code: "HW"},
	{prefix: "exam", code: "Exam"},
	{prefix: "special", code: "Special"},
}

func normalizeContentTagCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !contentTagCodes[value] {
		return "", errors.New("请选择正确的内容标签")
	}
	return value, nil
}

func contentTagCodeOrInferred(tagCode string, values ...string) string {
	if strings.TrimSpace(tagCode) != "" {
		return tagCode
	}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		for _, tag := range contentTagPrefixes {
			if value == tag.prefix || strings.HasPrefix(value, tag.prefix+"_") || strings.HasPrefix(value, tag.prefix+"-") || strings.HasPrefix(value, tag.prefix+" ") {
				return tag.code
			}
		}
	}
	return ""
}

func normalizeAssessmentType(value string) string {
	if strings.TrimSpace(value) == "mock_exam" {
		return "mock_exam"
	}
	return "practice"
}

func normalizeDeadlineAt(deadlineAt, legacyDeadline, assessmentType string, creating bool) (string, error) {
	if deadlineAt == "" && legacyDeadline != "" {
		return legacyDeadline + "T23:59:59+08:00", nil
	}
	if deadlineAt == "" {
		if assessmentType == "mock_exam" {
			return "", errors.New("模拟考试必须设置截止时间")
		}
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, deadlineAt)
	if err != nil {
		return "", errors.New("截止时间格式不正确")
	}
	if creating && parsed.Before(time.Now().Add(5*time.Minute)) {
		return "", errors.New("截止时间至少应晚于当前时间 5 分钟")
	}
	return parsed.Format(time.RFC3339), nil
}

func (s *MemoryStore) createMaterialUnlocked(operator string, principal learning.Principal, req learning.MaterialUploadRequest) (learning.Material, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Material, error) {
			return work.createMaterialUnlocked(operator, principal, req)
		})
	}
	req.Title = strings.TrimSpace(req.Title)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.LessonID = strings.TrimSpace(req.LessonID)
	tagCode, err := normalizeContentTagCode(req.TagCode)
	if err != nil {
		return learning.Material{}, err
	}
	tagCode = contentTagCodeOrInferred(tagCode, req.Title, req.File.FileName)
	if req.Title == "" {
		return learning.Material{}, errors.New("请输入学习资料标题")
	}
	course, err := s.courseForUpload(principal, req.CourseID, req.LearningSpaceID)
	if err != nil {
		return learning.Material{}, err
	}
	if !canUploadHandout(principal) {
		return learning.Material{}, errors.New("当前账号没有上传学习资料权限，请联系管理员开通")
	}
	curriculum, err := curriculumPathForLesson(course, req.LessonID)
	if err != nil {
		return learning.Material{}, err
	}
	asset := req.File
	s.fileAssets[asset.ID] = asset
	if asset.ID != "" {
		s.enqueuePreviewJobUnlocked(asset.ID)
	}
	item := learning.Material{
		ID:               "material-" + time.Now().Format("20060102150405.000000000"),
		Title:            req.Title,
		CourseID:         course.ID,
		Course:           course.Name,
		LearningSpaceID:  course.LearningSpaceID,
		LessonID:         req.LessonID,
		Curriculum:       curriculum,
		TagCode:          tagCode,
		Type:             "课程讲义",
		OwnerTeacherID:   principal.UserID,
		OwnerTeacherName: principal.Name,
		PublishStatus:    "已发布",
		Status:           learning.StatusEnabled,
		FileID:           asset.ID,
		FileName:         asset.FileName,
		FileSize:         asset.FileSize,
		FileType:         asset.FileType,
		PreviewStatus:    asset.PreviewStatus,
		CreatedAt:        time.Now().Format("2006-01-02 15:04:05"),
		SortOrder:        s.nextMaterialSortOrder(course.ID),
		PreviewURL:       "/api/files/" + asset.ID + "/preview",
		DownloadURL:      "/api/files/" + asset.ID + "/download",
	}
	s.materials = append([]learning.Material{item}, s.materials...)
	s.prependLog(operator, "上传学习资料", item.Title)
	return s.decorateMaterial(item), nil
}

func (s *MemoryStore) reorderMaterialsUnlocked(operator string, principal learning.Principal, req learning.MaterialReorderRequest) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error {
			return work.reorderMaterialsUnlocked(operator, principal, req)
		})
	}
	req.CourseID = strings.TrimSpace(req.CourseID)
	if req.CourseID == "" || len(req.MaterialIDs) == 0 {
		return errors.New("请选择需要排序的课程资料")
	}
	if !canUploadHandout(principal) {
		return errors.New("当前账号没有维护学习资料权限，请联系管理员开通")
	}
	var course learning.Course
	foundCourse := false
	for _, item := range s.courses {
		if item.ID == req.CourseID {
			course = item
			foundCourse = true
			break
		}
	}
	if !foundCourse || !canSeeCourse(principal, course) {
		return errors.New("不能维护未负责的课程资料")
	}
	indices := make(map[string]int)
	for index, item := range s.materials {
		if item.CourseID == req.CourseID {
			indices[item.ID] = index
		}
	}
	seen := make(map[string]bool, len(req.MaterialIDs))
	for _, id := range req.MaterialIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return errors.New("排序资料不正确，请刷新后重试")
		}
		if _, ok := indices[id]; !ok {
			return errors.New("资料列表已变更，请刷新后重新排序")
		}
		seen[id] = true
	}
	for order, id := range req.MaterialIDs {
		s.materials[indices[id]].SortOrder = order + 1
	}
	s.prependLog(operator, "调整学习资料顺序", course.Name)
	return nil
}

func (s *MemoryStore) nextMaterialSortOrder(courseID string) int {
	maxOrder := 0
	for _, item := range s.materials {
		if item.CourseID == courseID && item.SortOrder > maxOrder {
			maxOrder = item.SortOrder
		}
	}
	return maxOrder + 1
}

func (s *MemoryStore) nextHomeworkSortOrder(courseID string) int {
	maxOrder := 0
	for _, item := range s.homework {
		if item.CourseID == courseID && item.SortOrder > maxOrder {
			maxOrder = item.SortOrder
		}
	}
	return maxOrder + 1
}

func orderMaterialsByCourse(rows []learning.Material) []learning.Material {
	grouped := make(map[string][]learning.Material)
	courseIDs := make([]string, 0)
	for _, item := range rows {
		if _, ok := grouped[item.CourseID]; !ok {
			courseIDs = append(courseIDs, item.CourseID)
		}
		grouped[item.CourseID] = append(grouped[item.CourseID], item)
	}
	out := make([]learning.Material, 0, len(rows))
	for _, courseID := range courseIDs {
		items := grouped[courseID]
		sort.SliceStable(items, func(i, j int) bool {
			left, right := items[i].SortOrder, items[j].SortOrder
			if left == 0 || right == 0 {
				if left != right {
					return left != 0 && right == 0
				}
			}
			if left != right {
				return left < right
			}
			if items[i].CreatedAt != items[j].CreatedAt {
				return items[i].CreatedAt < items[j].CreatedAt
			}
			return items[i].ID < items[j].ID
		})
		out = append(out, items...)
	}
	return out
}

func (s *MemoryStore) updateMaterialUnlocked(operator string, principal learning.Principal, id string, req learning.MaterialUpdateRequest) (learning.Material, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Material, error) {
			return work.updateMaterialUnlocked(operator, principal, id, req)
		})
	}
	id = strings.TrimSpace(id)
	req.Title = strings.TrimSpace(req.Title)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.LessonID = strings.TrimSpace(req.LessonID)
	tagCode, err := normalizeContentTagCode(req.TagCode)
	if err != nil {
		return learning.Material{}, err
	}
	tagCode = contentTagCodeOrInferred(tagCode, req.Title)
	if req.Title == "" {
		return learning.Material{}, errors.New("请输入学习资料标题")
	}
	req.Status = normalizeMaterialStatus(req.Status)
	if !isContentStatus(req.Status) {
		return learning.Material{}, errors.New("请选择正确的发布状态")
	}
	course, err := s.courseForUpload(principal, req.CourseID, req.LearningSpaceID)
	if err != nil {
		return learning.Material{}, err
	}
	if !canUploadHandout(principal) {
		return learning.Material{}, errors.New("当前账号没有维护学习资料权限，请联系管理员开通")
	}
	curriculum, err := curriculumPathForLesson(course, req.LessonID)
	if err != nil {
		return learning.Material{}, err
	}
	for index := range s.materials {
		if s.materials[index].ID != id {
			continue
		}
		if !canSeeCourse(principal, learning.Course{ID: s.materials[index].CourseID, LearningSpaceID: s.materials[index].LearningSpaceID}) {
			return learning.Material{}, errors.New("不能维护未负责的学习资料")
		}
		before := s.materials[index]
		s.materials[index].Title = req.Title
		s.materials[index].CourseID = course.ID
		s.materials[index].Course = course.Name
		s.materials[index].LearningSpaceID = course.LearningSpaceID
		s.materials[index].LessonID = req.LessonID
		s.materials[index].Curriculum = curriculum
		s.materials[index].TagCode = tagCode
		s.materials[index].Status = req.Status
		s.materials[index].PublishStatus = publishStatus(req.Status)
		s.prependLogDetail(operator, "编辑学习资料", req.Title, auditChangeDetail(materialAuditSnapshot(before), materialAuditSnapshot(s.materials[index])))
		return s.decorateMaterial(s.materials[index]), nil
	}
	return learning.Material{}, errors.New("学习资料不存在")
}

// deleteMaterialUnlocked 删的是学习资料这条记录本身，不动它背后的文件资产——
// 上传错了、传重了这种误操作，删掉列表条目就够用；物理文件留着不清理，
// 万一以后要恢复或者审计还能找回来，不会因为一次误删连底层文件都丢了。
// 顺手把学生端收藏里指向这条资料的收藏也删掉，不留一个点开就 404 的收藏项。
func (s *MemoryStore) deleteMaterialUnlocked(operator string, principal learning.Principal, id string) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error {
			return work.deleteMaterialUnlocked(operator, principal, id)
		})
	}
	id = strings.TrimSpace(id)
	if !canUploadHandout(principal) {
		return errors.New("当前账号没有维护学习资料权限，请联系管理员开通")
	}
	for index := range s.materials {
		if s.materials[index].ID != id {
			continue
		}
		if !canSeeCourse(principal, learning.Course{ID: s.materials[index].CourseID, LearningSpaceID: s.materials[index].LearningSpaceID}) {
			return errors.New("不能删除未负责的学习资料")
		}
		title := s.materials[index].Title
		s.materials = append(s.materials[:index:index], s.materials[index+1:]...)
		for key, favorite := range s.favorites {
			if favorite.TargetType == "material" && favorite.TargetID == id {
				delete(s.favorites, key)
			}
		}
		s.prependLogDetail(operator, "删除学习资料", title, "")
		return nil
	}
	return errors.New("学习资料不存在")
}

func (s *MemoryStore) homeworkUnlocked(principal learning.Principal) []learning.Homework {
	courses := courseNames(s.coursesUnlocked(principal))
	return s.homeworkForCourses(courses)
}

func (s *MemoryStore) homeworkSubmissionsUnlocked(principal learning.Principal, homeworkID string) (learning.HomeworkSubmissionSummary, error) {
	homeworkID = strings.TrimSpace(homeworkID)
	var homework learning.Homework
	found := false
	for _, item := range s.homeworkUnlocked(principal) {
		if item.ID == homeworkID {
			homework = item
			found = true
			break
		}
	}
	if !found {
		return learning.HomeworkSubmissionSummary{}, errors.New("练习不存在或没有权限查看")
	}
	students := make([]learning.HomeworkSubmissionStudent, 0)
	studentIDs := map[string]bool{}
	for _, grant := range s.grants {
		if !grantActive(grant) {
			continue
		}
		pkg, ok := s.findPackage(grant.PackageID)
		if !ok || !s.packageOpensContent(pkg, homework.LearningSpaceID, "question") {
			continue
		}
		student, ok := s.findStudent(grant.StudentID)
		if !ok || studentIDs[student.ID] {
			continue
		}
		studentIDs[student.ID] = true
		row := learning.HomeworkSubmissionStudent{
			StudentID:    student.ID,
			StudentName:  student.Name,
			Phone:        student.Phone,
			ReviewStatus: "未提交",
		}
		if submission, ok := s.latestSubmissionForStudent(student.ID, homework.ID); ok {
			row.SubmittedAt = submission.CreatedAt
			row.ReviewStatus = submission.Status
			row.SubmissionID = submission.ID
		}
		students = append(students, row)
	}
	return learning.HomeworkSubmissionSummary{
		HomeworkID:    homework.ID,
		HomeworkTitle: homework.Title,
		TotalNum:      len(students),
		SubmittedNum:  countSubmittedStudents(students),
		Students:      students,
	}, nil
}

func (s *MemoryStore) notifyHomeworkPublished(homework learning.Homework) {
	students := s.expectedStudentsForHomework(homework)
	for _, student := range students {
		summary := homework.Title + "已发布，请按时完成。"
		if homework.Deadline != "" {
			summary = homework.Title + "已发布，截止时间 " + homework.Deadline + "。"
		}
		notice := learning.Notice{
			ID:              "notice-homework-" + homework.ID + "-" + student.ID,
			Type:            "练",
			Title:           homework.Subject + "练习已发布",
			Target:          student.Name,
			Summary:         summary,
			Channel:         "公众号模板消息",
			RecipientOpenID: student.OfficialAccountOpenID,
			RelatedType:     "homework",
			RelatedID:       homework.ID,
		}
		notice = s.deliverNotice(notice)
		s.prependNoticeRecord(notice)
	}
}

func (s *MemoryStore) expectedStudentsForHomework(homework learning.Homework) []learning.Student {
	students := make([]learning.Student, 0)
	seen := map[string]bool{}
	for _, grant := range s.grants {
		if !grantActive(grant) {
			continue
		}
		pkg, ok := s.findPackage(grant.PackageID)
		if !ok || !s.packageOpensContent(pkg, homework.LearningSpaceID, "question") {
			continue
		}
		student, ok := s.findStudent(grant.StudentID)
		if !ok || seen[student.ID] {
			continue
		}
		seen[student.ID] = true
		students = append(students, student)
	}
	return students
}

func (s *MemoryStore) questionsUnlocked(principal learning.Principal, query learning.QuestionBankQuery) []learning.QuestionBankItem {
	query.Grade = strings.TrimSpace(query.Grade)
	query.Semester = strings.TrimSpace(query.Semester)
	query.Subject = strings.TrimSpace(query.Subject)
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	out := make([]learning.QuestionBankItem, 0, len(s.questionBank))
	for _, item := range s.questionBank {
		if !canSeeQuestionScope(principal, item.Grade, item.Semester, item.Subject, s.learningSpaces) ||
			(query.Grade != "" && item.Grade != query.Grade) ||
			(query.Semester != "" && item.Semester != query.Semester) ||
			(query.Subject != "" && item.Subject != query.Subject) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(strings.Join([]string{item.Title, item.Stem, item.Grade, item.Semester, item.Subject, item.Type}, " ")), keyword) {
			continue
		}
		out = append(out, cloneQuestionBankItem(item))
	}
	return out
}

func (s *MemoryStore) createQuestionUnlocked(operator string, principal learning.Principal, req learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.QuestionBankItem, error) {
			return work.createQuestionUnlocked(operator, principal, req)
		})
	}
	item, err := s.questionFromRequest("qb-"+time.Now().Format("20060102150405.000000000"), principal, req)
	if err != nil {
		return learning.QuestionBankItem{}, err
	}
	item.OwnerTeacherID = principal.UserID
	item.OwnerTeacherName = principal.Name
	now := time.Now().Format("2006-01-02 15:04:05")
	item.CreatedAt = now
	item.UpdatedAt = now
	s.questionBank = append([]learning.QuestionBankItem{cloneQuestionBankItem(item)}, s.questionBank...)
	s.prependLog(operator, "新增题库题目", item.Grade+" "+item.Semester+" "+item.Subject)
	return cloneQuestionBankItem(item), nil
}

func (s *MemoryStore) updateQuestionUnlocked(operator string, principal learning.Principal, id string, req learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.QuestionBankItem, error) {
			return work.updateQuestionUnlocked(operator, principal, id, req)
		})
	}
	id = strings.TrimSpace(id)
	for index := range s.questionBank {
		if s.questionBank[index].ID != id {
			continue
		}
		if !canEditQuestion(principal, s.questionBank[index]) {
			return learning.QuestionBankItem{}, errors.New("只能编辑自己创建或有管理权限的题目")
		}
		item, err := s.questionFromRequest(id, principal, req)
		if err != nil {
			return learning.QuestionBankItem{}, err
		}
		item.OwnerTeacherID = s.questionBank[index].OwnerTeacherID
		item.OwnerTeacherName = s.questionBank[index].OwnerTeacherName
		item.CreatedAt = s.questionBank[index].CreatedAt
		item.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
		before := s.questionBank[index]
		s.questionBank[index] = cloneQuestionBankItem(item)
		s.refreshHomeworkQuestionSnapshots(item.ID)
		s.prependLogDetail(operator, "编辑题库题目", item.Stem, auditChangeDetail(map[string]any{"stem": before.Stem, "status": before.Status}, map[string]any{"stem": item.Stem, "status": item.Status}))
		return cloneQuestionBankItem(item), nil
	}
	return learning.QuestionBankItem{}, errors.New("题目不存在")
}

func (s *MemoryStore) questionFromRequest(id string, principal learning.Principal, req learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Grade = strings.TrimSpace(req.Grade)
	req.Semester = strings.TrimSpace(req.Semester)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Type = strings.TrimSpace(req.Type)
	req.Stem = sanitizeRichText(req.Stem)
	req.Answer = strings.TrimSpace(req.Answer)
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = string(learning.StatusEnabled)
	}
	if req.Grade == "" || req.Semester == "" || req.Subject == "" {
		return learning.QuestionBankItem{}, errors.New("请选择年级、学期和学科")
	}
	if !canSeeQuestionScope(principal, req.Grade, req.Semester, req.Subject, s.learningSpaces) {
		return learning.QuestionBankItem{}, errors.New("不能维护未负责范围的题库")
	}
	if req.Stem == "" {
		return learning.QuestionBankItem{}, errors.New("请输入题干")
	}
	if len([]rune(req.Stem)) > 4000 {
		return learning.QuestionBankItem{}, errors.New("题干最多 4000 个字")
	}
	if req.Title == "" {
		req.Title = shortQuestionTitle(req.Stem)
	}
	if req.Type != "single" && req.Type != "multiple" && req.Type != "judge" && req.Type != "fill" && req.Type != "text" {
		return learning.QuestionBankItem{}, errors.New("请选择正确的题型")
	}
	if !isContentStatus(learning.Status(status)) {
		return learning.QuestionBankItem{}, errors.New("请选择正确的发布状态")
	}
	options := cleanPhrases(req.Options)
	answers := cleanPhrases(req.Answers)
	if req.Type == "single" {
		if len(options) < 2 || req.Answer == "" {
			return learning.QuestionBankItem{}, errors.New("单选题需要至少两个选项和一个正确答案")
		}
		answers = []string{req.Answer}
	}
	if req.Type == "multiple" {
		if len(options) < 2 || len(answers) == 0 {
			return learning.QuestionBankItem{}, errors.New("多选题需要至少两个选项和正确答案")
		}
	}
	if req.Type == "judge" {
		options = []string{"正确", "错误"}
		if req.Answer != "正确" && req.Answer != "错误" {
			return learning.QuestionBankItem{}, errors.New("判断题请选择正确或错误")
		}
		answers = []string{req.Answer}
	}
	if req.Type == "fill" {
		options = nil
		if len(answers) == 0 && req.Answer != "" {
			answers = []string{req.Answer}
		}
		if len(answers) == 0 {
			return learning.QuestionBankItem{}, errors.New("填空题需要填写参考答案")
		}
		req.Answer = answers[0]
	}
	if req.Type == "text" {
		options = nil
		answers = nil
		req.Answer = ""
	}
	score := req.Score
	if score <= 0 {
		score = 10
	}
	return learning.QuestionBankItem{
		ID: id, Title: req.Title, Grade: req.Grade, Semester: req.Semester, Subject: req.Subject, Type: req.Type, Stem: req.Stem,
		Options: options, Answer: req.Answer, Answers: answers, Score: score, Status: status,
	}, nil
}

func (s *MemoryStore) createHomeworkUnlocked(operator string, principal learning.Principal, req learning.HomeworkUploadRequest) (learning.Homework, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Homework, error) {
			return work.createHomeworkUnlocked(operator, principal, req)
		})
	}
	req.Title = strings.TrimSpace(req.Title)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.LessonID = strings.TrimSpace(req.LessonID)
	req.Deadline = strings.TrimSpace(req.Deadline)
	req.DeadlineAt = strings.TrimSpace(req.DeadlineAt)
	req.AssessmentType = normalizeAssessmentType(req.AssessmentType)
	tagCode, err := normalizeContentTagCode(req.TagCode)
	if err != nil {
		return learning.Homework{}, err
	}
	tagCode = contentTagCodeOrInferred(tagCode, req.Title, req.File.FileName)
	if req.Title == "" {
		return learning.Homework{}, errors.New("请输入题目标题")
	}
	course, err := s.courseForUpload(principal, req.CourseID, req.LearningSpaceID)
	if err != nil {
		return learning.Homework{}, err
	}
	if !canUploadQuestion(principal) {
		return learning.Homework{}, errors.New("当前账号没有上传题目权限，请联系管理员开通")
	}
	curriculum, err := curriculumPathForLesson(course, req.LessonID)
	if err != nil {
		return learning.Homework{}, err
	}
	deadlineAt, err := normalizeDeadlineAt(req.DeadlineAt, req.Deadline, req.AssessmentType, true)
	if err != nil {
		return learning.Homework{}, err
	}
	questions, err := s.questionsForHomework(course, req.QuestionIDs)
	if err != nil {
		return learning.Homework{}, err
	}
	asset := req.File
	if asset.ID != "" {
		s.fileAssets[asset.ID] = asset
		s.enqueuePreviewJobUnlocked(asset.ID)
	}
	status := learning.Status(strings.TrimSpace(req.Status))
	if status == "" {
		status = learning.StatusEnabled
	}
	if !isContentStatus(status) {
		return learning.Homework{}, errors.New("请选择正确的发布状态")
	}
	item := learning.Homework{
		ID:               "homework-" + time.Now().Format("20060102150405.000000000"),
		Title:            req.Title,
		LessonID:         req.LessonID,
		Curriculum:       curriculum,
		TagCode:          tagCode,
		PackageName:      course.Subject + "题",
		CourseID:         course.ID,
		Course:           course.Name,
		LearningSpaceID:  course.LearningSpaceID,
		Grade:            course.Grade,
		Semester:         s.semesterForSpace(course.LearningSpaceID),
		Subject:          course.Subject,
		QuestionNum:      len(questions),
		QuestionIDs:      questionIDs(questions),
		Questions:        questions,
		Deadline:         req.Deadline,
		DeadlineAt:       deadlineAt,
		AssessmentType:   req.AssessmentType,
		OwnerTeacherID:   principal.UserID,
		OwnerTeacherName: principal.Name,
		PublishStatus:    publishStatus(status),
		Status:           string(status),
		FileID:           asset.ID,
		FileName:         asset.FileName,
		FileSize:         asset.FileSize,
		FileType:         asset.FileType,
		PreviewStatus:    asset.PreviewStatus,
		PreviewURL:       "/api/files/" + asset.ID + "/preview",
		DownloadURL:      "/api/files/" + asset.ID + "/download",
		SortOrder:        s.nextHomeworkSortOrder(course.ID),
	}
	s.homework = append([]learning.Homework{cloneHomework(item)}, s.homework...)
	if status == learning.StatusEnabled {
		s.notifyHomeworkPublished(item)
	}
	s.prependLog(operator, "上传题目", item.Title)
	return cloneHomework(item), nil
}

func (s *MemoryStore) updateHomeworkUnlocked(operator string, principal learning.Principal, id string, req learning.HomeworkUpdateRequest) (learning.Homework, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Homework, error) {
			return work.updateHomeworkUnlocked(operator, principal, id, req)
		})
	}
	id = strings.TrimSpace(id)
	req.Title = strings.TrimSpace(req.Title)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.LessonID = strings.TrimSpace(req.LessonID)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.Deadline = strings.TrimSpace(req.Deadline)
	req.DeadlineAt = strings.TrimSpace(req.DeadlineAt)
	req.AssessmentType = normalizeAssessmentType(req.AssessmentType)
	tagCode, err := normalizeContentTagCode(req.TagCode)
	if err != nil {
		return learning.Homework{}, err
	}
	tagCode = contentTagCodeOrInferred(tagCode, req.Title)
	status := learning.Status(strings.TrimSpace(req.Status))
	if req.Title == "" {
		return learning.Homework{}, errors.New("请输入题目标题")
	}
	if !isContentStatus(status) {
		return learning.Homework{}, errors.New("请选择正确的发布状态")
	}
	course, err := s.courseForUpload(principal, req.CourseID, req.LearningSpaceID)
	if err != nil {
		return learning.Homework{}, err
	}
	if !canUploadQuestion(principal) {
		return learning.Homework{}, errors.New("当前账号没有维护题目权限，请联系管理员开通")
	}
	curriculum, err := curriculumPathForLesson(course, req.LessonID)
	if err != nil {
		return learning.Homework{}, err
	}
	deadlineAt, err := normalizeDeadlineAt(req.DeadlineAt, req.Deadline, req.AssessmentType, false)
	if err != nil {
		return learning.Homework{}, err
	}
	questions, err := s.questionsForHomework(course, req.QuestionIDs)
	if err != nil {
		return learning.Homework{}, err
	}
	for index := range s.homework {
		if s.homework[index].ID != id {
			continue
		}
		if !canSeeCourse(principal, learning.Course{ID: s.homework[index].CourseID, LearningSpaceID: s.homework[index].LearningSpaceID}) {
			return learning.Homework{}, errors.New("不能维护未负责的题目")
		}
		before := s.homework[index]
		s.homework[index].Title = req.Title
		s.homework[index].LessonID = req.LessonID
		s.homework[index].Curriculum = curriculum
		s.homework[index].TagCode = tagCode
		s.homework[index].PackageName = course.Subject + "题"
		s.homework[index].CourseID = course.ID
		s.homework[index].Course = course.Name
		s.homework[index].LearningSpaceID = course.LearningSpaceID
		s.homework[index].Grade = course.Grade
		s.homework[index].Semester = s.semesterForSpace(course.LearningSpaceID)
		s.homework[index].Subject = course.Subject
		s.homework[index].QuestionIDs = questionIDs(questions)
		s.homework[index].Questions = questions
		s.homework[index].QuestionNum = len(questions)
		s.homework[index].Deadline = req.Deadline
		s.homework[index].DeadlineAt = deadlineAt
		s.homework[index].AssessmentType = req.AssessmentType
		s.homework[index].Status = string(status)
		s.homework[index].PublishStatus = publishStatus(status)
		if before.Status != string(learning.StatusEnabled) && status == learning.StatusEnabled {
			s.notifyHomeworkPublished(s.homework[index])
		}
		s.prependLogDetail(operator, "编辑题目", req.Title, auditChangeDetail(homeworkAuditSnapshot(before), homeworkAuditSnapshot(s.homework[index])))
		return cloneHomework(s.homework[index]), nil
	}
	return learning.Homework{}, errors.New("题目不存在")
}

// deleteHomeworkUnlocked 只允许删除尚未产生学生提交的练习。已提交练习承载
// 学生答案和批改记录，改为停用才能保留学习历史，避免误删学习成果。
func (s *MemoryStore) deleteHomeworkUnlocked(operator string, principal learning.Principal, id string) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error {
			return work.deleteHomeworkUnlocked(operator, principal, id)
		})
	}
	id = strings.TrimSpace(id)
	if !canUploadQuestion(principal) {
		return errors.New("当前账号没有维护题目权限，请联系管理员开通")
	}
	for index := range s.homework {
		if s.homework[index].ID != id {
			continue
		}
		if !canSeeCourse(principal, learning.Course{ID: s.homework[index].CourseID, LearningSpaceID: s.homework[index].LearningSpaceID}) {
			return errors.New("不能删除未负责的题目")
		}
		for _, submission := range s.submissions {
			if submission.HomeworkID == id {
				return errors.New("已有学生提交记录，不能删除练习；请改为停用")
			}
		}
		title := s.homework[index].Title
		s.homework = append(s.homework[:index:index], s.homework[index+1:]...)
		s.prependLogDetail(operator, "删除课后练习", title, "")
		return nil
	}
	return errors.New("课后练习不存在")
}

func (s *MemoryStore) contentFileUnlocked(principal learning.Principal, fileID string) (learning.FileAsset, error) {
	asset, ok := s.fileAssets[fileID]
	if !ok {
		return learning.FileAsset{}, errors.New("文件不存在")
	}
	for _, material := range s.materialsUnlocked(principal) {
		if material.FileID == fileID {
			return asset, nil
		}
	}
	for _, item := range s.homeworkUnlocked(principal) {
		if item.FileID == fileID {
			return asset, nil
		}
	}
	return learning.FileAsset{}, errors.New("没有权限查看该文件")
}
func (s *MemoryStore) reviewsUnlocked(principal learning.Principal) []learning.Review {
	if hasRole(principal.Roles, learning.RoleOpsStaff) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleSuperAdmin) {
		return append([]learning.Review(nil), s.reviews...)
	}
	if hasRole(principal.Roles, learning.RoleTeacher) {
		out := make([]learning.Review, 0, len(s.reviews))
		subjects := subjectsForCourses(s.coursesUnlocked(principal))
		for _, review := range s.reviews {
			if review.ReviewerTeacherID == principal.UserID {
				out = append(out, review)
				continue
			}
			// 存量任务还没有责任快照时，暂按原课程范围展示，避免迁移
			// 当天出现无人可批的历史作业。所有新任务一旦有分派信息，
			// 都只按 reviewerTeacherID 路由。
			if review.ReviewerTeacherID == "" && (canSeeSubject(principal, subjects, review.PackageName) || canSeeSubject(principal, subjects, review.Homework)) {
				out = append(out, review)
			}
		}
		return out
	}
	subjects := subjectsForCourses(s.coursesUnlocked(principal))
	out := make([]learning.Review, 0, len(s.reviews))
	for _, review := range s.reviews {
		if canSeeSubject(principal, subjects, review.PackageName) || canSeeSubject(principal, subjects, review.Homework) {
			out = append(out, review)
		}
	}
	return out
}

func (s *MemoryStore) assignReviewUnlocked(operator string, principal learning.Principal, id string, req learning.ReviewAssignRequest) (learning.Review, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Review, error) {
			return work.assignReviewUnlocked(operator, principal, id, req)
		})
	}
	if !canManageTutoringAssignments(principal) {
		return learning.Review{}, errors.New("没有权限分派批改任务")
	}
	req.TeacherID = strings.TrimSpace(req.TeacherID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.TeacherID == "" || req.Reason == "" {
		return learning.Review{}, errors.New("请选择老师并填写分派原因")
	}
	teacher, ok := s.findUser(req.TeacherID)
	if !ok || !isActiveTeacher(teacher) || !teacher.CanReview {
		return learning.Review{}, errors.New("该老师不可承担批改任务")
	}
	if !canManageTeacher(principal, teacher) && !hasRole(principal.Roles, learning.RoleOpsStaff) {
		return learning.Review{}, errors.New("不能分派给其他校区老师")
	}
	for index := range s.reviews {
		review := &s.reviews[index]
		if review.ID != strings.TrimSpace(id) {
			continue
		}
		if review.Status != "待批改" && review.Status != "待复核" {
			return learning.Review{}, errors.New("该任务当前不可重新分派")
		}
		review.ReviewerTeacherID = teacher.ID
		review.ReviewerTeacherName = teacher.Name
		review.TutoringAssignmentID = ""
		review.AssignedAt = time.Now().Format("2006-01-02 15:04:05")
		s.prependLogDetail(operator, "分派批改任务", review.StudentName, "老师 "+teacher.Name+" / 原因 "+req.Reason)
		return *review, nil
	}
	return learning.Review{}, errors.New("待批改记录不存在")
}

func (s *MemoryStore) completeReviewUnlocked(operator string, principal learning.Principal, id string, req learning.ReviewCompleteRequest) (learning.Submission, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Submission, error) {
			return work.completeReviewUnlocked(operator, principal, id, req)
		})
	}
	if !canReviewHomework(principal) {
		return learning.Submission{}, errors.New("当前账号没有批改权限，请联系管理员开通")
	}
	id = strings.TrimSpace(id)
	req.TeacherComment = sanitizeRichText(req.TeacherComment)
	req.Reward = strings.TrimSpace(req.Reward)
	req.FinalStatus = normalizeReviewFinalStatus(req.FinalStatus)
	if req.FinalStatus == "" {
		return learning.Submission{}, errors.New("批改状态只能为待复核或已批改")
	}
	if req.Score < 0 || req.Score > 100 {
		return learning.Submission{}, errors.New("分数需在 0 到 100 之间")
	}
	if req.TeacherComment == "" {
		return learning.Submission{}, errors.New("请填写给学生看的评语")
	}
	reviewIndex := -1
	var review learning.Review
	for index, item := range s.reviews {
		if item.ID == id {
			reviewIndex = index
			review = item
			break
		}
	}
	if reviewIndex < 0 {
		return learning.Submission{}, errors.New("待批改记录不存在")
	}
	visible := false
	for _, item := range s.reviewsUnlocked(principal) {
		if item.ID == id {
			visible = true
			break
		}
	}
	if !visible {
		return learning.Submission{}, errors.New("没有权限批改该练习")
	}
	if review.StudentID == "" || review.HomeworkID == "" {
		return learning.Submission{}, errors.New("待批改记录缺少学生或题目信息")
	}
	homework, ok := s.findHomework(review.HomeworkID)
	if !ok {
		return learning.Submission{}, errors.New("题目不存在")
	}
	if req.Reward == "" {
		req.Reward = rewardForScore(req.Score)
	}
	submission, ok := s.submissions[review.SubmissionID]
	if !ok {
		submission = learning.Submission{
			ID:         "sub-review-" + id,
			HomeworkID: homework.ID,
			StudentID:  review.StudentID,
			TaskTitle:  homework.Title,
			CreatedAt:  time.Now().Format("2006-01-02 15:04:05"),
		}
	}
	submission.Score = req.Score
	submission.FinalScore = req.Score
	if submission.ObjectiveScore == 0 {
		submission.ObjectiveScore = review.SystemScore
	}
	submission.TeacherComment = req.TeacherComment
	submission.Reward = req.Reward
	submission.Status = req.FinalStatus
	s.submissions[submission.ID] = cloneSubmission(submission)
	if req.FinalStatus == "待复核" {
		review.SubmissionID = submission.ID
		review.SystemScore = req.Score
		review.TeacherComment = req.TeacherComment
		review.Reward = req.Reward
		review.Status = "待复核"
		s.reviews[reviewIndex] = review
	} else {
		s.reviews = append(s.reviews[:reviewIndex], s.reviews[reviewIndex+1:]...)
	}
	notice := learning.Notice{
		ID:              "notice-review-" + time.Now().Format("20060102150405.000000000"),
		Type:            "评",
		Title:           reviewNoticeTitle(req.FinalStatus),
		Target:          review.StudentName,
		Summary:         reviewNoticeSummary(homework.Title, req.FinalStatus),
		Channel:         "公众号模板消息",
		RecipientOpenID: s.officialAccountOpenIDForTarget(review.StudentName),
		RelatedType:     "review",
		RelatedID:       submission.ID,
	}
	notice = s.deliverNotice(notice)
	s.prependNoticeRecord(notice)
	s.prependLog(operator, reviewLogAction(req.FinalStatus), review.StudentName+" · "+homework.Title)
	return cloneSubmission(submission), nil
}
