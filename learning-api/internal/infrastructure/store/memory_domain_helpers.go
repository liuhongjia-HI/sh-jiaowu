package store

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) findStudent(id string) (learning.Student, bool) {
	for _, student := range s.students {
		if student.ID == id {
			return s.decorateStudent(student), true
		}
	}
	return learning.Student{}, false
}

func (s *MemoryStore) visibleStudent(principal learning.Principal, id string) (learning.Student, error) {
	student, ok := s.findStudent(id)
	if !ok {
		return learning.Student{}, errors.New("student not found")
	}
	if !s.canSeeStudent(principal, student, s.coursesForStudent(student.ID)) {
		return learning.Student{}, errors.New("没有权限访问该学生")
	}
	return student, nil
}

func (s *MemoryStore) decorateStudent(student learning.Student) learning.Student {
	applyDerivedGrade(&student, s.configuredAcademicYear())
	student.ActiveTutoringAssignments = s.activeTutoringAssignmentsForStudent(student.ID)
	student.AverageScore = s.studentAverageScore(student.ID)
	if user, ok := s.findUserByStudentID(student.ID); ok && strings.TrimSpace(user.OpenID) != "" {
		student.BindStatus = "已绑定"
	} else if student.BindStatus == "" {
		student.BindStatus = "待绑定"
	}
	effectiveUntil := ""
	packages := make([]string, 0)
	packageRefs := make([]learning.StudentPackageRef, 0)
	hasActiveGrant := false
	for _, grant := range s.grants {
		if grant.StudentID != student.ID || grant.Status == "revoked" {
			continue
		}
		if grantActive(grant) {
			hasActiveGrant = true
		}
		if grantEndsAt(grant) > effectiveUntil {
			effectiveUntil = grantEndsAt(grant)
		}
		if pkg, ok := s.findPackage(grant.PackageID); ok {
			packages = appendUnique(packages, pkg.Name)
			packageRefs = append(packageRefs, learning.StudentPackageRef{PackageID: pkg.ID, PackageName: pkg.Name})
		}
	}
	if effectiveUntil != "" {
		student.EffectiveUntil = effectiveUntil
	}
	student.OpenedPackages = packages
	student.OpenedPackageRefs = packageRefs
	student.FollowUpStatus = ""
	if len(student.OpenedPackages) == 0 {
		student.FollowUpStatus = "待跟进"
	}
	if hasActiveGrant && student.LearningStatus == "待开通" {
		student.LearningStatus = "已开通"
	}
	if submission, ok := s.latestStudentSubmission(student.ID); ok {
		student.LastSubmittedAt = submission.CreatedAt
		student.LastSubmissionStatus = submission.Status
	}
	return student
}

func (s *MemoryStore) activeTutoringAssignmentsForStudent(studentID string) []learning.TutoringAssignmentSummary {
	assignments := make([]learning.TutoringAssignmentSummary, 0)
	for _, assignment := range s.tutoringAssignments {
		if assignment.StudentID != studentID || assignment.Status != learning.TutoringAssignmentActive {
			continue
		}
		teacherName := assignment.TeacherName
		if teacher, ok := s.findUser(assignment.TeacherID); ok && strings.TrimSpace(teacher.Name) != "" {
			teacherName = teacher.Name
		}
		assignments = append(assignments, learning.TutoringAssignmentSummary{
			TeacherID: assignment.TeacherID, TeacherName: teacherName, SubjectName: assignment.SubjectName,
			LevelCode: assignment.LevelCode, Role: assignment.Role, StartsAt: assignment.StartsAt,
		})
	}
	sort.SliceStable(assignments, func(i, j int) bool {
		if assignments[i].Role != assignments[j].Role {
			return assignments[i].Role == learning.TutoringAssignmentPrimary
		}
		if assignments[i].TeacherName != assignments[j].TeacherName {
			return assignments[i].TeacherName < assignments[j].TeacherName
		}
		return assignments[i].SubjectName < assignments[j].SubjectName
	})
	return assignments
}

// studentAverageScore 用学生已批改小挑战的真实得分算平均分，不是一个可以手工填写、
// 永远不更新的静态字段。没有任何已批改记录时返回 0，前端据此显示“完成后生成”，
// 这是诚实的“还没有数据”，不是坏掉的字段。
func (s *MemoryStore) studentAverageScore(studentID string) int {
	total := 0
	count := 0
	for _, submission := range s.submissions {
		if submission.StudentID != studentID || submission.Status != "已批改" {
			continue
		}
		total += submission.FinalScore
		count++
	}
	if count == 0 {
		return 0
	}
	return int(math.Round(float64(total) / float64(count)))
}

func (s *MemoryStore) latestStudentSubmission(studentID string) (learning.Submission, bool) {
	var latest learning.Submission
	found := false
	for _, submission := range s.submissions {
		if submission.StudentID != studentID {
			continue
		}
		if !found || submission.CreatedAt > latest.CreatedAt {
			latest = submission
			found = true
		}
	}
	return latest, found
}

func (s *MemoryStore) normalizeScoreRequest(principal learning.Principal, student learning.Student, req learning.StudentScoreUpsertRequest) (learning.StudentScoreUpsertRequest, error) {
	req.Subject = strings.TrimSpace(req.Subject)
	req.ExamType = strings.TrimSpace(req.ExamType)
	req.ExamName = strings.TrimSpace(req.ExamName)
	req.ExamDate = strings.TrimSpace(req.ExamDate)
	req.TeacherComment = sanitizeRichText(req.TeacherComment)
	if req.Subject == "" {
		return req, errors.New("请选择学科")
	}
	if req.ExamType == "" {
		req.ExamType = "阶段测评"
	}
	if !validExamType(req.ExamType) {
		return req, errors.New("考试类型只能为期中、期末、单元测、模拟考或阶段测评")
	}
	if req.ExamName == "" {
		return req, errors.New("请输入考试或测评名称")
	}
	if req.ExamDate == "" {
		return req, errors.New("请选择考试日期")
	}
	if _, err := time.Parse("2006-01-02", req.ExamDate); err != nil {
		return req, errors.New("考试日期格式应为 YYYY-MM-DD")
	}
	if req.FullScore <= 0 {
		return req, errors.New("满分必须大于 0")
	}
	if req.Score < 0 {
		return req, errors.New("分数不能小于 0")
	}
	if req.Score > req.FullScore {
		return req, errors.New("分数不能大于满分")
	}
	if req.AverageScore < 0 {
		return req, errors.New("平均分不能小于 0")
	}
	if req.AverageScore > req.FullScore {
		return req, errors.New("平均分不能大于满分")
	}
	if len([]rune(req.ExamName)) > 64 {
		return req, errors.New("考试或测评名称最多 64 个字")
	}
	if len([]rune(req.TeacherComment)) > 1000 {
		return req, errors.New("老师建议最多 1000 个字")
	}
	if !s.canWriteScoreSubject(principal, student, req.Subject) {
		return req, errors.New("没有权限录入该学生这个学科的成绩")
	}
	return req, nil
}

func (s *MemoryStore) canWriteScoreSubject(principal learning.Principal, student learning.Student, subject string) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if !hasRole(principal.Roles, learning.RoleTeacher) {
		return false
	}
	for _, space := range s.learningSpaces {
		if containsString(principal.LearningSpaceIDs, space.ID) && space.Grade == student.Grade && subjectsMatch(space.Subject, subject) && space.Status == learning.StatusEnabled {
			return true
		}
	}
	return false
}

func validExamType(value string) bool {
	switch value {
	case "期中", "期末", "单元测", "模拟考", "阶段测评":
		return true
	default:
		return false
	}
}

