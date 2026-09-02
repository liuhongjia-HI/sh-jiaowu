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

func isTestStudent(student learning.Student) bool {
	text := strings.ToLower(strings.TrimSpace(student.Name + " " + student.Remark + " " + student.RegistrationSource))
	return strings.HasPrefix(student.ID, "test-") || strings.Contains(text, "测试") || strings.Contains(text, "test") || student.ID == "stu-001" || student.ID == "stu-002" || student.ID == "stu-003"
}

func filterStudentRows[T any](items []T, remove func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if !remove(item) {
			out = append(out, item)
		}
	}
	return out
}

// cleanupStudentReferences removes rows in the in-memory aggregate whose owning
// record is the student being cleaned. The persistence delta then deletes the
// corresponding MySQL rows in the same transaction.
func (s *MemoryStore) cleanupStudentReferences(studentID string) {
	s.users = filterStudentRows(s.users, func(v learning.User) bool { return v.StudentID == studentID })
	s.guardianStudents = filterStudentRows(s.guardianStudents, func(v learning.GuardianStudent) bool { return v.StudentID == studentID })
	s.grants = filterStudentRows(s.grants, func(v packageGrant) bool { return v.StudentID == studentID })
	s.trials = filterStudentRows(s.trials, func(v studentTrialRecord) bool { return v.StudentID == studentID })
	s.availability = filterStudentRows(s.availability, func(v learning.AvailabilitySlot) bool { return v.OwnerID == studentID })
	s.tutoringAssignments = filterStudentRows(s.tutoringAssignments, func(v learning.TutoringAssignment) bool { return v.StudentID == studentID })
	s.lessonFeedbacks = filterStudentRows(s.lessonFeedbacks, func(v learning.LessonFeedback) bool { return v.StudentID == studentID })
	s.commercialOrders = filterStudentRows(s.commercialOrders, func(v learning.CommercialOrder) bool { return v.StudentID == studentID })
	s.lessonConsumptions = filterStudentRows(s.lessonConsumptions, func(v learning.LessonConsumption) bool { return v.StudentID == studentID })
	s.renewalReminders = filterStudentRows(s.renewalReminders, func(v learning.RenewalReminder) bool { return v.StudentID == studentID })
	s.parentNotices = filterStudentRows(s.parentNotices, func(v learning.ParentNotice) bool { return v.StudentID == studentID })
	s.scoreRecords = filterStudentRows(s.scoreRecords, func(v learning.StudentScoreRecord) bool { return v.StudentID == studentID })
	s.classReservations = filterStudentRows(s.classReservations, func(v learning.ClassReservationIntent) bool { return v.StudentID == studentID })
	s.notices = filterStudentRows(s.notices, func(v learning.Notice) bool { return v.RelatedType == "student" && v.RelatedID == studentID })
	delete(s.subscriptionPreferences, studentID)
	for key, submission := range s.submissions {
		if submission.StudentID == studentID {
			delete(s.submissions, key)
		}
	}
	for key, favorite := range s.favorites {
		if favorite.StudentID == studentID {
			delete(s.favorites, key)
		}
	}
	for index := range s.scheduleClasses {
		students := s.scheduleClasses[index].Students[:0]
		for _, candidate := range s.scheduleClasses[index].Students {
			if candidate.ID != studentID {
				students = append(students, candidate)
			}
		}
		s.scheduleClasses[index].Students = students
	}
}

func (s *MemoryStore) cleanupTestStudentsUnlocked(operator string, principal learning.Principal) (learning.StudentCleanupResult, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.StudentCleanupResult, error) {
			return work.cleanupTestStudentsUnlocked(operator, principal)
		})
	}
	_ = principal
	deleted := make([]string, 0)
	kept := make([]learning.Student, 0, len(s.students))
	for _, student := range s.students {
		if !isTestStudent(student) {
			kept = append(kept, student)
			continue
		}
		deleted = append(deleted, student.ID)
		s.cleanupStudentReferences(student.ID)
	}
	s.students = kept
	for _, id := range deleted {
		s.prependLog(operator, "清理测试学生", id)
	}
	return learning.StudentCleanupResult{DeletedCount: len(deleted), DeletedIDs: deleted}, nil
}

func materialTagIn(tag string, allowed ...string) bool {
	if strings.TrimSpace(tag) == "" {
		return true
	}
	for _, value := range allowed {
		if strings.EqualFold(strings.TrimSpace(tag), value) {
			return true
		}
	}
	return false
}

