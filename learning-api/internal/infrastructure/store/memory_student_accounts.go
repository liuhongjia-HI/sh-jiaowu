package store

import (
	"errors"
	"strings"

	"starline/learning-api/internal/domain/learning"
)

// studentAccountsUnlocked 列出当前家长能看到的所有孩子，用来渲染"我的"页的
// 切换器。来源是 guardian_students 关系表，不再靠"同一个手机号"临时撞出来——
// 关系在登录成功的那一刻就已经建好（见 ensureGuardianLink），这里只是读。
func (s *MemoryStore) studentAccountsUnlocked(principal learning.Principal) ([]learning.StudentAccount, error) {
	if principal.GuardianID == "" {
		return nil, errors.New("当前账号未关联家长身份")
	}
	accounts := make([]learning.StudentAccount, 0)
	for _, relation := range s.guardianStudents {
		if relation.GuardianID != principal.GuardianID || relation.Status != learning.GuardianStudentActive {
			continue
		}
		student, ok := s.findStudent(relation.StudentID)
		if !ok || student.AccountStatus != "正常" {
			continue
		}
		decorated := s.decorateStudent(student)
		accounts = append(accounts, learning.StudentAccount{StudentID: student.ID, Name: student.Name, Grade: decorated.Grade, Active: student.ID == principal.StudentID})
	}
	return accounts, nil
}

func (s *MemoryStore) switchStudentAccountUnlocked(principal learning.Principal, studentID string) (learning.Principal, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Principal, error) {
			return work.switchStudentAccountUnlocked(principal, studentID)
		})
	}
	if principal.GuardianID == "" {
		return learning.Principal{}, errors.New("当前账号未关联家长身份")
	}
	studentID = strings.TrimSpace(studentID)
	if !s.guardianStudentActive(principal.GuardianID, studentID) {
		return learning.Principal{}, errors.New("不能切换到该学生账号")
	}
	student, ok := s.findStudent(studentID)
	if !ok || student.AccountStatus != "正常" {
		return learning.Principal{}, errors.New("不能切换到该学生账号")
	}
	user, ok := s.findUserByStudentID(student.ID)
	if !ok {
		return learning.Principal{}, errors.New("学生登录账号不存在")
	}
	next := principalFromUser(user)
	next.AuthMethod = principal.AuthMethod
	next.GuardianID = principal.GuardianID
	// 记一下"最近查看"，只影响下次静默登录（不带手机号，只带 openID）默认恢复
	// 显示哪个孩子，不参与任何权限判断——真正的权限判断始终是 guardianStudentActive。
	for i := range s.guardians {
		if s.guardians[i].ID == principal.GuardianID {
			s.guardians[i].LastStudentID = student.ID
			break
		}
	}
	return next, nil
}