func (s *MemoryStore) scoreSummariesForStudent(studentID string) []learning.StudentScoreSummary {
	bySubject := map[string][]learning.StudentScoreRecord{}
	for _, item := range s.scoreRecords {
		if item.StudentID != studentID {
			continue
		}
		bySubject[item.Subject] = append(bySubject[item.Subject], item)
	}
	subjects := make([]string, 0, len(bySubject))
	for subject := range bySubject {
		subjects = append(subjects, subject)
	}
	sort.Strings(subjects)
	out := make([]learning.StudentScoreSummary, 0, len(subjects))
	for _, subject := range subjects {
		records := append([]learning.StudentScoreRecord(nil), bySubject[subject]...)
		sort.Slice(records, func(i, j int) bool {
			if records[i].ExamDate == records[j].ExamDate {
				return records[i].CreatedAt < records[j].CreatedAt
			}
			return records[i].ExamDate < records[j].ExamDate
		})
		summary := learning.StudentScoreSummary{Subject: subject, Records: records}
		if len(records) > 0 {
			first := records[0]
			latest := records[len(records)-1]
			summary.FirstRecord = &first
			summary.LatestRecord = &latest
			summary.Improvement = latest.Score - first.Score
			if first.FullScore > 0 && latest.FullScore > 0 {
				firstPct := first.Score * 100 / first.FullScore
				latestPct := latest.Score * 100 / latest.FullScore
				summary.ImprovementPct = latestPct - firstPct
			}
			summary.Description = scoreSummaryDescription(summary)
			summary.ProblemPoint = scoreProblemPoint(summary)
			summary.NextStep = scoreNextStep(summary)
		}
		sort.Slice(summary.Records, func(i, j int) bool {
			if summary.Records[i].ExamDate == summary.Records[j].ExamDate {
				return summary.Records[i].CreatedAt > summary.Records[j].CreatedAt
			}
			return summary.Records[i].ExamDate > summary.Records[j].ExamDate
		})
		out = append(out, summary)
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := out[i].LatestRecord, out[j].LatestRecord
		if left == nil || right == nil {
			return left != nil
		}
		if left.ExamDate == right.ExamDate {
			return left.CreatedAt > right.CreatedAt
		}
		return left.ExamDate > right.ExamDate
	})
	return out
}

func scoreSummaryDescription(summary learning.StudentScoreSummary) string {
	if summary.LatestRecord == nil {
		return ""
	}
	latest := *summary.LatestRecord
	if summary.FirstRecord == nil || summary.FirstRecord.ID == latest.ID {
		if latest.TeacherComment != "" {
			return latest.Subject + "最近 " + itoa(latest.Score) + " 分。" + richTextPlainText(latest.TeacherComment)
		}
		return latest.Subject + "最近 " + itoa(latest.Score) + " 分。"
	}
	first := *summary.FirstRecord
	change := summary.Improvement
	changeText := "提升 " + itoa(change) + " 分"
	if change < 0 {
		changeText = "下降 " + itoa(0-change) + " 分"
	} else if change == 0 {
		changeText = "保持稳定"
	}
	text := latest.Subject + "最近 " + itoa(latest.Score) + " 分，比" + first.ExamName + changeText + "。"
	if latest.TeacherComment != "" {
		text += richTextPlainText(latest.TeacherComment)
	}
	return text
}

func scoreProblemPoint(summary learning.StudentScoreSummary) string {
	if summary.LatestRecord == nil || summary.LatestRecord.FullScore <= 0 {
		return ""
	}
	latest := *summary.LatestRecord
	latestPct := latest.Score * 100 / latest.FullScore
	if latest.AverageScore > 0 && latest.Score < latest.AverageScore {
		return latest.Subject + "最近低于班级平均分，优先复盘失分集中的题型。"
	}
	if summary.FirstRecord != nil && summary.FirstRecord.ID != latest.ID && summary.Improvement < 0 {
		return latest.Subject + "最近成绩有回落，需要检查基础题稳定性和考试节奏。"
	}
	if latestPct < 80 {
		return latest.Subject + "基础掌握还不够稳，需要先补核心概念和常见题型。"
	}
	if latestPct < 90 {
		return latest.Subject + "整体已达标，主要问题在准确率和细节失分。"
	}
	return latest.Subject + "表现稳定，下一步重点放在难题突破和表达完整度。"
}

func scoreNextStep(summary learning.StudentScoreSummary) string {
	if summary.LatestRecord == nil {
		return ""
	}
	latest := *summary.LatestRecord
	if latest.TeacherComment != "" {
		return latest.TeacherComment
	}
	if latest.FullScore > 0 {
		latestPct := latest.Score * 100 / latest.FullScore
		if latestPct < 80 {
			return "先完成错题订正，再做同类基础题巩固。"
		}
		if latestPct < 90 {
			return "保持当前节奏，每周集中处理易错题和审题问题。"
		}
	}
	return "继续保持练习频率，增加综合题和限时训练。"
}

func (s *MemoryStore) findUserByStudentID(studentID string) (learning.User, bool) {
	for _, user := range s.users {
		if user.StudentID == studentID {
			return user, true
		}
	}
	return learning.User{}, false
}

func (s *MemoryStore) findUserIndexByStudentID(studentID string) int {
	for i, user := range s.users {
		if user.StudentID == studentID {
			return i
		}
	}
	return -1
}