func homeworkTagIn(tag string, allowed ...string) bool { return materialTagIn(tag, allowed...) }

func (s *MemoryStore) studentsUnlocked(principal learning.Principal, query learning.StudentQuery) []learning.Student {
	students := make([]learning.Student, 0, len(s.students))
	for _, student := range s.students {
		decorated := s.decorateStudent(student)
		if s.canSeeStudent(principal, decorated, s.coursesForStudent(student.ID)) && matchesStudentQuery(decorated, query) {
			students = append(students, decorated)
		}
	}
	sort.SliceStable(students, func(i, j int) bool {
		return students[i].CreatedAt > students[j].CreatedAt
	})
	return students
}

func (s *MemoryStore) studentDetailUnlocked(principal learning.Principal, id string) (learning.StudentDetail, error) {
	student, err := s.visibleStudent(principal, id)
	if err != nil {
		return learning.StudentDetail{}, err
	}
	grants, _ := s.studentGrantsUnlocked(principal, id)
	records, _ := s.studentLearningRecordsUnlocked(principal, id)
	permissions := s.permissionForStudent(student)
	return learning.StudentDetail{
		Student:         student,
		Grants:          grants,
		OpeningMatrix:   s.openingMatrixForStudent(student),
		Permissions:     permissions,
		LearningRecords: records,
		Notices:         s.noticesForStudent(student),
		Logs:            s.logsForStudent(student),
	}, nil
}

