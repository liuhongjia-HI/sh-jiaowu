package store

import (
	"errors"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

func (s *MemoryStore) loginWithWechatResolvedUnlocked(req learning.WechatLoginRequest, openID string, realWechatLogin bool) (learning.Principal, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Principal, error) {
			return work.loginWithWechatResolvedUnlocked(req, openID, realWechatLogin)
		})
	}
	if req.Phone != "" {
		matches := make([]int, 0, 1)
		for i, user := range s.users {
			if user.Phone != req.Phone || !canBindByPhone(user) {
				continue
			}
			matches = append(matches, i)
		}
		if len(matches) == 0 {
			if principal, ok, err := s.bindExistingStudentByMaskedPhone(openID, req); ok || err != nil {
				return principal, err
			}
			return s.createWechatStudentAccount(openID, req)
		}
		if len(matches) > 1 {
			return learning.Principal{}, errors.New("手机号匹配到多个账号，请联系老师确认后再绑定")
		}
		i := matches[0]
		user := s.users[i]
		if !canRebindByPhone(user, openID, realWechatLogin) {
			if hasRole(user.Roles, learning.RoleStudent) {
				return learning.Principal{}, errors.New("该学生已绑定其他微信，请联系老师处理")
			}
			return learning.Principal{}, errors.New("该手机号已绑定其他微信，请联系管理员处理")
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		if hasRole(user.Roles, learning.RoleStudent) {
			if err := s.validateStudentWechatBinding(user, openID, req); err != nil {
				return learning.Principal{}, err
			}
			s.applyStudentBindingProfile(user.StudentID, req)
		}
		s.users[i].OpenID = openID
		s.removeWechatOnlyStudent(openID, s.users[i].StudentID)
		action := "绑定教师微信"
		if hasRole(user.Roles, learning.RoleStudent) {
			action = "绑定学生微信"
		} else if isAdminStaffUser(user) {
			action = "绑定后台人员微信"
		}
		s.prependLog(user.Name, action, user.Name)
		return principalFromUser(s.users[i]), nil
	}
	for _, user := range s.users {
		if user.OpenID != openID {
			continue
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		return principalFromUser(user), nil
	}
	return learning.Principal{}, errors.New("微信账号未绑定，请先填写学生信息并授权手机号完成身份绑定")
}

func (s *MemoryStore) bindExistingStudentByMaskedPhone(openID string, req learning.WechatLoginRequest) (learning.Principal, bool, error) {
	matches := make([]int, 0, 1)
	for i, student := range s.students {
		if phoneSame(student.Phone, req.Phone) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return learning.Principal{}, false, nil
	}
	if len(matches) > 1 {
		return learning.Principal{}, true, errors.New("手机号匹配到多个学生档案，请联系老师确认后再绑定")
	}
	student := s.students[matches[0]]
	userIndex := s.findUserIndexByStudentID(student.ID)
	user := learning.User{
		ID:            "user-" + student.ID,
		Name:          firstNonEmpty(student.Name, req.StudentName),
		Phone:         req.Phone,
		AccountStatus: firstNonEmpty(student.AccountStatus, "正常"),
		Roles:         []learning.Role{learning.RoleStudent},
		StudentID:     student.ID,
	}
	if userIndex >= 0 {
		user = s.users[userIndex]
		if strings.TrimSpace(user.Phone) == "" || strings.Contains(user.Phone, "*") {
			user.Phone = req.Phone
		}
	}
	if user.AccountStatus != "正常" {
		return learning.Principal{}, true, errors.New("账号已停用，请联系管理员")
	}
	if err := s.validateStudentWechatBinding(user, openID, req); err != nil {
		return learning.Principal{}, true, err
	}
	user.OpenID = openID
	user.Name = firstNonEmpty(req.StudentName, user.Name)
	user.Phone = req.Phone
	user.AccountStatus = "正常"
	user.Roles = appendUniqueRoles(user.Roles, learning.RoleStudent)
	user.StudentID = student.ID
	if userIndex >= 0 {
		s.users[userIndex] = user
	} else {
		s.users = append(s.users, user)
	}
	s.applyStudentBindingProfile(student.ID, req)
	s.removeWechatOnlyStudent(openID, student.ID)
	s.prependLog(user.Name, "绑定学生微信", user.Name)
	return principalFromUser(user), true, nil
}

func (s *MemoryStore) createWechatStudentAccount(openID string, req learning.WechatLoginRequest) (learning.Principal, error) {
	if req.StudentName == "" || req.SchoolName == "" || req.Grade == "" {
		return learning.Principal{}, errors.New("请填写学生姓名、学校和年级后再绑定")
	}
	if req.Phone == "" {
		return learning.Principal{}, errors.New("请授权手机号后再登录")
	}
	if s.phoneExists("", req.Phone) {
		return learning.Principal{}, errors.New("手机号已存在，请联系老师确认学生档案")
	}
	nowID := time.Now().Format("20060102150405.000000000")
	studentID := "stu-wx-" + nowID
	student := learning.Student{
		ID:                     studentID,
		Name:                   req.StudentName,
		EnrollmentAcademicYear: currentAcademicYear(),
		EnrollmentGrade:        req.Grade,
		Phone:                  req.Phone,
		SchoolName:             req.SchoolName,
		OpenedPackages:         []string{},
		LearningStatus:         "待开通",
		AccountStatus:          "正常",
		Remark:                 "微信授权自动创建，待后台开通学习权限",
		BindStatus:             "已绑定",
	}
	user := learning.User{
		ID:            "user-" + studentID,
		Name:          req.StudentName,
		Phone:         req.Phone,
		OpenID:        openID,
		AccountStatus: "正常",
		Remark:        student.Remark,
		Roles:         []learning.Role{learning.RoleStudent},
		StudentID:     studentID,
	}
	s.students = append([]learning.Student{student}, s.students...)
	s.users = append(s.users, user)
	s.removeWechatOnlyStudent(openID, studentID)
	s.prependLogDetail("微信登录", "微信授权创建学生", student.Name, "手机号未匹配后台档案，已创建待开通学生账号")
	return principalFromUser(user), nil
}

func (s *MemoryStore) removeWechatOnlyStudent(openID, keepStudentID string) {
	removeStudentIDs := map[string]bool{}
	users := make([]learning.User, 0, len(s.users))
	for _, user := range s.users {
		if user.OpenID == openID && user.StudentID != keepStudentID && user.Phone == "" && hasRole(user.Roles, learning.RoleStudent) {
			removeStudentIDs[user.StudentID] = true
			continue
		}
		users = append(users, user)
	}
	if len(removeStudentIDs) == 0 {
		return
	}
	students := make([]learning.Student, 0, len(s.students))
	for _, student := range s.students {
		if removeStudentIDs[student.ID] && student.Remark == "微信授权自动创建" {
			continue
		}
		students = append(students, student)
	}
	s.users = users
	s.students = students
}

func (s *MemoryStore) loginWithAdminPasswordUnlocked(phone, password string) (learning.Principal, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" || password == "" {
		return learning.Principal{}, errors.New("请输入手机号和密码")
	}
	for _, user := range s.users {
		if user.Phone != phone {
			continue
		}
		if !hasRole(user.Roles, learning.RoleTeacher) && !isAdminStaffUser(user) {
			return learning.Principal{}, errors.New("手机号或密码错误")
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			return learning.Principal{}, errors.New("手机号或密码错误")
		}
		return principalFromUser(user), nil
	}
	return learning.Principal{}, errors.New("手机号或密码错误")
}