func (s *MemoryStore) phoneExists(currentStudentID, phone string) bool {
	for _, student := range s.students {
		if student.ID != currentStudentID && phoneSame(student.Phone, phone) {
			return true
		}
	}
	for _, user := range s.users {
		if user.StudentID != currentStudentID && phoneSame(user.Phone, phone) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) studentAdminPhoneConflicts(phone string) bool {
	for _, user := range s.users {
		if user.StudentID == "" && phoneSame(user.Phone, phone) {
			return true
		}
	}
	return false
}

func phoneSame(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return left == right || left == maskPhone(right) || maskPhone(left) == right
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len([]rune(phone)) != 11 || strings.Contains(phone, "*") {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}

func appendUniqueRoles(values []learning.Role, additions ...learning.Role) []learning.Role {
	seen := make(map[learning.Role]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if value == "" || seen[value] {
			continue
		}
		values = append(values, value)
		seen[value] = true
	}
	return values
}

func (s *MemoryStore) syncStudentUser(student learning.Student) {
	for i := range s.users {
		if s.users[i].StudentID != student.ID {
			continue
		}
		s.users[i].Name = student.Name
		if !strings.Contains(student.Phone, "*") {
			s.users[i].Phone = student.Phone
		}
		s.users[i].AccountStatus = student.AccountStatus
		return
	}
	s.users = append(s.users, learning.User{
		ID:            "user-" + student.ID,
		Name:          student.Name,
		Phone:         student.Phone,
		AccountStatus: student.AccountStatus,
		Roles:         []learning.Role{learning.RoleStudent},
		StudentID:     student.ID,
	})
}

func (s *MemoryStore) permissionForStudent(student learning.Student) learning.StudentPermissionSummary {
	packages := make([]string, 0)
	courses := make([]string, 0)
	materials := make([]string, 0)
	homework := make([]string, 0)
	learningSpaces := make([]string, 0)
	contentTypes := make([]string, 0)
	effectiveUntil := ""
	hasActive := false
	hasUpcoming := false
	for _, grant := range s.grants {
		if grant.StudentID != student.ID || grant.Status == "revoked" || grantPeriodExpired(grantEndsAt(grant)) {
			continue
		}
		pkg, ok := s.findPackage(grant.PackageID)
		if !ok {
			continue
		}
		packages = appendUnique(packages, pkg.Name)
		contentTypes = appendUnique(contentTypes, s.contentTypeLabelsForPackage(pkg.ID)...)
		if grantActive(grant) {
			hasActive = true
			learningSpaces = appendUnique(learningSpaces, s.learningSpaceNamesForGrant(grant.ID)...)
			pkgCourses, pkgMaterials, pkgHomework := s.openContentForStudentGrant(grant)
			courses = appendUnique(courses, pkgCourses...)
			materials = appendUnique(materials, pkgMaterials...)
			homework = appendUnique(homework, pkgHomework...)
		} else if grantPermissionState(grant) == "未开始" {
			hasUpcoming = true
			learningSpaces = appendUnique(learningSpaces, s.learningSpaceNamesForPackage(pkg.ID)...)
		}
		if grantEndsAt(grant) > effectiveUntil {
			effectiveUntil = grantEndsAt(grant)
		}
	}
	state := "未开通"
	if hasActive {
		state = "生效中"
	} else if hasUpcoming {
		state = "未开始"
	}
	return learning.StudentPermissionSummary{
		StudentID: student.ID, StudentName: student.Name, Grade: student.Grade, AccountStatus: student.AccountStatus,
		OpenedPackages: packages, LearningSpaces: learningSpaces, ContentTypes: contentTypes,
		OpenCourses: courses, OpenMaterials: materials, OpenHomework: homework,
		EffectiveUntil: effectiveUntil, PermissionState: state,
	}
}

func (s *MemoryStore) noticesForStudent(student learning.Student) []learning.Notice {
	out := make([]learning.Notice, 0)
	courses := s.coursesForStudent(student.ID)
	subjects := subjectsForCourses(courses)
	for _, notice := range s.notices {
		if studentNoticeVisible(notice) && noticeMatchesStudent(notice, student, subjects) {
			out = append(out, notice)
		}
	}
	return out
}

func studentNoticeVisible(notice learning.Notice) bool {
	if notice.Channel == "公众号模板消息" {
		return false
	}
	status := strings.TrimSpace(notice.Status)
	return status == "已发送" || status == "自动发送"
}

func noticeMatchesStudent(notice learning.Notice, student learning.Student, subjects []string) bool {
	// 只要通知带有明确的学生关联，就以关联关系为准，不能再回退到
	// 姓名/年级/课程文本匹配，避免多孩子家庭或同名学生串收通知。
	if strings.EqualFold(strings.TrimSpace(notice.RelatedType), "student") && strings.TrimSpace(notice.RelatedID) != "" {
		return strings.TrimSpace(notice.RelatedID) == student.ID
	}
	target := notice.Target + " " + notice.Title + " " + notice.Summary
	if strings.Contains(notice.Target, "全部") || strings.Contains(target, student.Name) || strings.Contains(target, student.Grade) {
		return true
	}
	if student.Phone != "" && strings.Contains(target, student.Phone) {
		return true
	}
	for _, pkg := range student.OpenedPackages {
		if pkg != "" && strings.Contains(target, pkg) {
			return true
		}
	}
	for _, subject := range subjects {
		if subject != "" && subjectTextContains(target, subject) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) logsForStudent(student learning.Student) []learning.OperationLog {
	out := make([]learning.OperationLog, 0)
	for _, log := range s.logs {
		if strings.Contains(log.Target, student.Name) {
			out = append(out, log)
		}
	}
	return out
}

func (s *MemoryStore) findPackage(id string) (learning.Package, bool) {
	for _, pkg := range s.packages {
		if pkg.ID == id {
			return s.decoratePackage(pkg), true
		}
	}
	return learning.Package{}, false
}

func (s *MemoryStore) findHomework(id string) (learning.Homework, bool) {
	for _, item := range s.homework {
		if item.ID == id {
			return cloneHomework(item), true
		}
	}
	return learning.Homework{}, false
}

func (s *MemoryStore) questionsForHomework(course learning.Course, ids []string) ([]learning.Question, error) {
	if len(ids) == 0 {
		return []learning.Question{}, nil
	}
	space, ok := s.findLearningSpace(course.LearningSpaceID)
	if !ok {
		return nil, errors.New("请选择正确的课程范围")
	}
	out := make([]learning.Question, 0, len(ids))
	for _, id := range ids {
		item, ok := s.findQuestionBankItem(id)
		if !ok {
			return nil, errors.New("题库题目不存在")
		}
		if item.Status != string(learning.StatusEnabled) {
			return nil, errors.New("只能选择启用的题库题目")
		}
		if item.Grade != space.Grade || item.Semester != space.Semester || !subjectsMatch(item.Subject, space.Subject) {
			return nil, errors.New("题目范围必须和发布课程范围一致")
		}
		out = append(out, bankItemQuestion(item))
	}
	return out, nil
}

func (s *MemoryStore) findQuestionBankItem(id string) (learning.QuestionBankItem, bool) {
	for _, item := range s.questionBank {
		if item.ID == id {
			return cloneQuestionBankItem(item), true
		}
	}
	return learning.QuestionBankItem{}, false
}

func (s *MemoryStore) semesterForSpace(id string) string {
	if space, ok := s.findLearningSpace(id); ok {
		return space.Semester
	}
	return ""
}

func (s *MemoryStore) refreshHomeworkQuestionSnapshots(questionID string) {
	for index := range s.homework {
		if !containsString(s.homework[index].QuestionIDs, questionID) {
			continue
		}
		questions := make([]learning.Question, 0, len(s.homework[index].QuestionIDs))
		for _, id := range s.homework[index].QuestionIDs {
			if item, ok := s.findQuestionBankItem(id); ok {
				questions = append(questions, bankItemQuestion(item))
			}
		}
		s.homework[index].Questions = questions
		s.homework[index].QuestionNum = len(questions)
	}
}

func (s *MemoryStore) packageFromRequest(id string, req learning.PackageUpsertRequest) (learning.Package, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.AcademicYear = strings.TrimSpace(req.AcademicYear)
	req.Grade = strings.TrimSpace(req.Grade)
	req.Semester = strings.TrimSpace(req.Semester)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Level = strings.TrimSpace(req.Level)
	req.PhaseScope = strings.TrimSpace(req.PhaseScope)
	req.PackageType = strings.TrimSpace(req.PackageType)
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Name == "" {
		return learning.Package{}, errors.New("请输入学习套餐名称")
	}
	if req.AcademicYear == "" {
		// 跟随系统设置里的当前学年。学习空间不再参与学年匹配（见
		// learningSpaceMatches），所以套餐可以自由归属任意学年，不会因为
		// 学年跨年而绑不上学习空间。
		req.AcademicYear = s.configuredAcademicYear()
	}
	if req.Grade == "" || req.Subject == "" || req.Semester == "" {
		return learning.Package{}, errors.New("请选择年级、学科和学期")
	}
	if req.PhaseScope == "" {
		req.PhaseScope = "全学期"
	}
	if req.PackageType == "" {
		req.PackageType = packageTypeLabel(req.ContentTypeCodes)
	}
	if req.Status == "" {
		req.Status = learning.StatusEnabled
	}
	if req.Status != learning.StatusEnabled && req.Status != learning.StatusDraft && req.Status != learning.StatusDisabled {
		return learning.Package{}, errors.New("套餐状态不正确")
	}
	if len(req.LearningSpaceIDs) == 0 {
		return learning.Package{}, errors.New("请选择套餐开放的学习空间")
	}
	selectedLevel := ""
	for _, spaceID := range req.LearningSpaceIDs {
		space, exists := s.findLearningSpace(spaceID)
		if !exists {
			return learning.Package{}, errors.New("学习空间不存在：" + spaceID)
		}
		if !s.learningSpaceMatches(spaceID, req.Grade, req.Subject, req.Semester) {
			return learning.Package{}, errors.New("学习空间需与套餐年级、学科和学期一致")
		}
		spaceLevel := strings.TrimSpace(space.Level)
		if spaceLevel == "" {
			spaceLevel = "S"
		}
		if selectedLevel == "" {
			selectedLevel = spaceLevel
		}
		if selectedLevel != spaceLevel {
			return learning.Package{}, errors.New("同一套餐不能混合不同等级的学习空间")
		}
	}
	if req.Level == "" {
		req.Level = selectedLevel
	}
	if !validLearningLevel(req.Level) {
		return learning.Package{}, errors.New("请选择正确的课程等级")
	}
	if req.Level != selectedLevel {
		return learning.Package{}, errors.New("套餐等级需与学习空间等级一致")
	}
	if len(req.ContentTypeCodes) == 0 {
		return learning.Package{}, errors.New("请选择套餐开放的内容类型")
	}
	for _, code := range req.ContentTypeCodes {
		if !validContentType(code) {
			return learning.Package{}, errors.New("内容类型不正确：" + code)
		}
	}
	return learning.Package{
		ID:           id,
		Name:         req.Name,
		AcademicYear: req.AcademicYear,
		Grade:        req.Grade,
		Semester:     req.Semester,
		Subject:      req.Subject,
		Level:        req.Level,
		PhaseScope:   req.PhaseScope,
		PackageType:  req.PackageType,
		Summary:      req.Summary,
		TrialEnabled: req.TrialEnabled,
		Status:       req.Status,
	}, nil
}

func validLearningLevel(level string) bool {
	switch level {
	case "S", "S+", "H", "H+":
		return true
	default:
		return false
	}
}

func (s *MemoryStore) packageNameExists(currentID, name string) bool {
	for _, pkg := range s.packages {
		if pkg.ID != currentID && pkg.Name == name {
			return true
		}
	}
	return false
}

func (s *MemoryStore) courseFromRequest(principal learning.Principal, id string, req learning.CourseUpsertRequest) (learning.Course, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	if req.Name == "" {
		return learning.Course{}, errors.New("请输入课程名称")
	}
	if req.LearningSpaceID == "" {
		return learning.Course{}, errors.New("请选择课程所属学习空间")
	}
	curriculum, err := normalizeCurriculum(req.Curriculum)
	if err != nil {
		return learning.Course{}, err
	}
	if req.Status == "" {
		req.Status = learning.StatusEnabled
	}
	if req.Status != learning.StatusEnabled && req.Status != learning.StatusDraft && req.Status != learning.StatusDisabled {
		return learning.Course{}, errors.New("课程状态不正确")
	}
	space, ok := s.findLearningSpace(req.LearningSpaceID)
	if !ok {
		return learning.Course{}, errors.New("学习空间不存在")
	}
	course := learning.Course{
		ID:              id,
		Name:            req.Name,
		Subject:         space.Subject,
		Grade:           space.Grade,
		LearningSpaceID: space.ID,
		LessonCount:     countCurriculumLessons(curriculum),
		Curriculum:      curriculum,
		Status:          req.Status,
	}
	if !canSeeCourse(principal, course) {
		return learning.Course{}, errors.New("不能维护未负责的课程范围")
	}
	return course, nil
}

func normalizeCurriculum(nodes []learning.CurriculumNode) ([]learning.CurriculumNode, error) {
	if len(nodes) == 0 {
		return nil, errors.New("请至少维护一个 Unit、Chapter 和 Lesson")
	}
	result := make([]learning.CurriculumNode, 0, len(nodes))
	byID := make(map[string]learning.CurriculumNode, len(nodes))
	nameByParent := make(map[string]map[string]bool)
	for index, node := range nodes {
		node.ID = strings.TrimSpace(node.ID)
		node.ParentID = strings.TrimSpace(node.ParentID)
		node.Name = strings.TrimSpace(node.Name)
		if node.ID == "" || node.Name == "" {
			return nil, errors.New("目录节点名称不能为空")
		}
		if _, exists := byID[node.ID]; exists {
			return nil, errors.New("目录节点 ID 不能重复")
		}
		if node.Type != learning.CurriculumUnit && node.Type != learning.CurriculumChapter && node.Type != learning.CurriculumLesson {
			return nil, errors.New("目录节点类型不正确")
		}
		if nameByParent[node.ParentID] == nil {
			nameByParent[node.ParentID] = map[string]bool{}
		}
		if nameByParent[node.ParentID][node.Name] {
			return nil, errors.New("同一级目录名称不能重复")
		}
		nameByParent[node.ParentID][node.Name] = true
		if node.SortOrder <= 0 {
			node.SortOrder = index + 1
		}
		byID[node.ID] = node
		result = append(result, node)
	}
	unitCount, chapterCount, lessonCount := 0, 0, 0
	for _, node := range result {
		switch node.Type {
		case learning.CurriculumUnit:
			if node.ParentID != "" {
				return nil, errors.New("Unit 不能设置上级目录")
			}
			unitCount++
		case learning.CurriculumChapter:
			parent, ok := byID[node.ParentID]
			if !ok || parent.Type != learning.CurriculumUnit {
				return nil, errors.New("Chapter 必须归属 Unit")
			}
			chapterCount++
		case learning.CurriculumLesson:
			parent, ok := byID[node.ParentID]
			if !ok || parent.Type != learning.CurriculumChapter {
				return nil, errors.New("Lesson 必须归属 Chapter")
			}
			lessonCount++
		}
	}
	if unitCount == 0 || chapterCount == 0 || lessonCount == 0 {
		return nil, errors.New("课程目录必须包含 Unit、Chapter 和 Lesson")
	}
	return result, nil
}

func countCurriculumLessons(nodes []learning.CurriculumNode) int {
	count := 0
	for _, node := range nodes {
		if node.Type == learning.CurriculumLesson {
			count++
		}
	}
	return count
}

func curriculumPathForLesson(course learning.Course, lessonID string) (learning.CurriculumPath, error) {
	lessonID = strings.TrimSpace(lessonID)
	if lessonID == "" {
		return learning.CurriculumPath{}, errors.New("请选择课节")
	}
	byID := make(map[string]learning.CurriculumNode, len(course.Curriculum))
	for _, node := range course.Curriculum {
		byID[node.ID] = node
	}
	lesson, ok := byID[lessonID]
	if !ok || lesson.Type != learning.CurriculumLesson {
		return learning.CurriculumPath{}, errors.New("请选择当前课程下的有效课节")
	}
	chapter, ok := byID[lesson.ParentID]
	if !ok || chapter.Type != learning.CurriculumChapter {
		return learning.CurriculumPath{}, errors.New("课节目录不完整")
	}
	unit, ok := byID[chapter.ParentID]
	if !ok || unit.Type != learning.CurriculumUnit {
		return learning.CurriculumPath{}, errors.New("课节目录不完整")
	}
	return learning.CurriculumPath{Unit: unit.Name, Chapter: chapter.Name, Lesson: lesson.Name}, nil
}

func (s *MemoryStore) courseNameExists(currentID, name string) bool {
	for _, course := range s.courses {
		if course.ID != currentID && course.Name == name {
			return true
		}
	}
	return false
}

func (s *MemoryStore) decorateCourse(course learning.Course) learning.Course {
	materialNum := 0
	for _, material := range s.materials {
		if material.CourseID == course.ID {
			materialNum++
		}
	}
	homeworkNum := 0
	for _, item := range s.homework {
		if item.CourseID == course.ID {
			homeworkNum++
		}
	}
	course.MaterialNum = materialNum
	course.HomeworkNum = homeworkNum
	return course
}

func (s *MemoryStore) syncCourseReferences(course learning.Course) {
	for index := range s.materials {
		if s.materials[index].CourseID == course.ID {
			s.materials[index].Course = course.Name
			s.materials[index].LearningSpaceID = course.LearningSpaceID
		}
	}
	for index := range s.homework {
		if s.homework[index].CourseID == course.ID {
			s.homework[index].Course = course.Name
			s.homework[index].LearningSpaceID = course.LearningSpaceID
		}
	}
}

func (s *MemoryStore) replacePackageRelations(packageID string, learningSpaceIDs []string, contentTypes []string) {
	nextSpaces := make([]packageSpace, 0, len(s.packageSpaces)+len(learningSpaceIDs))
	for _, relation := range s.packageSpaces {
		if relation.PackageID != packageID {
			nextSpaces = append(nextSpaces, relation)
		}
	}
	for _, spaceID := range learningSpaceIDs {
		nextSpaces = append(nextSpaces, packageSpace{PackageID: packageID, LearningSpaceID: spaceID})
	}
	s.packageSpaces = nextSpaces

	nextTypes := make([]packageContentType, 0, len(s.contentTypes)+len(contentTypes))
	for _, item := range s.contentTypes {
		if item.PackageID != packageID {
			nextTypes = append(nextTypes, item)
		}
	}
	for _, code := range contentTypes {
		nextTypes = append(nextTypes, packageContentType{PackageID: packageID, ContentType: code})
	}
	s.contentTypes = nextTypes
}

func (s *MemoryStore) refreshSpaceAccessForPackage(packageID string) {
	activeGrants := make([]packageGrant, 0)
	for _, grant := range s.grants {
		if grant.PackageID == packageID && grantActive(grant) {
			activeGrants = append(activeGrants, grant)
		}
	}
	nextAccess := make([]learningSpaceAccess, 0, len(s.spaceAccess))
	for _, access := range s.spaceAccess {
		remove := false
		for _, grant := range activeGrants {
			if access.PackageGrantID == grant.ID {
				remove = true
				break
			}
		}
		if !remove {
			nextAccess = append(nextAccess, access)
		}
	}
	s.spaceAccess = nextAccess
	for _, grant := range activeGrants {
		s.syncSpaceAccessForGrant(grant)
	}
}

func (s *MemoryStore) decoratePackage(pkg learning.Package) learning.Package {
	if strings.TrimSpace(pkg.Level) == "" {
		pkg.Level = "S"
	}
	pkg.LearningSpaceIDs = s.learningSpaceIDsForPackage(pkg.ID)
	pkg.LearningSpaces = s.learningSpaceNamesForPackage(pkg.ID)
	pkg.ContentTypeCodes = s.contentTypesForPackage(pkg.ID)
	pkg.ContentTypes = s.contentTypeLabelsForPackage(pkg.ID)
	pkg.OpenStudentNum = 0
	for _, grant := range s.grants {
		if grant.PackageID == pkg.ID && grantActive(grant) {
			pkg.OpenStudentNum++
		}
	}
	return pkg
}

func (s *MemoryStore) openContentForPackage(pkg learning.Package) ([]string, []string, []string) {
	courses := make([]string, 0)
	materials := make([]string, 0)
	homework := make([]string, 0)
	spaceIDs := s.learningSpaceIDsForPackage(pkg.ID)
	contentTypes := s.contentTypesForPackage(pkg.ID)
	for _, course := range s.courses {
		if course.Status != learning.StatusEnabled || !containsString(spaceIDs, course.LearningSpaceID) || !containsString(contentTypes, "course") {
			continue
		}
		courses = appendUnique(courses, course.Name)
	}
	for _, material := range s.materials {
		if materialPublished(material.Status) && materialTagIn(material.TagCode, "HD", "Blank") && containsString(spaceIDs, material.LearningSpaceID) && containsString(contentTypes, "handout") {
			materials = appendUnique(materials, material.Title)
		}
	}
	for _, item := range s.homework {
		if homeworkVisible(item.Status) && homeworkTagIn(item.TagCode, "HW", "EXAM", "Exam", "Special") && containsString(spaceIDs, item.LearningSpaceID) && containsString(contentTypes, "question") {
			homework = appendUnique(homework, item.Title)
		}
	}
	return courses, materials, homework
}

func (s *MemoryStore) openContentForStudentGrant(grant packageGrant) ([]string, []string, []string) {
	courses := make([]string, 0)
	materials := make([]string, 0)
	homework := make([]string, 0)
	spaceIDs := s.learningSpaceIDsForGrant(grant.ID)
	contentTypes := s.contentTypesForPackage(grant.PackageID)
	for _, course := range s.courses {
		if course.Status == learning.StatusEnabled && containsString(spaceIDs, course.LearningSpaceID) && containsString(contentTypes, "course") {
			courses = appendUnique(courses, course.Name)
		}
	}
	for _, material := range s.materials {
		if materialPublished(material.Status) && materialTagIn(material.TagCode, "HD", "Blank") && containsString(spaceIDs, material.LearningSpaceID) && containsString(contentTypes, "handout") {
			materials = appendUnique(materials, material.Title)
		}
	}
	for _, item := range s.homework {
		if homeworkVisible(item.Status) && homeworkTagIn(item.TagCode, "HW", "EXAM", "Exam", "Special") && containsString(spaceIDs, item.LearningSpaceID) && containsString(contentTypes, "question") {
			homework = appendUnique(homework, item.Title)
		}
	}
	return courses, materials, homework
}

func (s *MemoryStore) blockedContentForPackage(pkg learning.Package) []string {
	blocked := make([]string, 0)
	spaceIDs := s.learningSpaceIDsForPackage(pkg.ID)
	for _, space := range s.learningSpaces {
		if space.Status == learning.StatusDisabled || containsString(spaceIDs, space.ID) {
			continue
		}
		if space.Grade == pkg.Grade && space.Semester == pkg.Semester {
			blocked = appendUnique(blocked, space.Name)
		}
	}
	return blocked
}

func (s *MemoryStore) audienceForContent(learningSpaceID, contentType string) ([]string, []string) {
	packages := make([]string, 0)
	students := make([]string, 0)
	for _, pkg := range s.packages {
		if !containsString(s.learningSpaceIDsForPackage(pkg.ID), learningSpaceID) || !containsString(s.contentTypesForPackage(pkg.ID), contentType) {
			continue
		}
		packages = appendUnique(packages, pkg.Name)
		for _, grant := range s.grants {
			if grant.PackageID != pkg.ID || !grantActive(grant) || !s.grantOpensSpace(grant.ID, learningSpaceID) {
				continue
			}
			student, ok := s.findStudent(grant.StudentID)
			if ok {
				students = appendUnique(students, student.Name)
			}
		}
	}
	return packages, students
}

func (s *MemoryStore) findGrantIndex(studentID, packageID string) (int, bool) {
	for index, grant := range s.grants {
		if grant.StudentID == studentID && grant.PackageID == packageID && grant.Status != "revoked" {
			return index, true
		}
	}
	return -1, false
}

func (s *MemoryStore) activeGrantState(studentID, packageID string) (bool, string, string) {
	for _, grant := range s.grants {
		if grant.StudentID == studentID && grant.PackageID == packageID && grantActive(grant) {
			return true, grant.StartsAt, grantEndsAt(grant)
		}
	}
	return false, "", ""
}

func (s *MemoryStore) grantState(studentID, packageID string) (bool, string, string) {
	for _, grant := range s.grants {
		if grant.StudentID == studentID && grant.PackageID == packageID && grant.Status != "revoked" {
			return true, grant.StartsAt, grantEndsAt(grant)
		}
	}
	return false, "", ""
}

func (s *MemoryStore) addStudentOpenedPackage(studentID, packageName string) {
	for i := range s.students {
		if s.students[i].ID == studentID {
			s.students[i].OpenedPackages = appendUnique(s.students[i].OpenedPackages, packageName)
			if s.students[i].LearningStatus == "" || s.students[i].LearningStatus == "待开通" {
				s.students[i].LearningStatus = "已开通"
			}
			return
		}
	}
}

func (s *MemoryStore) coursesForStudent(studentID string) []learning.Course {
	out := make([]learning.Course, 0)
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) || len(s.contentTypesForPackage(grant.PackageID)) == 0 {
			continue
		}
		spaceIDs := s.learningSpaceIDsForGrant(grant.ID)
		for _, course := range s.courses {
			if course.Status == learning.StatusEnabled && containsString(spaceIDs, course.LearningSpaceID) {
				out = appendCourseUnique(out, course)
			}
		}
	}
	for _, course := range s.previewCoursesForStudent(studentID) {
		out = appendCourseUnique(out, course)
	}
	materialCounts := make(map[string]int)
	for _, material := range s.materialsForStudent(studentID) {
		materialCounts[material.CourseID]++
	}
	homeworkCounts := make(map[string]int)
	for _, item := range s.homeworkForStudent(studentID) {
		homeworkCounts[item.CourseID]++
	}
	for index := range out {
		out[index].MaterialNum = materialCounts[out[index].ID]
		out[index].HomeworkNum = homeworkCounts[out[index].ID]
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := s.courseAccessForStudent(studentID, out[i])
		right := s.courseAccessForStudent(studentID, out[j])
		if left.AvailableAt != right.AvailableAt {
			return left.AvailableAt > right.AvailableAt
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// previewCoursesForStudent 为未购课学生提供本年级每门学科的首个课节入口。
// 预览是由课程内容实时派生的永久权限，不写入套餐授权：内容发布完整后即可
// 自动出现，也不会与正式套餐的有效期或学习状态混在一起。
func (s *MemoryStore) previewCoursesForStudent(studentID string) []learning.Course {
	student, ok := s.findStudent(studentID)
	if !ok || student.AccountStatus != "正常" {
		return nil
	}
	selected := make(map[string]learning.Course)
	for _, course := range s.courses {
		if course.Status != learning.StatusEnabled || course.Grade != student.Grade {
			continue
		}
		// 首课预览只面向尚未获得该类内容授权的学生。已存在未来或已过期
		// 授权时，不能用永久预览绕开授权生效期或到期日。
		if s.hasCourseGrantForSubject(studentID, course.Grade, course.Subject) {
			continue
		}
		space, exists := s.findLearningSpace(course.LearningSpaceID)
		if !exists || space.Status != learning.StatusEnabled || space.Grade != student.Grade {
			continue
		}
		if _, ready := s.previewLessonForCourse(course); !ready {
			continue
		}
		key := subjectSlug(course.Subject)
		current, exists := selected[key]
		if !exists || s.previewCourseOrder(course) < s.previewCourseOrder(current) {
			selected[key] = course
		}
	}
	out := make([]learning.Course, 0, len(selected))
	for _, course := range selected {
		out = append(out, course)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Subject != out[j].Subject {
			return out[i].Subject < out[j].Subject
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *MemoryStore) previewCourseOrder(course learning.Course) string {
	space, ok := s.findLearningSpace(course.LearningSpaceID)
	if !ok {
		return course.ID
	}
	semester := "9"
	if semesterNumber(space.Semester) == "1" {
		semester = "1"
	} else if semesterNumber(space.Semester) == "2" {
		semester = "2"
	}
	phase := "9"
	if strings.Contains(space.Phase, "Q1") || strings.Contains(space.Phase, "期中") {
		phase = "1"
	} else if strings.Contains(space.Phase, "Q2") || strings.Contains(space.Phase, "期末") {
		phase = "2"
	}
	level := "1"
	if strings.TrimSpace(space.Level) == "S" || strings.TrimSpace(space.Level) == "" {
		level = "0"
	}
	return semester + phase + level + course.ID
}

// previewLessonForCourse 仅在首个 Lesson 同时具备已发布讲义和习题时返回。
func (s *MemoryStore) previewLessonForCourse(course learning.Course) (string, bool) {
	children := map[string][]learning.CurriculumNode{}
	for _, node := range course.Curriculum {
		children[node.ParentID] = append(children[node.ParentID], node)
	}
	for parentID := range children {
		sort.SliceStable(children[parentID], func(i, j int) bool {
			if children[parentID][i].SortOrder != children[parentID][j].SortOrder {
				return children[parentID][i].SortOrder < children[parentID][j].SortOrder
			}
			return children[parentID][i].Name < children[parentID][j].Name
		})
	}
	// 体验必须从第一单元开始，而不是把所有 Lesson 拍平后按编号挑一个；
	// 这样后续即使不同单元里的课节编号重复，也不会跳到第二单元。
	var firstUnit *learning.CurriculumNode
	for index := range children[""] {
		if children[""][index].Type == learning.CurriculumUnit {
			firstUnit = &children[""][index]
			break
		}
	}
	var firstLesson func(string) string
	firstLesson = func(parentID string) string {
		for _, node := range children[parentID] {
			if node.Type == learning.CurriculumLesson {
				return node.ID
			}
			if lessonID := firstLesson(node.ID); lessonID != "" {
				return lessonID
			}
		}
		return ""
	}
	lessonID := ""
	if firstUnit != nil {
		lessonID = firstLesson(firstUnit.ID)
	} else {
		lessonID = firstLesson("") // 兼容旧数据没有 Unit 的扁平目录。
	}
	if lessonID == "" {
		return "", false
	}
	return lessonID, s.previewLessonHasContent(course.ID, lessonID)
}

func (s *MemoryStore) previewLessonHasContent(courseID, lessonID string) bool {
	hasMaterial, hasHomework := false, false
	for _, material := range s.materials {
		if material.CourseID == courseID && material.LessonID == lessonID && materialPublished(material.Status) {
			hasMaterial = true
			break
		}
	}
	for _, homework := range s.homework {
		if homework.CourseID == courseID && homework.LessonID == lessonID && homeworkVisible(homework.Status) {
			hasHomework = true
			break
		}
	}
	return hasMaterial && hasHomework
}

// hasContentGrantForLearningSpace 判断学生是否已有过该学习空间、该内容类型的正式授权。
// 不以当前是否生效为条件：未来授权和已过期授权都应阻止首课预览绕开其授权周期。
func (s *MemoryStore) hasContentGrantForLearningSpace(studentID, learningSpaceID, contentType string) bool {
	for _, grant := range s.grants {
		if grant.StudentID != studentID || grant.Status == "revoked" {
			continue
		}
		if containsString(s.contentTypesForPackage(grant.PackageID), contentType) && containsString(s.learningSpaceIDsForPackage(grant.PackageID), learningSpaceID) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) hasAnyContentGrantForLearningSpace(studentID, learningSpaceID string) bool {
	for _, grant := range s.grants {
		if grant.StudentID == studentID && grant.Status != "revoked" && containsString(s.learningSpaceIDsForPackage(grant.PackageID), learningSpaceID) {
			return true
		}
	}
	return false
}

// hasCourseGrantForSubject 按学生已经购买或获授的课程包判断。套餐覆盖的是一门
// 学科的学习安排，而不是单个课节；因此未来/到期套餐都不应再显示同学科的永久预览。
func (s *MemoryStore) hasCourseGrantForSubject(studentID, grade, subject string) bool {
	for _, grant := range s.grants {
		if grant.StudentID != studentID || grant.Status == "revoked" || !containsString(s.contentTypesForPackage(grant.PackageID), "course") {
			continue
		}
		pkg, ok := s.findPackage(grant.PackageID)
		if ok && pkg.Grade == grade && subjectsMatch(pkg.Subject, subject) {
			return true
		}
	}
	return false
}

type courseAccess struct {
	OpenedAt       string
	AvailableAt    string
	HighlightUntil string
	IsNew          bool
}

func (s *MemoryStore) courseAccessForStudent(studentID string, course learning.Course) courseAccess {
	selected := packageGrant{}
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) || len(s.contentTypesForPackage(grant.PackageID)) == 0 || !containsString(s.learningSpaceIDsForGrant(grant.ID), course.LearningSpaceID) {
			continue
		}
		if selected.ID == "" || grant.StartsAt > selected.StartsAt || (grant.StartsAt == selected.StartsAt && grantOpenedAt(grant) > grantOpenedAt(selected)) {
			selected = grant
		}
	}
	if selected.ID == "" {
		return courseAccess{}
	}
	availableAt := selected.StartsAt
	if availableAt == "" {
		availableAt = selected.OpenedAt
	}
	visibleAt, _, err := normalizeGrantTimestamp(availableAt, false)
	if err != nil {
		return courseAccess{OpenedAt: grantOpenedAt(selected), AvailableAt: availableAt}
	}
	openedAtValue := grantOpenedAt(selected)
	if openedAtValue != "" {
		openedAt, _, openedErr := normalizeGrantTimestamp(openedAtValue, false)
		if openedErr == nil && openedAt.After(visibleAt) {
			visibleAt = openedAt
			availableAt = openedAtValue
		}
	}
	highlightUntil := visibleAt.Add(time.Hour)
	return courseAccess{
		OpenedAt:       openedAtValue,
		AvailableAt:    availableAt,
		HighlightUntil: highlightUntil.Format("2006-01-02 15:04:05"),
		IsNew:          !time.Now().Before(visibleAt) && time.Now().Before(highlightUntil),
	}
}

// studentAccessibleSpaceIDs 返回学生通过有效套餐开通的全部学习空间 ID，不区分内容类型。
func (s *MemoryStore) studentAccessibleSpaceIDs(studentID string) []string {
	out := make([]string, 0)
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) {
			continue
		}
		out = appendUnique(out, s.learningSpaceIDsForGrant(grant.ID)...)
	}
	return out
}

// studentHasSubjectGrade 判断学生是否开通了某学科+年级，用于「只有同年级同学科才能排一起」。
func (s *MemoryStore) studentHasSubjectGrade(studentID, subject, grade string) bool {
	return s.studentHasSubjectGradeLevel(studentID, subject, grade, "")
}

// studentHasSubjectGradeLevel 把课程等级纳入分班资格。level 为空时保留旧调用的
// “任意等级均可”语义；排课入口始终传入课程所属等级。
func (s *MemoryStore) studentHasSubjectGradeLevel(studentID, subject, grade, level string) bool {
	for _, id := range s.studentAccessibleSpaceIDs(studentID) {
		for _, space := range s.learningSpaces {
			spaceLevel := strings.TrimSpace(space.Level)
			if spaceLevel == "" {
				spaceLevel = "S"
			}
			if space.ID == id && space.Grade == grade && subjectsMatch(space.Subject, subject) && (level == "" || spaceLevel == level) {
				return true
			}
		}
	}
	return false
}

func (s *MemoryStore) learningSpaceLevel(id string) string {
	if space, ok := s.findLearningSpace(id); ok && strings.TrimSpace(space.Level) != "" {
		return strings.TrimSpace(space.Level)
	}
	return "S"
}

func (s *MemoryStore) materialsForStudent(studentID string) []learning.Material {
	out := make([]learning.Material, 0)
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) || !containsString(s.contentTypesForPackage(grant.PackageID), "handout") {
			continue
		}
		spaceIDs := s.learningSpaceIDsForGrant(grant.ID)
		for _, material := range s.materials {
			if firstLesson, limited := s.trialFirstLessonForGrant(grant, material.CourseID); limited && material.LessonID != firstLesson {
				continue
			}
			if materialPublished(material.Status) && materialTagIn(material.TagCode, "HD", "Blank") && containsString(spaceIDs, material.LearningSpaceID) {
				out = appendMaterialUnique(out, s.decorateMaterial(material))
			}
		}
	}
	for _, course := range s.previewCoursesForStudent(studentID) {
		if s.hasAnyContentGrantForLearningSpace(studentID, course.LearningSpaceID) {
			continue
		}
		lessonID, ready := s.previewLessonForCourse(course)
		if !ready {
			continue
		}
		for _, material := range s.materials {
			if material.CourseID == course.ID && material.LessonID == lessonID && materialPublished(material.Status) {
				out = appendMaterialUnique(out, s.decorateMaterial(material))
			}
		}
	}
	return out
}

func (s *MemoryStore) studentMaterialsForPrincipal(principal learning.Principal) []learning.Material {
	materials := orderMaterialsByCourse(s.materialsForStudent(principal.StudentID))
	for index := range materials {
		materials[index] = s.decorateStudentMaterial(principal, materials[index])
	}
	return materials
}

func (s *MemoryStore) homeworkForStudent(studentID string) []learning.Homework {
	out := make([]learning.Homework, 0)
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) || !containsString(s.contentTypesForPackage(grant.PackageID), "question") {
			continue
		}
		spaceIDs := s.learningSpaceIDsForGrant(grant.ID)
		for _, item := range s.homework {
			if firstLesson, limited := s.trialFirstLessonForGrant(grant, item.CourseID); limited && item.LessonID != firstLesson {
				continue
			}
			if homeworkVisible(item.Status) && homeworkTagIn(item.TagCode, "HW", "EXAM", "Exam", "Special") && containsString(spaceIDs, item.LearningSpaceID) {
				out = appendHomeworkUnique(out, item)
			}
		}
	}
	for _, course := range s.previewCoursesForStudent(studentID) {
		if s.hasAnyContentGrantForLearningSpace(studentID, course.LearningSpaceID) {
			continue
		}
		lessonID, ready := s.previewLessonForCourse(course)
		if !ready {
			continue
		}
		for _, item := range s.homework {
			if item.CourseID == course.ID && item.LessonID == lessonID && homeworkVisible(item.Status) {
				out = appendHomeworkUnique(out, item)
			}
		}
	}
	return out
}

