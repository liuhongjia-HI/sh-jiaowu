package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestCreateTeacherReturnsRandomOneTimePassword(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	created, err := store.CreateTeacher("超级管理员", principal, learning.TeacherUpsertRequest{
		Name: "新教师", Phone: "13900009999", LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		CanUploadHandout: true, CanUploadQuestion: true, CanReview: true,
	})
	if err != nil {
		t.Fatalf("expected teacher creation to succeed: %v", err)
	}
	if created.TemporaryPassword == "" || created.TemporaryPassword == demoLoginPassword {
		t.Fatalf("expected a random temporary password, got %q", created.TemporaryPassword)
	}
	loggedIn, err := store.LoginWithAdminPassword(created.Phone, created.TemporaryPassword)
	if err != nil || loggedIn.UserID != created.ID || !loggedIn.MustChangePassword {
		t.Fatalf("expected temporary password login requiring password change, user=%#v err=%v", loggedIn, err)
	}
	for _, listed := range store.Teachers(principal) {
		if listed.ID == created.ID && listed.TemporaryPassword != "" {
			t.Fatal("temporary password must only be returned once on creation")
		}
	}
}
