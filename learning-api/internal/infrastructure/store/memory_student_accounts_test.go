package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestParentPhoneCanSwitchBetweenStudentAccounts(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	first, ok := store.findStudent("stu-001")
	if !ok {
		t.Fatal("expected demo student")
	}
	second, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{
		Name: "小明妹妹", Phone: first.Phone, Grade: store.decorateStudent(first).Grade, AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("expected same guardian phone to be allowed: %v", err)
	}
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	accounts, err := store.StudentAccounts(principal)
	if err != nil || len(accounts) < 2 {
		t.Fatalf("expected sibling accounts, accounts=%#v err=%v", accounts, err)
	}
	next, err := store.SwitchStudentAccount(principal, second.ID)
	if err != nil || next.StudentID != second.ID || next.Name != second.Name {
		t.Fatalf("expected switched principal, principal=%#v err=%v", next, err)
	}
}