func (s *MemoryStore) studentHomeworkForPrincipal(principal learning.Principal) []learning.Homework {
	homework := s.homeworkForStudent(principal.StudentID)
	for index := range homework {
		homework[index] = s.decorateStudentHomework(principal, homework[index])
	}
	return homework
}

func (s *MemoryStore) learningSpaceIDsForPackage(packageID string) []string {
	out := make([]string, 0)
	for _, relation := range s.packageSpaces {
		if relation.PackageID == packageID && s.learningSpaceEnabled(relation.LearningSpaceID) {
			out = appendUnique(out, relation.LearningSpaceID)
		}
	}
	return out
}

func (s *MemoryStore) learningSpaceIDsForGrant(grantID string) []string {
	out := make([]string, 0)
	for _, access := range s.spaceAccess {
		if access.PackageGrantID == grantID && access.Status == "active" && grantPeriodActive(access.StartsAt, access.EndsAt) && s.learningSpaceEnabled(access.LearningSpaceID) {
			out = appendUnique(out, access.LearningSpaceID)
		}
	}
	return out
}

func (s *MemoryStore) learningSpaceNamesForPackage(packageID string) []string {
	names := make([]string, 0)
	for _, id := range s.learningSpaceIDsForPackage(packageID) {
		names = appendUnique(names, s.learningSpaceName(id))
	}
	return names
}

