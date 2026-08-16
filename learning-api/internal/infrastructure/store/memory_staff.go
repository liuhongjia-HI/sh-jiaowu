package store

import (
	"errors"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) adminStaffUnlocked() []learning.AdminStaff {
	out := make([]learning.AdminStaff, 0)
	for _, user := range s.users {
		if !isAdminStaffUser(user) {
			continue
		}
		out = append(out, adminStaffFromUser(user))
	}
	return out
}

func (s *MemoryStore) createAdminStaffUnlocked(operator string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.AdminStaff, error) {
			return work.createAdminStaffUnlocked(operator, req)
		})
	}
	req, err := normalizeAdminStaffRequest(req, false)
	if err != nil {
		return learning.AdminStaff{}, err
	}
	if s.userPhoneExists("", req.Phone) {
		return learning.AdminStaff{}, errors.New("手机号已存在")
	}
	user := learning.User{
		ID:                 "user-admin-" + time.Now().Format("20060102150405"),
		Name:               req.Name,
		Phone:              req.Phone,
		PasswordHash:       mustPasswordHash(demoLoginPassword),
		MustChangePassword: true,
		AccountStatus:      "正常",
		Roles:              []learning.Role{req.Role},
		CampusID:           req.CampusID,
		Remark:             req.Remark,
	}
	s.users = append(s.users, user)
	s.prependLog(operator, "新增管理人员", user.Name+" / "+roleName(req.Role))
	return adminStaffFromUser(user), nil
}

func (s *MemoryStore) updateAdminStaffUnlocked(operator string, principal learning.Principal, id string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.AdminStaff, error) {
			return work.updateAdminStaffUnlocked(operator, principal, id, req)
		})
	}
	req, err := normalizeAdminStaffRequest(req, true)
	if err != nil {
		return learning.AdminStaff{}, err
	}
	for i := range s.users {
		if s.users[i].ID != id {
			continue
		}
		if !isAdminStaffUser(s.users[i]) {
			return learning.AdminStaff{}, errors.New("只能编辑管理人员账号")
		}
		if id == principal.UserID && req.AccountStatus != "正常" {
			return learning.AdminStaff{}, errors.New("不能停用当前登录账号")
		}
		if s.userPhoneExists(id, req.Phone) {
			return learning.AdminStaff{}, errors.New("手机号已存在")
		}
		wasSuper := hasRole(s.users[i].Roles, learning.RoleSuperAdmin) && s.users[i].AccountStatus == "正常"
		willSuper := req.Role == learning.RoleSuperAdmin && req.AccountStatus == "正常"
		if wasSuper && !willSuper && s.activeSuperAdminCount() <= 1 {
			return learning.AdminStaff{}, errors.New("至少保留一个正常的超级管理员")
		}
		before := adminStaffFromUser(s.users[i])
		s.users[i].Name = req.Name
		s.users[i].Phone = req.Phone
		s.users[i].Roles = []learning.Role{req.Role}
		s.users[i].CampusID = req.CampusID
		s.users[i].AccountStatus = req.AccountStatus
		s.users[i].Remark = req.Remark
		after := adminStaffFromUser(s.users[i])
		s.prependLogDetail(operator, "更新管理人员", s.users[i].Name+" / "+roleName(req.Role), auditChangeDetail(adminStaffAuditSnapshot(before), adminStaffAuditSnapshot(after)))
		return after, nil
	}
	return learning.AdminStaff{}, errors.New("admin staff not found")
}

func (s *MemoryStore) teachersUnlocked(principal learning.Principal) []learning.Teacher {
	out := make([]learning.Teacher, 0)
	for _, user := range s.users {
		if !hasRole(user.Roles, learning.RoleTeacher) {
			continue
		}
		if hasRole(principal.Roles, learning.RoleTeacher) && principal.UserID == user.ID {
			out = append(out, s.teacherFromUser(user))
			continue
		}
		if !canManageTeacher(principal, user) {
			continue
		}
		out = append(out, s.teacherFromUser(user))
	}
	return out
}

func (s *MemoryStore) createTeacherUnlocked(operator string, principal learning.Principal, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Teacher, error) {
			return work.createTeacherUnlocked(operator, principal, req)
		})
	}
	req, err := s.normalizeTeacherRequest(principal, req, false)
	if err != nil {
		return learning.Teacher{}, err
	}
	for _, user := range s.users {
		if user.Phone == req.Phone {
			return learning.Teacher{}, errors.New("手机号已存在")
		}
	}
	temporaryPassword, err := generateTemporaryPassword()
	if err != nil {
		return learning.Teacher{}, errors.New("临时密码生成失败")
	}
	user := learning.User{
		ID:                 "user-teacher-" + time.Now().Format("20060102150405"),
		Name:               req.Name,
		Phone:              req.Phone,
		PasswordHash:       mustPasswordHash(temporaryPassword),
		MustChangePassword: true,
		AccountStatus:      "正常",
		Roles:              []learning.Role{learning.RoleTeacher},
		CampusID:           req.CampusID,
		LearningSpaceIDs:   cloneStrings(req.LearningSpaceIDs),
		CanUploadHandout:   req.CanUploadHandout,
		CanUploadQuestion:  req.CanUploadQuestion,
		CanReview:          req.CanReview,
		Remark:             req.Remark,
	}
	s.users = append(s.users, user)
	s.prependLog(operator, "新增教师", user.Name)
	created := s.teacherFromUser(user)
	created.TemporaryPassword = temporaryPassword
	return created, nil
}

func (s *MemoryStore) updateTeacherUnlocked(operator string, principal learning.Principal, id string, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Teacher, error) {
			return work.updateTeacherUnlocked(operator, principal, id, req)
		})
	}
	req, err := s.normalizeTeacherRequest(principal, req, true)
	if err != nil {
		return learning.Teacher{}, err
	}
	for i := range s.users {
		if s.users[i].ID != id {
			continue
		}
		if !hasRole(s.users[i].Roles, learning.RoleTeacher) {
			return learning.Teacher{}, errors.New("只能编辑教师账号")
		}
		if !canManageTeacher(principal, s.users[i]) {
			return learning.Teacher{}, errors.New("不能管理其他校区教师")
		}
		for _, user := range s.users {
			if user.ID != id && user.Phone == req.Phone {
				return learning.Teacher{}, errors.New("手机号已存在")
			}
		}
		before := s.teacherFromUser(s.users[i])
		s.users[i].Name = req.Name
		s.users[i].Phone = req.Phone
		s.users[i].CampusID = req.CampusID
		s.users[i].LearningSpaceIDs = cloneStrings(req.LearningSpaceIDs)
		s.users[i].CanUploadHandout = req.CanUploadHandout
		s.users[i].CanUploadQuestion = req.CanUploadQuestion
		s.users[i].CanReview = req.CanReview
		s.users[i].AccountStatus = req.AccountStatus
		s.users[i].Remark = req.Remark
		after := s.teacherFromUser(s.users[i])
		s.prependLogDetail(operator, "更新教师", s.users[i].Name, auditChangeDetail(teacherAuditSnapshot(before), teacherAuditSnapshot(after)))
		return after, nil
	}
	return learning.Teacher{}, errors.New("teacher not found")
}
