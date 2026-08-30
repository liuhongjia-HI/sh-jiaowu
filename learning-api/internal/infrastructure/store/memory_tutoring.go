package store

import (
	"errors"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) StudentTutoringAssignments(principal learning.Principal, studentID string) ([]learning.TutoringAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.studentTutoringAssignmentsUnlocked(principal, studentID)
}

func (s *MemoryStore) CreateTutoringAssignment(operator string, principal learning.Principal, studentID string, req learning.TutoringAssignmentCreateRequest) (learning.TutoringAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createTutoringAssignmentUnlocked(operator, principal, studentID, req)
}

func (s *MemoryStore) EndTutoringAssignment(operator string, principal learning.Principal, id string, req learning.TutoringAssignmentEndRequest) (learning.TutoringAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endTutoringAssignmentUnlocked(operator, principal, id, req)
}

func (s *MemoryStore) TransferTutoringAssignment(operator string, principal learning.Principal, id string, req learning.TutoringAssignmentTransferRequest) (learning.TutoringAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transferTutoringAssignmentUnlocked(operator, principal, id, req)
}

func (s *MemoryStore) studentTutoringAssignmentsUnlocked(principal learning.Principal, studentID string) ([]learning.TutoringAssignment, error) {
	if _, err := s.visibleStudent(principal, studentID); err != nil {
		return nil, err
	}
	out := make([]learning.TutoringAssignment, 0)
	for _, item := range s.tutoringAssignments {
		if item.StudentID == studentID {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status == learning.TutoringAssignmentActive
		}
		return out[i].StartsAt > out[j].StartsAt
	})
	return out, nil
}

func (s *MemoryStore) createTutoringAssignmentUnlocked(operator string, principal learning.Principal, studentID string, req learning.TutoringAssignmentCreateRequest) (learning.TutoringAssignment, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.TutoringAssignment, error) {
			return work.createTutoringAssignmentUnlocked(operator, principal, studentID, req)
		})
	}
	if !canManageTutoringAssignments(principal) {
		return learning.TutoringAssignment{}, errors.New("没有权限分配辅导老师")
	}
	student, ok := s.findStudent(studentID)
	if !ok {
		return learning.TutoringAssignment{}, errors.New("学生不存在")
	}
	req, teacher, subject, err := s.normalizeTutoringAssignmentRequest(student, req)
	if err != nil {
		return learning.TutoringAssignment{}, err
	}
	if !canManageTeacher(principal, teacher) && !hasRole(principal.Roles, learning.RoleOpsStaff) {
		return learning.TutoringAssignment{}, errors.New("不能分配其他校区老师")
	}
	if req.Role == learning.TutoringAssignmentPrimary && s.hasActivePrimaryAssignment(student.ID, req.SubjectID, req.LevelCode, "") {
		return learning.TutoringAssignment{}, errors.New("该学生当前学科已有主辅导老师，请先转交或结束原关系")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	item := learning.TutoringAssignment{
		ID: "ta-" + time.Now().Format("20060102150405.000000000"), StudentID: student.ID,
		TeacherID: teacher.ID, TeacherName: teacher.Name, CampusID: teacher.CampusID,
		AcademicYear: s.configuredAcademicYear(), GradeSnapshot: student.Grade,
		SubjectID: req.SubjectID, SubjectName: subject.Name, LevelCode: req.LevelCode,
		Role: req.Role, Status: learning.TutoringAssignmentActive, SourceType: req.SourceType, SourceID: req.SourceID,
		StartsAt: req.StartsAt, AssignedBy: operator, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	s.tutoringAssignments = append(s.tutoringAssignments, item)
	s.prependLogDetail(operator, "分配辅导老师", student.Name, "学科 "+item.SubjectName+" / 老师 "+item.TeacherName)
	return item, nil
}

func (s *MemoryStore) endTutoringAssignmentUnlocked(operator string, principal learning.Principal, id string, req learning.TutoringAssignmentEndRequest) (learning.TutoringAssignment, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.TutoringAssignment, error) {
			return work.endTutoringAssignmentUnlocked(operator, principal, id, req)
		})
	}
	if !canManageTutoringAssignments(principal) {
		return learning.TutoringAssignment{}, errors.New("没有权限结束辅导关系")
	}
	req.EndsAt = strings.TrimSpace(req.EndsAt)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.EndsAt == "" {
		req.EndsAt = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", req.EndsAt); err != nil {
		return learning.TutoringAssignment{}, errors.New("结束日期格式应为 YYYY-MM-DD")
	}
	if req.Reason == "" {
		return learning.TutoringAssignment{}, errors.New("请填写结束原因")
	}
	for index := range s.tutoringAssignments {
		item := &s.tutoringAssignments[index]
		if item.ID != strings.TrimSpace(id) {
			continue
		}
		if item.Status != learning.TutoringAssignmentActive {
			return learning.TutoringAssignment{}, errors.New("该辅导关系已结束")
		}
		if req.Version != 0 && req.Version != item.Version {
			return learning.TutoringAssignment{}, errors.New("辅导关系已被其他人更新，请刷新后重试")
		}
		item.Status = learning.TutoringAssignmentEnded
		item.EndsAt = req.EndsAt
		item.EndedReason = req.Reason
		item.EndedBy = operator
		item.Version++
		item.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
		student, _ := s.findStudent(item.StudentID)
		s.prependLogDetail(operator, "结束辅导关系", student.Name, "老师 "+item.TeacherName+" / 原因 "+req.Reason)
		return *item, nil
	}
	return learning.TutoringAssignment{}, errors.New("辅导关系不存在")
}