func (s *MemoryStore) learningSpaceNamesForGrant(grantID string) []string {
	names := make([]string, 0)
	for _, id := range s.learningSpaceIDsForGrant(grantID) {
		names = appendUnique(names, s.learningSpaceName(id))
	}
	return names
}

func (s *MemoryStore) learningSpaceNames(ids []string) []string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = appendUnique(names, s.learningSpaceName(id))
	}
	return names
}

func (s *MemoryStore) learningSpaceGrades(ids []string) []string {
	grades := make([]string, 0)
	for _, id := range ids {
		for _, space := range s.learningSpaces {
			if space.ID == id {
				grades = appendUnique(grades, space.Grade)
				break
			}
		}
	}
	return grades
}

func (s *MemoryStore) learningSpaceSubjects(ids []string) []string {
	subjects := make([]string, 0)
	for _, id := range ids {
		for _, space := range s.learningSpaces {
			if space.ID == id {
				subjects = appendUnique(subjects, space.Subject)
				break
			}
		}
	}
	return subjects
}

func (s *MemoryStore) learningSpaceName(id string) string {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return space.Name
		}
	}
	return id
}

func (s *MemoryStore) decorateMaterial(material learning.Material) learning.Material {
	material.Type = "课程讲义"
	material.TagCode = contentTagCodeOrInferred(material.TagCode, material.Title, material.FileName)
	if space, ok := s.findLearningSpace(material.LearningSpaceID); ok {
		material.Grade = space.Grade
		material.Semester = space.Semester
		material.Subject = space.Subject
	}
	if asset, ok := s.fileAssets[material.FileID]; ok {
		material.PreviewStatus = asset.PreviewStatus
		material.PreviewError = asset.PreviewError
	}
	return material
}

