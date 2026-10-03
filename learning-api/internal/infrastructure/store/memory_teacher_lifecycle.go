package store

import (
	"errors"
	"slices"
	"starline/learning-api/internal/domain/learning"
)

// Status changes preserve the current scopes and teaching records, including
// legacy scopes which may no longer be selectable in the editing form.
func (s *MemoryStore) SetTeacherStatus(operator string, principal learning.Principal, id, status string) (learning.Teacher, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.Teacher, error) {
		if status != "正常" && status != "停用" {
			return learning.Teacher{}, errors.New("账号状态只能为正常或停用")
		}
		index, err := work.managedTeacherIndex(principal, id)
		if err != nil {
			return learning.Teacher{}, err
		}
		user := &work.users[index]
		before := work.teacherFromUser(*user)
		if user.AccountStatus == "正常" && status == "停用" {
			user.TokenVersion++
		}
		user.AccountStatus = status
		after := work.teacherFromUser(*user)
		work.prependLogDetail(operator, "更新教师状态", user.Name, auditChangeDetail(teacherAuditSnapshot(before), teacherAuditSnapshot(after)))
		return after, nil
	})
}

func (s *MemoryStore) managedTeacherIndex(principal learning.Principal, id string) (int, error) {
	for i, user := range s.users {
		if user.ID != id {
			continue
		}
		if !hasRole(user.Roles, learning.RoleTeacher) {
			return -1, errors.New("只能管理教师账号")
		}
		if !canManageTeacher(principal, user) {
			return -1, errors.New("无权管理该教师")
		}
		return i, nil
	}
	return -1, errors.New("教师不存在")
}

func (s *MemoryStore) DeleteTeacher(operator string, principal learning.Principal, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		index, err := work.managedTeacherIndex(principal, id)
		if err != nil {
			return err
		}
		user := work.users[index]
		if len(user.Roles) != 1 || user.StudentID != "" {
			return errors.New("账号还有其他身份，不能删除，请停用")
		}
		if reason := work.teacherDeletionBlock(id); reason != "" {
			return errors.New("该教师关联" + reason + "，不能删除，请停用账号以保留记录")
		}
		work.users = slices.Delete(work.users, index, index+1)
		work.availability = slices.DeleteFunc(work.availability, func(v learning.AvailabilitySlot) bool { return v.OwnerType == "teacher" && v.OwnerID == id })
		work.teacherMaterialReads = slices.DeleteFunc(work.teacherMaterialReads, func(v teacherMaterialRead) bool { return v.UserID == id })
		work.prependLogDetail(operator, "删除教师", user.Name, auditChangeDetail(teacherAuditSnapshot(work.teacherFromUser(user)), nil))
		return nil
	})
}

// Historical and cancelled records also block deletion. Only an unused test
// account can be removed; its account-owned preferences are cleaned with it.
func (s *MemoryStore) teacherDeletionBlock(id string) string {
	for _, v := range s.scheduleClasses {
		if v.TeacherID == id || v.CreatedBy == id || v.AuditedBy == id {
			return "排课记录"
		}
	}
	for _, v := range s.tutoringAssignments {
		if v.TeacherID == id || v.AssignedBy == id || v.EndedBy == id {
			return "辅导关系"
		}
	}
	for _, v := range s.lessonFeedbacks {
		if v.TeacherID == id {
			return "课程反馈"
		}
	}
	for _, v := range s.reviews {
		if v.ReviewerTeacherID == id {
			return "批改记录"
		}
	}
	for _, v := range s.questionBank {
		if v.OwnerTeacherID == id {
			return "题库"
		}
	}
	for _, v := range s.materials {
		if v.OwnerTeacherID == id {
			return "学习资料"
		}
	}
	for _, v := range s.homework {
		if v.OwnerTeacherID == id {
			return "练习"
		}
	}
	for _, v := range s.teachingPlans {
		if v.UploaderID == id {
			return "教案"
		}
	}
	for _, v := range s.scoreRecords {
		if v.CreatedBy == id {
			return "成绩记录"
		}
	}
	for _, v := range s.materialDownloads {
		if v.OwnerID == id {
			return "资料下载任务"
		}
	}
	return ""
}