func (s *MemoryStore) openingMatrixForStudent(student learning.Student) []learning.StudentOpeningScope {
	result := make([]learning.StudentOpeningScope, 0)
	for _, space := range s.learningSpaces {
		if space.Status != learning.StatusEnabled || space.Grade != student.Grade {
			continue
		}
		content := []learning.StudentOpeningCell{
			{ContentTypeCode: "course", Items: s.openingItemsForSpace(space.ID, "course")},
			{ContentTypeCode: "handout", Items: s.openingItemsForSpace(space.ID, "handout")},
			{ContentTypeCode: "question", Items: s.openingItemsForSpace(space.ID, "question")},
			{ContentTypeCode: "download", Items: s.openingItemsForSpace(space.ID, "download")},
		}
		for _, grant := range s.grants {
			if grant.StudentID != student.ID || !grantActive(grant) {
				continue
			}
			pkg, exists := s.findPackage(grant.PackageID)
			if !exists {
				continue
			}
			for index := range content {
				cell := &content[index]
				if !s.packageOpensContent(pkg, space.ID, cell.ContentTypeCode) {
					continue
				}
				cell.Opened = true
				if isDirectGrantPackage(pkg.ID) {
					cell.DirectOpened = true
				} else {
					cell.PackageOpened = true
					cell.PackageNames = appendUnique(cell.PackageNames, pkg.Name)
				}
			}
		}
		result = append(result, learning.StudentOpeningScope{
			LearningSpaceID: space.ID,
			Name:            space.Name,
			Subject:         space.Subject,
			Content:         content,
		})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (s *MemoryStore) openingItemsForSpace(learningSpaceID, contentTypeCode string) []learning.StudentOpeningItem {
	items := make([]learning.StudentOpeningItem, 0)
	switch contentTypeCode {
	case "course":
		for _, course := range s.courses {
			if course.LearningSpaceID == learningSpaceID && course.Status == learning.StatusEnabled {
				items = append(items, learning.StudentOpeningItem{ID: course.ID, Title: course.Name})
			}
		}
	case "handout":
		for _, material := range s.materials {
			if material.LearningSpaceID == learningSpaceID && materialPublished(material.Status) && materialTagIn(material.TagCode, "HD", "Blank") {
				items = append(items, learning.StudentOpeningItem{ID: material.ID, Title: material.Title})
			}
		}
	case "download":
		for _, material := range s.materials {
			if material.LearningSpaceID == learningSpaceID && materialPublished(material.Status) && materialTagIn(material.TagCode, "HD", "Blank") {
				items = append(items, learning.StudentOpeningItem{ID: material.ID, Title: material.Title})
			}
		}
	case "question":
		for _, homework := range s.homework {
			if homework.LearningSpaceID == learningSpaceID && homeworkVisible(homework.Status) && homeworkTagIn(homework.TagCode, "HW", "EXAM", "Exam", "Special") {
				items = append(items, learning.StudentOpeningItem{ID: homework.ID, Title: homework.Title})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	return items
}

func (s *MemoryStore) createStudentUnlocked(operator string, principal learning.Principal, req learning.StudentUpsertRequest) (learning.Student, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Student, error) {
			return work.createStudentUnlocked(operator, principal, req)
		})
	}
	req, err := normalizeStudentRequest(req, false)
	if err != nil {
		return learning.Student{}, err
	}
	if s.studentAdminPhoneConflicts(req.Phone) {
		return learning.Student{}, errors.New("手机号已存在")
	}
	id := "stu-" + time.Now().Format("20060102150405.000000000")
	student := learning.Student{
		ID:        id,
		Name:      req.Name,
		Nickname:  "",
		AvatarURL: "",
		// 年级不直接存 req.Grade：只记录入学基准，当前年级由 decorateStudent
		// 按学年滚动推导，7 月 1 日自动升一级。
		EnrollmentAcademicYear: s.configuredAcademicYear(),
		EnrollmentGrade:        req.Grade,
		Phone:                  req.Phone,
		SchoolName:             req.SchoolName,
		GuardianName:           req.GuardianName,
		OfficialAccountOpenID:  req.OfficialAccountOpenID,
		OpenedPackages:         []string{},
		LearningStatus:         "未开始",
		AccountStatus:          "正常",
		RegistrationSource:     "后台",
		Remark:                 req.Remark,
		BindStatus:             "待绑定",
		CreatedAt:              time.Now().Format("2006-01-02 15:04:05"),
		// 学生建档不等同于开通套餐，不预设期中/期末等时间段的有效期。
		// 实际权限有效期在开通套餐时按当前学年校历自动计算。
		EffectiveUntil: "",
	}
	s.students = append([]learning.Student{student}, s.students...)
	s.users = append(s.users, learning.User{
		ID:            "user-" + id,
		Name:          req.Name,
		Phone:         req.Phone,
		AccountStatus: "正常",
		Roles:         []learning.Role{learning.RoleStudent},
		StudentID:     id,
		CampusID:      principal.CampusID,
	})
	s.prependLog(operator, "新增学生", student.Name)
	return s.decorateStudent(student), nil
}

func (s *MemoryStore) updateStudentUnlocked(operator string, principal learning.Principal, id string, req learning.StudentUpsertRequest) (learning.Student, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Student, error) {
			return work.updateStudentUnlocked(operator, principal, id, req)
		})
	}
	req, err := normalizeStudentRequest(req, true)
	if err != nil {
		return learning.Student{}, err
	}
	for i := range s.students {
		if s.students[i].ID != id {
			continue
		}
		if _, err := s.visibleStudent(principal, id); err != nil {
			return learning.Student{}, err
		}
		if s.studentAdminPhoneConflicts(req.Phone) {
			return learning.Student{}, errors.New("手机号已存在")
		}
		before := s.decorateStudent(s.students[i])
		wasPendingApproval := s.students[i].AccountStatus == "待审核"
		s.students[i].Name = req.Name
		s.students[i].Phone = req.Phone
		if req.Grade != before.Grade {
			// 管理端改年级是一次人工订正：以“现在”为新的入学基准重新起算，
			// 之后的学年滚动从这个订正点继续走，而不是覆盖一个孤立的静态值。
			s.students[i].EnrollmentAcademicYear = s.configuredAcademicYear()
			s.students[i].EnrollmentGrade = req.Grade
		}
		s.students[i].SchoolName = req.SchoolName
		s.students[i].GuardianName = req.GuardianName
		// openid 只能靠学生关注公众号后回传获得，管理端编辑表单不再采集这一项，
		// 空值意味着"这次提交没带这个字段"，不能拿来覆盖已经绑定上的 openid。
		if req.OfficialAccountOpenID != "" {
			s.students[i].OfficialAccountOpenID = req.OfficialAccountOpenID
		}
		s.students[i].AccountStatus = req.AccountStatus
		s.students[i].Remark = req.Remark
		s.syncStudentUser(s.students[i])
		after := s.decorateStudent(s.students[i])
		action := "更新学生"
		if wasPendingApproval && s.students[i].AccountStatus == "正常" {
			s.activatePendingGuardianRelations(s.students[i].ID)
			action = "审核通过学生申请"
		}
		s.prependLogDetail(operator, action, s.students[i].Name, auditChangeDetail(studentAuditSnapshot(before), studentAuditSnapshot(after)))
		return after, nil
	}
	return learning.Student{}, errors.New("student not found")
}