func (s *MemoryStore) decorateStudentMaterial(principal learning.Principal, material learning.Material) learning.Material {
	material = s.decorateMaterial(material)
	material.WatermarkText = s.studentWatermarkText(principal)
	material.SecurityNotice = studentSecurityNotice()
	if material.FileID != "" {
		material.PreviewURL = "/api/student/materials/" + material.ID + "/preview"
		if s.studentMaterialDownloadEnabled() && s.studentHasActiveContentGrantForLearningSpace(principal.StudentID, material.LearningSpaceID, "download") {
			material.DownloadURL = "/api/student/materials/" + material.ID + "/download"
		}
	}
	return material
}

// studentHasActiveContentGrantForLearningSpace 判断学生当前有效授权是否包含指定内容类型。
// 课程目录可见性与内容下载权限分离：download 仅作为安全下载许可，handout 仍控制讲义是否可见。
func (s *MemoryStore) studentHasActiveContentGrantForLearningSpace(studentID, learningSpaceID, contentType string) bool {
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) || !containsString(s.contentTypesForPackage(grant.PackageID), contentType) {
			continue
		}
		if containsString(s.learningSpaceIDsForGrant(grant.ID), learningSpaceID) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) studentMaterialDownloadEnabled() bool {
	return strings.TrimSpace(s.settings["downloadPolicy"]) == "允许下载带水印PDF"
}