func (s *MemoryStore) transferTutoringAssignmentUnlocked(operator string, principal learning.Principal, id string, req learning.TutoringAssignmentTransferRequest) (learning.TutoringAssignment, error) {
	if !canManageTutoringAssignments(principal) {
		return learning.TutoringAssignment{}, errors.New("没有权限转交辅导关系")
	}
	var current learning.TutoringAssignment
	for _, item := range s.tutoringAssignments {
		if item.ID == strings.TrimSpace(id) {
			current = item
			break
		}
	}
	if current.ID == "" {
		return learning.TutoringAssignment{}, errors.New("辅导关系不存在")
	}
	if current.Status != learning.TutoringAssignmentActive {
		return learning.TutoringAssignment{}, errors.New("只能转交有效的辅导关系")
	}
	if req.Version != 0 && req.Version != current.Version {
		return learning.TutoringAssignment{}, errors.New("辅导关系已被其他人更新，请刷新后重试")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return learning.TutoringAssignment{}, errors.New("请填写转交原因")
	}
	startsAt := strings.TrimSpace(req.StartsAt)
	if startsAt == "" {
		startsAt = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", startsAt); err != nil {
		return learning.TutoringAssignment{}, errors.New("生效日期格式应为 YYYY-MM-DD")
	}
	if strings.TrimSpace(req.TeacherID) == current.TeacherID {
		return learning.TutoringAssignment{}, errors.New("请选择其他老师")
	}
	if _, err := s.endTutoringAssignmentUnlocked(operator, principal, current.ID, learning.TutoringAssignmentEndRequest{EndsAt: startsAt, Reason: req.Reason, Version: current.Version}); err != nil {
		return learning.TutoringAssignment{}, err
	}
	return s.createTutoringAssignmentUnlocked(operator, principal, current.StudentID, learning.TutoringAssignmentCreateRequest{
		TeacherID: req.TeacherID, SubjectID: current.SubjectID, LevelCode: current.LevelCode,
		Role: current.Role, StartsAt: startsAt, SourceType: "manual", SourceID: current.ID,
	})
}

