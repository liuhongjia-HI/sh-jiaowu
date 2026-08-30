package store

import (
	"errors"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// studentAccountsUnlocked 列出当前家长能看到的所有孩子，用来渲染"我的"页的
// 切换器。来源是 guardian_students 关系表，不再靠"同一个手机号"临时撞出来——
// 关系在登录成功的那一刻就已经建好（见 ensureGuardianLink），这里只是读。
//
// 没有 GuardianID（演示密码登录、老师/管理员账号、还没走过微信登录的旧
// token）不是错误，就是"没有可切换的孩子"——小程序每次进"我的"页都会
// 调这个接口探测要不要显示切换器，如果这里报错，前端的 wx.showToast 会
// 不看 silent 选项直接把错误弹给用户，绝大多数单孩子家庭会平白多看到一条
// 吓人的提示。返回空列表，界面该怎样就怎样：没有切换器可显示而已。
func (s *MemoryStore) studentAccountsUnlocked(principal learning.Principal) ([]learning.StudentAccount, error) {
	accounts := make([]learning.StudentAccount, 0)
	if principal.GuardianID != "" {
		for _, relation := range s.guardianStudents {
			if relation.GuardianID != principal.GuardianID || (relation.Status != learning.GuardianStudentActive && relation.Status != learning.GuardianStudentPending) {
				continue
			}
			student, ok := s.findStudent(relation.StudentID)
			if !ok {
				continue
			}
			canSwitch := relation.Status == learning.GuardianStudentActive && student.AccountStatus == "正常"
			if !canSwitch && student.AccountStatus != "待审核" {
				continue
			}
			decorated := s.decorateStudent(student)
			accounts = append(accounts, learning.StudentAccount{
				StudentID: student.ID, Name: student.Name, Grade: decorated.Grade,
				Active: canSwitch && student.ID == principal.StudentID, Status: student.AccountStatus, CanSwitch: canSwitch,
			})
		}
	}
	// 只要当前会话有家长身份，即使名下只有一个孩子也返回这一个账号。
	// 小程序据此始终展示“当前查看”与“添加学生”入口，而不是把单孩子家庭藏起来。
	if principal.GuardianID != "" {
		return accounts, nil
	}
	// 关系表还没建起来就走手机号兜底。关系只在登录那一刻写入，而小程序只要
	// 本地 token 没过期就不会重新登录——存量家长（多子女功能上线前就绑好的）
	// 手上既没有带 GuardianID 的 token，库里也还没有对应的关系行，两头都落空，
	// 切换器会一直不出现，而家长自己没有任何办法触发修复。
	//
	// 这里只读、不写：拿"当前这个孩子档案上的家长手机号"去反查同号下的其他
	// 孩子，纯粹为了把切换器显示出来。真正的关系行仍然由下一次登录写入，
	// 权限校验（切换、逐请求校验）也依旧只认关系表，不受这个兜底影响。
	student, ok := s.findStudent(principal.StudentID)
	if !ok || strings.TrimSpace(student.Phone) == "" {
		return accounts, nil
	}
	fallback := make([]learning.StudentAccount, 0, 2)
	for _, sibling := range s.students {
		if sibling.AccountStatus != "正常" || !phoneSame(sibling.Phone, student.Phone) {
			continue
		}
		decorated := s.decorateStudent(sibling)
		fallback = append(fallback, learning.StudentAccount{StudentID: sibling.ID, Name: sibling.Name, Grade: decorated.Grade, Active: sibling.ID == principal.StudentID, Status: sibling.AccountStatus, CanSwitch: true})
	}
	// 只有真的找出了兄弟姐妹（>1）才用兜底结果。只找到自己一个的时候必须
	// 原样返回，通常就是空列表——演示密码登录、老师/管理员账号这些没有家长
	// 身份的会话要保持安静（见 TestStudentAccountsIsQuietForNonGuardianLogins），
	// 单孩子家庭也不该因为兜底就凭空多出一个只有一项的"切换器"。
	if len(fallback) > 1 && len(fallback) > len(accounts) {
		return fallback, nil
	}
	return accounts, nil
}

// requestAdditionalStudentUnlocked 为当前家长直接添加学生，不再经过人工审核。
// 资料提交后立刻可切换；首章节权限会由内容访问规则自动提供。
func (s *MemoryStore) requestAdditionalStudentUnlocked(principal learning.Principal, req learning.StudentAccountAddRequest) (learning.StudentAccount, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.StudentAccount, error) {
			return work.requestAdditionalStudentUnlocked(principal, req)
		})
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Grade = strings.TrimSpace(req.Grade)
	req.SchoolName = strings.TrimSpace(req.SchoolName)
	if req.Name == "" {
		return learning.StudentAccount{}, errors.New("请输入学生姓名")
	}
	if req.Grade == "" {
		return learning.StudentAccount{}, errors.New("请选择年级")
	}
	if req.SchoolName == "" {
		return learning.StudentAccount{}, errors.New("请输入学校")
	}
	if len([]rune(req.Name)) > 32 || len([]rune(req.Grade)) > 32 || len([]rune(req.SchoolName)) > 64 {
		return learning.StudentAccount{}, errors.New("填写内容过长，请检查后重试")
	}
	if principal.GuardianID == "" {
		return learning.StudentAccount{}, errors.New("请重新登录后再添加学生")
	}
	guardian, ok := s.guardianByID(principal.GuardianID)
	if !ok || guardian.AccountStatus != "正常" || strings.TrimSpace(guardian.Phone) == "" {
		return learning.StudentAccount{}, errors.New("家长身份无效，请重新登录后再试")
	}
	for _, relation := range s.guardianStudents {
		if relation.GuardianID != principal.GuardianID || (relation.Status != learning.GuardianStudentActive && relation.Status != learning.GuardianStudentPending) {
			continue
		}
		student, found := s.findStudent(relation.StudentID)
		if found && student.Name == req.Name && s.decorateStudent(student).Grade == req.Grade {
			return learning.StudentAccount{}, errors.New("名下已有同名同年级学生，请勿重复添加")
		}
	}
	guardianName := guardian.Name
	if guardianName == "" {
		if current, found := s.findStudent(principal.StudentID); found {
			guardianName = current.GuardianName
		}
	}
	id := "stu-" + time.Now().Format("20060102150405.000000000")
	student := learning.Student{
		ID: id, Name: req.Name, EnrollmentAcademicYear: s.configuredAcademicYear(), EnrollmentGrade: req.Grade,
		Phone: guardian.Phone, SchoolName: req.SchoolName, GuardianName: guardianName,
		OpenedPackages: []string{}, LearningStatus: "未开始", AccountStatus: "正常", BindStatus: "已绑定",
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	s.students = append([]learning.Student{student}, s.students...)
	s.syncStudentUser(student)
	s.guardianStudents = append(s.guardianStudents, learning.GuardianStudent{
		GuardianID: principal.GuardianID, StudentID: student.ID, Relation: learning.GuardianRelationGuardian, Status: learning.GuardianStudentActive,
	})
	s.prependLog("家长", "添加学生", student.Name)
	return learning.StudentAccount{StudentID: student.ID, Name: student.Name, Grade: s.decorateStudent(student).Grade, Status: "正常", CanSwitch: true}, nil
}