func (s *MemoryStore) decorateStudentHomework(principal learning.Principal, homework learning.Homework) learning.Homework {
	if homework.AssessmentType == "" {
		homework.AssessmentType = "practice"
	}
	if homework.DeadlineAt != "" {
		if deadline, err := time.Parse(time.RFC3339, homework.DeadlineAt); err == nil {
			homework.IsOverdue = !time.Now().Before(deadline)
		}
	}
	if asset, ok := s.fileAssets[homework.FileID]; ok {
		homework.PreviewStatus = asset.PreviewStatus
		homework.PreviewError = asset.PreviewError
	}
	homework.WatermarkText = s.studentWatermarkText(principal)
	homework.SecurityNotice = studentSecurityNotice()
	homework.DownloadURL = ""
	return homework
}

func (s *MemoryStore) studentWatermarkText(principal learning.Principal) string {
	name := strings.TrimSpace(principal.Name)
	phone := strings.TrimSpace(principal.Phone)
	studentID := strings.TrimSpace(principal.StudentID)
	if student, ok := s.findStudent(principal.StudentID); ok {
		if strings.TrimSpace(student.Nickname) != "" {
			name = strings.TrimSpace(student.Nickname)
		} else if strings.TrimSpace(student.Name) != "" {
			name = strings.TrimSpace(student.Name)
		}
		if strings.TrimSpace(student.Phone) != "" {
			phone = strings.TrimSpace(student.Phone)
		}
	}
	if name == "" {
		name = "Starline学员"
	}
	parts := []string{name}
	if tail := phoneTail(phone); tail != "" {
		parts = append(parts, tail)
	}
	parts = append(parts, time.Now().Format("2006-01-02 15:04"))
	if suffix := idSuffix(studentID); suffix != "" {
		parts = append(parts, suffix)
	}
	return strings.Join(parts, " · ")
}