func (s *MemoryStore) changePasswordUnlocked(operator string, principal learning.Principal, req learning.PasswordChangeRequest) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error { return work.changePasswordUnlocked(operator, principal, req) })
	}
	req.OldPassword = strings.TrimSpace(req.OldPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.OldPassword == "" || req.NewPassword == "" {
		return errors.New("请输入原密码和新密码")
	}
	if err := validateNewPassword(req.NewPassword); err != nil {
		return err
	}
	for i := range s.users {
		if s.users[i].ID != principal.UserID {
			continue
		}
		if s.users[i].PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(s.users[i].PasswordHash), []byte(req.OldPassword)) != nil {
			s.prependLogDetail(operator, "修改密码失败", s.users[i].Name, "原密码错误")
			return errors.New("原密码不正确")
		}
		if bcrypt.CompareHashAndPassword([]byte(s.users[i].PasswordHash), []byte(req.NewPassword)) == nil {
			return errors.New("新密码不能和原密码相同")
		}
		s.users[i].PasswordHash = mustPasswordHash(req.NewPassword)
		s.users[i].MustChangePassword = false
		s.users[i].TokenVersion++
		s.prependLogDetail(operator, "修改密码", s.users[i].Name, "用户主动修改密码")
		return nil
	}
	return errors.New("账号不存在，请重新登录")
}

func (s *MemoryStore) resetPasswordUnlocked(operator string, principal learning.Principal, userID string) (learning.PasswordResetResult, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.PasswordResetResult, error) {
			return work.resetPasswordUnlocked(operator, principal, userID)
		})
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return learning.PasswordResetResult{}, errors.New("请选择账号")
	}
	for i := range s.users {
		if s.users[i].ID != userID {
			continue
		}
		if !canResetPassword(principal, s.users[i]) {
			return learning.PasswordResetResult{}, errors.New("没有权限重置该账号密码")
		}
		temp, err := generateTemporaryPassword()
		if err != nil {
			return learning.PasswordResetResult{}, errors.New("临时密码生成失败")
		}
		s.users[i].PasswordHash = mustPasswordHash(temp)
		s.users[i].MustChangePassword = true
		s.users[i].TokenVersion++
		s.prependLogDetail(operator, "重置密码", s.users[i].Name, "已生成临时密码并要求下次登录修改")
		return learning.PasswordResetResult{UserID: s.users[i].ID, TemporaryPassword: temp, MustChangePassword: true}, nil
	}
	return learning.PasswordResetResult{}, errors.New("账号不存在")
}

func (s *MemoryStore) recordSecurityEventUnlocked(operator, action, target, detail string) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error {
			return work.recordSecurityEventUnlocked(operator, action, target, detail)
		})
	}
	s.prependLogDetail(operator, action, target, detail)
	return nil
}

func (s *MemoryStore) loginWithDemoStudentPasswordUnlocked(phone, password string) (learning.Principal, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" || password != demoLoginPassword {
		return learning.Principal{}, errors.New("手机号或密码错误")
	}
	for _, user := range s.users {
		if user.Phone != phone || !hasRole(user.Roles, learning.RoleStudent) {
			continue
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		return principalFromUser(user), nil
	}
	return learning.Principal{}, errors.New("手机号或密码错误")
}

func (s *MemoryStore) principalByUserIDUnlocked(userID string) (learning.Principal, error) {
	for _, user := range s.users {
		if user.ID != userID {
			continue
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		return principalFromUser(user), nil
	}
	return learning.Principal{}, errors.New("账号不存在，请重新登录")
}