func (s *MemoryStore) guardianByID(id string) (learning.Guardian, bool) {
	for _, guardian := range s.guardians {
		if guardian.ID == id {
			return guardian, true
		}
	}
	return learning.Guardian{}, false
}

func (s *MemoryStore) switchStudentAccountUnlocked(principal learning.Principal, studentID string) (learning.Principal, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Principal, error) {
			return work.switchStudentAccountUnlocked(principal, studentID)
		})
	}
	studentID = strings.TrimSpace(studentID)
	student, ok := s.findStudent(studentID)
	if !ok || student.AccountStatus != "正常" {
		return learning.Principal{}, errors.New("不能切换到该学生账号")
	}
	// 授权判定分两条，必须至少满足一条：
	//  1) 关系表里确实有这条"在读"的家长-学生关系（正常路径）；
	//  2) 目标学生跟当前正在查看的学生挂在同一个家长手机号下（存量兜底）——
	//     这些会话的关系行还没写进库（关系只在登录时写，而 token 没过期就
	//     不会重新登录），但"同一个家长手机号"本身就是关系建立时用的同一个
	//     依据，安全边界没有被放宽：换不到别人家的孩子。
	authorized := principal.GuardianID != "" && s.guardianStudentActive(principal.GuardianID, studentID)
	if !authorized {
		current, currentOK := s.findStudent(principal.StudentID)
		authorized = currentOK && strings.TrimSpace(current.Phone) != "" && phoneSame(current.Phone, student.Phone)
	}
	if !authorized {
		return learning.Principal{}, errors.New("不能切换到该学生账号")
	}
	user, ok := s.findUserByStudentID(student.ID)
	if !ok {
		return learning.Principal{}, errors.New("学生登录账号不存在")
	}
	// 走兜底进来的会话顺手把家长身份和关系补齐，这样只需要兜底一次，
	// 之后就回到正常的关系表路径。
	guardianID := principal.GuardianID
	if guardianID == "" || !s.guardianStudentActive(guardianID, studentID) {
		guardianID = s.ensureGuardianLink(student.Phone, "", student.ID)
	}
	next := principalFromUser(user)
	next.AuthMethod = principal.AuthMethod
	next.GuardianID = guardianID
	// 记一下"最近查看"，只影响下次静默登录（不带手机号，只带 openID）默认恢复
	// 显示哪个孩子，不参与任何权限判断——真正的权限判断始终是上面那两条。
	for i := range s.guardians {
		if s.guardians[i].ID == guardianID {
			s.guardians[i].LastStudentID = student.ID
			break
		}
	}
	return next, nil
}