func (s *MemoryStore) activatePendingGuardianRelations(studentID string) {
	for i := range s.guardianStudents {
		if s.guardianStudents[i].StudentID == studentID && s.guardianStudents[i].Status == learning.GuardianStudentPending {
			s.guardianStudents[i].Status = learning.GuardianStudentActive
		}
	}
}

func normalizeStudentProfileUpdateRequest(req learning.StudentProfileUpdateRequest) (learning.StudentProfileUpdateRequest, bool, error) {
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.AvatarURL = strings.TrimSpace(req.AvatarURL)
	req.StudentName = strings.TrimSpace(req.StudentName)
	req.Grade = strings.TrimSpace(req.Grade)
	req.SchoolName = strings.TrimSpace(req.SchoolName)
	req.GuardianName = strings.TrimSpace(req.GuardianName)
	req.PhoneCode = strings.TrimSpace(req.PhoneCode)
	if len([]rune(req.Nickname)) > 32 {
		return req, false, errors.New("昵称最多 32 个字")
	}
	if len(req.AvatarURL) > 1000 {
		return req, false, errors.New("头像地址过长")
	}
	if len([]rune(req.StudentName)) > 32 {
		return req, false, errors.New("学生姓名最多 32 个字")
	}
	if len([]rune(req.Grade)) > 32 {
		return req, false, errors.New("年级最多 32 个字")
	}
	if len([]rune(req.SchoolName)) > 64 {
		return req, false, errors.New("学校名称最多 64 个字")
	}
	if len([]rune(req.GuardianName)) > 32 {
		return req, false, errors.New("家长称呼最多 32 个字")
	}
	if len([]rune(req.PhoneCode)) > 256 {
		return req, false, errors.New("手机号授权已失效，请重新授权")
	}
	// 头像、昵称、手机号是独立的轻量资料更新，不应因为历史学生档案尚未补齐学校而无法使用。
	// 姓名、年级、学校、家长称呼仍按基础资料流程校验学校信息。
	requiresBasicProfile := req.StudentName != "" || req.Grade != "" || req.SchoolName != "" || req.GuardianName != ""
	return req, requiresBasicProfile, nil
}

func (s *MemoryStore) updateStudentProfileResolvedUnlocked(operator string, principal learning.Principal, req learning.StudentProfileUpdateRequest, requiresBasicProfile bool, resolvedPhone string) (learning.Student, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Student, error) {
			return work.updateStudentProfileResolvedUnlocked(operator, principal, req, requiresBasicProfile, resolvedPhone)
		})
	}
	for i := range s.students {
		if s.students[i].ID != principal.StudentID {
			continue
		}
		if s.students[i].AccountStatus == "停用" {
			return learning.Student{}, errors.New("账号已停用，请联系老师或管理员")
		}
		if resolvedPhone != "" && s.phoneExists(principal.StudentID, resolvedPhone) {
			return learning.Student{}, errors.New("手机号已绑定其他学生，请联系老师处理")
		}
		if requiresBasicProfile && strings.TrimSpace(s.students[i].SchoolName) == "" && req.SchoolName == "" {
			return learning.Student{}, errors.New("请填写学校")
		}
		before := s.decorateStudent(s.students[i])
		if req.Nickname != "" {
			s.students[i].Nickname = req.Nickname
		}
		if req.AvatarURL != "" {
			s.students[i].AvatarURL = req.AvatarURL
		}
		if req.StudentName != "" {
			s.students[i].Name = req.StudentName
		}
		// 年级不接受学生自助修改：它是按入学基准推导出来的，改错了会连累开通校验、
		// 排课候选这些依赖真实年级的逻辑。有异议走管理端订正入学年级。
		if req.SchoolName != "" {
			s.students[i].SchoolName = req.SchoolName
		}
		if resolvedPhone != "" {
			s.students[i].Phone = resolvedPhone
		}
		if requiresBasicProfile {
			s.students[i].GuardianName = req.GuardianName
		}
		s.syncStudentUser(s.students[i])
		after := s.decorateStudent(s.students[i])
		if detail := auditChangeDetail(studentAuditSnapshot(before), studentAuditSnapshot(after)); detail != "" {
			s.prependLogDetail(operator, "更新学生资料", s.students[i].Name, detail)
		}
		return after, nil
	}
	return learning.Student{}, errors.New("student not found")
}