func (s *MemoryStore) normalizeTutoringAssignmentRequest(student learning.Student, req learning.TutoringAssignmentCreateRequest) (learning.TutoringAssignmentCreateRequest, learning.User, learning.SubjectMetadata, error) {
	req.TeacherID = strings.TrimSpace(req.TeacherID)
	req.SubjectID = strings.TrimSpace(req.SubjectID)
	req.LevelCode = strings.TrimSpace(req.LevelCode)
	req.Role = strings.TrimSpace(req.Role)
	req.StartsAt = strings.TrimSpace(req.StartsAt)
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.SourceID = strings.TrimSpace(req.SourceID)
	if req.Role == "" {
		req.Role = learning.TutoringAssignmentPrimary
	}
	if req.Role != learning.TutoringAssignmentPrimary && req.Role != learning.TutoringAssignmentAssistant {
		return req, learning.User{}, learning.SubjectMetadata{}, errors.New("辅导角色只能为主辅导老师或协作老师")
	}
	if req.StartsAt == "" {
		req.StartsAt = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", req.StartsAt); err != nil {
		return req, learning.User{}, learning.SubjectMetadata{}, errors.New("生效日期格式应为 YYYY-MM-DD")
	}
	if req.SourceType == "" {
		req.SourceType = "manual"
	}
	if req.SourceType != "manual" && req.SourceType != "schedule_approval" && req.SourceType != "import" {
		return req, learning.User{}, learning.SubjectMetadata{}, errors.New("辅导关系来源不正确")
	}
	if req.TeacherID == "" || req.SubjectID == "" || req.LevelCode == "" {
		return req, learning.User{}, learning.SubjectMetadata{}, errors.New("请选择老师、学科和等级")
	}
	teacher, ok := s.findUser(req.TeacherID)
	if !ok || !isActiveTeacher(teacher) {
		return req, learning.User{}, learning.SubjectMetadata{}, errors.New("辅导老师不存在或已停用")
	}
	subject, ok := s.subjectByID(req.SubjectID)
	if !ok || subject.Status != "启用" {
		return req, learning.User{}, learning.SubjectMetadata{}, errors.New("辅导学科不存在或已停用")
	}
	if !s.teacherCoversStudentScope(teacher, student, subject.Name, req.LevelCode) {
		return req, learning.User{}, learning.SubjectMetadata{}, errors.New("该老师未覆盖学生当前年级、学科和等级")
	}
	return req, teacher, subject, nil
}

func (s *MemoryStore) subjectByID(id string) (learning.SubjectMetadata, bool) {
	for _, subject := range s.subjects {
		if subject.ID == id {
			return subject, true
		}
	}
	return learning.SubjectMetadata{}, false
}

func (s *MemoryStore) teacherCoversStudentScope(teacher learning.User, student learning.Student, subject, level string) bool {
	for _, space := range s.learningSpaces {
		if containsString(teacher.LearningSpaceIDs, space.ID) && space.Status == learning.StatusEnabled && space.Grade == student.Grade && subjectsMatch(space.Subject, subject) && space.Level == level {
			return true
		}
	}
	return false
}

func (s *MemoryStore) hasActivePrimaryAssignment(studentID, subjectID, levelCode, exceptID string) bool {
	for _, item := range s.tutoringAssignments {
		if item.ID != exceptID && item.StudentID == studentID && item.SubjectID == subjectID && item.LevelCode == levelCode && item.Role == learning.TutoringAssignmentPrimary && item.Status == learning.TutoringAssignmentActive {
			return true
		}
	}
	return false
}

func teacherHasActiveTutoringAssignment(teacherID, studentID string, assignments []learning.TutoringAssignment) bool {
	for _, item := range assignments {
		if item.TeacherID == teacherID && item.StudentID == studentID && item.Status == learning.TutoringAssignmentActive {
			return true
		}
	}
	return false
}

func canManageTutoringAssignments(principal learning.Principal) bool {
	return hasRole(principal.Roles, learning.RoleOpsStaff) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleSuperAdmin)
}
