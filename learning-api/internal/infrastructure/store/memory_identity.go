package store

import (
	"errors"
	"strings"

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
			// 多子女：手机号命中多个学生账号。家长带着选中的 studentId 重新提交时，
			// 把匹配范围收窄到那一个；否则把候选列表交给前端弹选择框，而不是直接拒绝登录。
			if req.SelectedStudentID != "" {
				narrowed := -1
				for _, idx := range matches {
					if s.users[idx].StudentID == req.SelectedStudentID {
						narrowed = idx
						break
					}
				}
				if narrowed == -1 {
					return learning.Principal{}, errors.New("选择的学生账号不存在，请重新选择")
				}
				matches = []int{narrowed}
			} else if candidates, ok := s.studentSelectionCandidates(matches); ok {
				return learning.Principal{}, &learning.StudentSelectionRequiredError{Candidates: candidates}
			} else {
				return learning.Principal{}, errors.New("手机号匹配到多个账号，请联系老师确认后再绑定")
			}
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
		principal := principalFromUser(s.users[i])
		if hasRole(user.Roles, learning.RoleStudent) {
			// 家长身份只对学生登录有意义，老师/管理员用手机号绑定跟"家长-孩子"
			// 关系表完全无关。
			principal.GuardianID = s.ensureGuardianLink(req.Phone, openID, s.users[i].StudentID)
		}
		return principal, nil
	}
	for _, user := range s.users {
		if user.OpenID != openID {
			continue
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		if !hasRole(user.Roles, learning.RoleStudent) {
			return principalFromUser(user), nil
		}
		// 静默重新登录（只带 openID，不带手机号）：优先恢复家长上次查看的孩子，
		// 而不是死认最初完成手机号绑定时用的那个 user 记录——不然家长在"我的"
		// 页切到老二之后，小程序一重启又跳回老大，跟切换器承诺的行为对不上。
		principal := principalFromUser(user)
		if idx, ok := s.findGuardianByOpenIDIndex(openID); ok {
			guardian := s.guardians[idx]
			principal.GuardianID = guardian.ID
			if guardian.LastStudentID != "" && guardian.LastStudentID != user.StudentID &&
				s.guardianStudentActive(guardian.ID, guardian.LastStudentID) {
				if lastUser, ok := s.findUserByStudentID(guardian.LastStudentID); ok && lastUser.AccountStatus == "正常" {
					resumed := principalFromUser(lastUser)
					resumed.GuardianID = guardian.ID
					return resumed, nil
				}
			}
		}
		return principal, nil
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
		// 多子女：几个孩子还没有各自的登录账号（user 记录），只有学生档案。
		// 同样走"选择哪一个"而不是直接拒绝，逻辑和上面 user 匹配的分支对称。
		if req.SelectedStudentID != "" {
			narrowed := -1
			for _, idx := range matches {
				if s.students[idx].ID == req.SelectedStudentID {
					narrowed = idx
					break
				}
			}
			if narrowed == -1 {
				return learning.Principal{}, true, errors.New("选择的学生账号不存在，请重新选择")
			}
			matches = []int{narrowed}
		} else {
			candidates := make([]learning.StudentAccount, 0, len(matches))
			for _, idx := range matches {
				student := s.students[idx]
				if student.AccountStatus != "正常" {
					continue
				}
				decorated := s.decorateStudent(student)
				candidates = append(candidates, learning.StudentAccount{StudentID: student.ID, Name: student.Name, Grade: decorated.Grade})
			}
			if len(candidates) < 2 {
				return learning.Principal{}, true, errors.New("手机号匹配到多个学生档案，请联系老师确认后再绑定")
			}
			return learning.Principal{}, true, &learning.StudentSelectionRequiredError{Candidates: candidates}
		}
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
	principal := principalFromUser(user)
	principal.GuardianID = s.ensureGuardianLink(req.Phone, openID, student.ID)
	return principal, true, nil
}

// createWechatStudentAccount 曾经会在手机号匹配不到任何后台档案时自动建一个学生账号。
// 这是多子女/多家长脏数据的根源：家长换个手机号授权（比如爸爸换成妈妈的号），
// 系统会静默地把同一个孩子建出第二份档案、第二套套餐，作业记录也跟着分裂成两边，
// 且过程中没有任何报错提示。建档现在必须只走后台，微信这边查不到就明确告诉家长
// 联系老师，而不是替他建一个"待开通"的影子账号。
func (s *MemoryStore) createWechatStudentAccount(openID string, req learning.WechatLoginRequest) (learning.Principal, error) {
	return learning.Principal{}, errors.New("未找到学生档案，请联系老师完成学生建档后再授权绑定")
}

// studentSelectionCandidates 把命中同一手机号的多个 user 记录转成家长可以看懂的
// "选哪个孩子"候选列表。只在匹配到的账号全部是纯学生角色时才返回候选——如果混进了
// 老师/管理员账号，说明这是真的手机号冲突而不是多子女，仍然要走原来的拒绝逻辑。
func (s *MemoryStore) studentSelectionCandidates(matches []int) ([]learning.StudentAccount, bool) {
	candidates := make([]learning.StudentAccount, 0, len(matches))
	for _, idx := range matches {
		user := s.users[idx]
		if len(user.Roles) != 1 || user.Roles[0] != learning.RoleStudent {
			return nil, false
		}
		student, ok := s.findStudent(user.StudentID)
		if !ok || student.AccountStatus != "正常" {
			continue
		}
		decorated := s.decorateStudent(student)
		candidates = append(candidates, learning.StudentAccount{StudentID: student.ID, Name: student.Name, Grade: decorated.Grade})
	}
	if len(candidates) < 2 {
		return nil, false
	}
	return candidates, true
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