func (s *MemoryStore) remindStudentUnlocked(operator string, principal learning.Principal, id string) (learning.StudentRemindResult, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.StudentRemindResult, error) {
			return work.remindStudentUnlocked(operator, principal, id)
		})
	}
	student, err := s.visibleStudent(principal, id)
	if err != nil {
		return learning.StudentRemindResult{}, err
	}
	noticeID := "notice-" + time.Now().Format("20060102150405")
	notice := learning.Notice{
		ID:              noticeID,
		Type:            "提醒",
		Title:           "学习提醒",
		Target:          student.Name,
		Summary:         "今天的小挑战别忘啦",
		Channel:         "公众号模板消息",
		RecipientOpenID: student.OfficialAccountOpenID,
		RelatedType:     "student",
		RelatedID:       student.ID,
	}
	notice = s.deliverNotice(notice)
	s.prependNoticeRecord(notice)
	s.prependLog(operator, "提醒学生", student.Name)
	if notice.Status == "已发送" {
		return learning.StudentRemindResult{NoticeID: noticeID, Message: "已发送学习提醒"}, nil
	}
	return learning.StudentRemindResult{NoticeID: noticeID, Message: "已创建学习提醒，待完成通知配置后补发"}, nil
}

func (s *MemoryStore) importStudentsUnlocked(operator string, principal learning.Principal, rows []learning.StudentUpsertRequest) (learning.StudentImportResult, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.StudentImportResult, error) {
			return work.importStudentsUnlocked(operator, principal, rows)
		})
	}
	result := learning.StudentImportResult{Errors: []learning.StudentImportRowError{}}
	for index, row := range rows {
		if _, err := s.createStudentUnlocked(operator, principal, row); err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, learning.StudentImportRowError{Row: index + 1, Message: err.Error()})
			continue
		}
		result.SuccessCount++
	}
	s.prependLog(operator, "批量导入学生", "成功 "+itoa(result.SuccessCount)+" 条")
	return result, nil
}

func (s *MemoryStore) studentGrantsUnlocked(principal learning.Principal, id string) ([]learning.StudentGrant, error) {
	if _, err := s.visibleStudent(principal, id); err != nil {
		return nil, err
	}
	grants := make([]learning.StudentGrant, 0)
	for _, grant := range s.grants {
		if grant.StudentID != id || grant.Status == "revoked" {
			continue
		}
		pkg, ok := s.findPackage(grant.PackageID)
		if !ok {
			continue
		}
		openCourses, openMaterials, openHomework := s.openContentForPackage(pkg)
		grants = append(grants, learning.StudentGrant{
			StudentID: id, PackageID: pkg.ID, PackageName: pkg.Name, StartsAt: grant.StartsAt,
			EffectiveUntil: grantEndsAt(grant), PermissionState: grantPermissionState(grant),
			IsDirect: isDirectGrantPackage(pkg.ID), LearningSpaceIDs: s.learningSpaceIDsForPackage(pkg.ID), LearningSpaces: s.learningSpaceNamesForPackage(pkg.ID),
			ContentTypes: s.contentTypeLabelsForPackage(pkg.ID), OpenCourses: openCourses, OpenMaterials: openMaterials, OpenHomework: openHomework,
		})
	}
	return grants, nil
}

func (s *MemoryStore) studentLearningRecordsUnlocked(principal learning.Principal, id string) ([]learning.StudentLearningRecord, error) {
	student, err := s.visibleStudent(principal, id)
	if err != nil {
		return nil, err
	}
	records := make([]learning.StudentLearningRecord, 0)
	for _, material := range s.materialsForStudent(id) {
		records = append(records, learning.StudentLearningRecord{
			ID: "learn-" + material.ID, Type: "资料", Title: material.Title, Course: material.Course,
			Status: "已学习", OccurredAt: firstNonEmpty(student.LastStudyAt, "2026-05-22 18:20:00"), Description: "查看课件资料",
		})
	}
	for _, review := range s.reviews {
		if review.StudentName != student.Name {
			continue
		}
		records = append(records, learning.StudentLearningRecord{
			ID: "review-" + review.ID, Type: "小挑战", Title: review.Homework, Course: review.PackageName,
			Status: review.Status, Score: review.SystemScore, OccurredAt: "2026-05-22 20:10:00", Description: "提交后等待老师反馈",
		})
	}
	for _, feedback := range s.lessonFeedbacks {
		if feedback.StudentID != id {
			continue
		}
		records = append(records, learning.StudentLearningRecord{ID: "lesson-" + feedback.ID, Type: "课后反馈", Title: feedback.CourseName, Course: feedback.TeacherName, Status: "已反馈", OccurredAt: firstNonEmpty(feedback.LessonDate, feedback.UpdatedAt), Description: feedback.Summary})
	}
	for _, summary := range s.scoreSummariesForStudent(id) {
		if summary.LatestRecord == nil {
			continue
		}
		score := *summary.LatestRecord
		records = append(records, learning.StudentLearningRecord{
			ID: "score-" + score.ID, Type: "成绩", Title: score.ExamName, Course: score.Subject,
			Status: "已记录", Score: score.Score, FullScore: score.FullScore, OccurredAt: score.ExamDate, Description: summary.Description,
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].OccurredAt > records[j].OccurredAt })
	return records, nil
}