func (s *MemoryStore) studentWatermarkStampText(principal learning.Principal, materialID string, generatedAt time.Time) (string, string) {
	studentRef := idSuffix(strings.TrimSpace(principal.StudentID))
	if studentRef == "" {
		studentRef = "ANON"
	}
	phoneRef := strings.TrimPrefix(phoneTail(principal.Phone), "尾号")
	if phoneRef == "" {
		phoneRef = "NONE"
	}
	digest := sha256.Sum256([]byte(principal.StudentID + "|" + materialID + "|" + generatedAt.Format(time.RFC3339Nano)))
	traceCode := fmt.Sprintf("%X", digest[:])[:10]
	stamp := fmt.Sprintf("STARLINE | U-%s | P-%s | %s | T-%s", studentRef, phoneRef, generatedAt.Format("2006-01-02 15:04"), traceCode)
	return stamp, traceCode
}

func studentSecurityNotice() string {
	return "这份资料仅供你本人学习，已添加专属水印。请不要分享、截图或录屏。"
}

func phoneTail(phone string) string {
	digits := make([]rune, 0, 4)
	for _, value := range phone {
		if value >= '0' && value <= '9' {
			digits = append(digits, value)
		}
	}
	if len(digits) == 0 {
		return ""
	}
	if len(digits) > 4 {
		digits = digits[len(digits)-4:]
	}
	return "尾号" + string(digits)
}

func idSuffix(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	runes := []rune(id)
	if len(runes) > 6 {
		runes = runes[len(runes)-6:]
	}
	return "ID" + string(runes)
}

func studentAuditOperator(principal learning.Principal) string {
	return middlewareAuditLabel(principal.Name, principal.UserID)
}

func studentSecurityTarget(principal learning.Principal, req learning.SecurityEventRequest) string {
	target := strings.TrimSpace(req.TargetType)
	if req.TargetID != "" {
		target += ":" + req.TargetID
	}
	if target == "" || target == ":" {
		target = principal.Name
	}
	return target
}

func middlewareAuditLabel(name, id string) string {
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}{Name: strings.TrimSpace(name), ID: strings.TrimSpace(id)})
	if err != nil {
		return strings.TrimSpace(name)
	}
	return "audit:" + base64.RawURLEncoding.EncodeToString(payload)
}

func (s *MemoryStore) learningSpaceEnabled(id string) bool {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return space.Status == learning.StatusEnabled
		}
	}
	return false
}

func (s *MemoryStore) learningSpaceExists(id string) bool {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return true
		}
	}
	return false
}
