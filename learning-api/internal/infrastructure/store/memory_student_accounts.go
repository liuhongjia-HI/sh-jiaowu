package store

import (
	"errors"
	"strings"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) studentAccountsUnlocked(principal learning.Principal) ([]learning.StudentAccount, error) {
	if principal.StudentID == "" || strings.TrimSpace(principal.Phone) == "" {
		return nil, errors.New("当前学生账号未绑定家长手机号")
	}
	accounts := make([]learning.StudentAccount, 0)
	for _, student := range s.students {
		if !phoneSame(student.Phone, principal.Phone) || student.AccountStatus != "正常" {
			continue
		}
		decorated := s.decorateStudent(student)
		accounts = append(accounts, learning.StudentAccount{StudentID: student.ID, Name: student.Name, Grade: decorated.Grade, Active: student.ID == principal.StudentID})
	}
	return accounts, nil
}

func (s *MemoryStore) switchStudentAccountUnlocked(principal learning.Principal, studentID string) (learning.Principal, error) {
	if principal.StudentID == "" || strings.TrimSpace(principal.Phone) == "" {
		return learning.Principal{}, errors.New("当前学生账号未绑定家长手机号")
	}
	student, ok := s.findStudent(strings.TrimSpace(studentID))
	if !ok || student.AccountStatus != "正常" || !phoneSame(student.Phone, principal.Phone) {
		return learning.Principal{}, errors.New("不能切换到该学生账号")
	}
	user, ok := s.findUserByStudentID(student.ID)
	if !ok {
		return learning.Principal{}, errors.New("学生登录账号不存在")
	}
	next := principalFromUser(user)
	next.AuthMethod = principal.AuthMethod
	return next, nil
}