func (s *MemoryStore) studentScoresUnlocked(principal learning.Principal, id string) ([]learning.StudentScoreSummary, error) {
	if _, err := s.visibleStudent(principal, id); err != nil {
		return nil, err
	}
	return s.scoreSummariesForStudent(id), nil
}

func (s *MemoryStore) studentOwnScoresUnlocked(principal learning.Principal) ([]learning.StudentScoreSummary, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	if _, err := s.visibleStudent(principal, principal.StudentID); err != nil {
		return nil, err
	}
	return s.scoreSummariesForStudent(principal.StudentID), nil
}

func (s *MemoryStore) createStudentScoreUnlocked(operator string, principal learning.Principal, studentID string, req learning.StudentScoreUpsertRequest) (learning.StudentScoreRecord, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.StudentScoreRecord, error) {
			return work.createStudentScoreUnlocked(operator, principal, studentID, req)
		})
	}
	student, err := s.visibleStudent(principal, studentID)
	if err != nil {
		return learning.StudentScoreRecord{}, err
	}
	req, err = s.normalizeScoreRequest(principal, student, req)
	if err != nil {
		return learning.StudentScoreRecord{}, err
	}
	nowTime := time.Now()
	now := nowTime.Format("2006-01-02 15:04:05")
	item := learning.StudentScoreRecord{
		ID:             "score-" + nowTime.Format("20060102150405") + "-" + strconv.Itoa(nowTime.Nanosecond()),
		StudentID:      student.ID,
		Subject:        req.Subject,
		ExamType:       req.ExamType,
		ExamName:       req.ExamName,
		ExamDate:       req.ExamDate,
		Score:          req.Score,
		FullScore:      req.FullScore,
		AverageScore:   req.AverageScore,
		TeacherComment: req.TeacherComment,
		CreatedBy:      principal.Name,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.scoreRecords = append([]learning.StudentScoreRecord{item}, s.scoreRecords...)
	s.prependLogDetail(operator, "录入成绩", student.Name, "学科: "+item.Subject+"; 测评: "+item.ExamName)
	return item, nil
}

func (s *MemoryStore) updateStudentScoreUnlocked(operator string, principal learning.Principal, studentID string, scoreID string, req learning.StudentScoreUpsertRequest) (learning.StudentScoreRecord, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.StudentScoreRecord, error) {
			return work.updateStudentScoreUnlocked(operator, principal, studentID, scoreID, req)
		})
	}
	student, err := s.visibleStudent(principal, studentID)
	if err != nil {
		return learning.StudentScoreRecord{}, err
	}
	req, err = s.normalizeScoreRequest(principal, student, req)
	if err != nil {
		return learning.StudentScoreRecord{}, err
	}
	for i := range s.scoreRecords {
		if s.scoreRecords[i].ID != scoreID || s.scoreRecords[i].StudentID != student.ID {
			continue
		}
		s.scoreRecords[i].Subject = req.Subject
		s.scoreRecords[i].ExamType = req.ExamType
		s.scoreRecords[i].ExamName = req.ExamName
		s.scoreRecords[i].ExamDate = req.ExamDate
		s.scoreRecords[i].Score = req.Score
		s.scoreRecords[i].FullScore = req.FullScore
		s.scoreRecords[i].AverageScore = req.AverageScore
		s.scoreRecords[i].TeacherComment = req.TeacherComment
		s.scoreRecords[i].UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
		s.prependLogDetail(operator, "修正成绩", student.Name, "学科: "+req.Subject+"; 测评: "+req.ExamName)
		return s.scoreRecords[i], nil
	}
	return learning.StudentScoreRecord{}, errors.New("成绩记录不存在")
}
