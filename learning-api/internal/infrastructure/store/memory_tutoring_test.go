package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestTutoringAssignmentScopesTeacherStudentsAndKeepsTransferHistory(t *testing.T) {
	store := NewMemoryStore()
	admin := learning.Principal{UserID: "user-super", Roles: []learning.Role{learning.RoleSuperAdmin}, CampusID: "campus-main"}
	firstTeacher := learning.Principal{
		UserID:           "user-teacher",
		Roles:            []learning.Role{learning.RoleTeacher},
		CampusID:         "campus-main",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1", "space-g05-english-s1-q2"},
	}
	store.users = append(store.users, learning.User{
		ID: "user-teacher-002", Name: "英语老师乙", Phone: "13900000004", AccountStatus: "正常",
		Roles: []learning.Role{learning.RoleTeacher}, CampusID: "campus-main",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1", "space-g05-english-s1-q2"}, CanReview: true,
	})
	secondTeacher := learning.Principal{
		UserID:           "user-teacher-002",
		Roles:            []learning.Role{learning.RoleTeacher},
		CampusID:         "campus-main",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1", "space-g05-english-s1-q2"},
	}

	if got := store.Students(firstTeacher, learning.StudentQuery{}); len(got) != 0 {
		t.Fatalf("teacher must not see students before a tutoring assignment, got %#v", got)
	}

	assignment, err := store.CreateTutoringAssignment("教务老师", admin, "stu-001", learning.TutoringAssignmentCreateRequest{
		TeacherID: "user-teacher", SubjectID: "english", LevelCode: "S", StartsAt: "2026-08-30",
	})
	if err != nil {
		t.Fatalf("create tutoring assignment: %v", err)
	}
	if assignment.Status != learning.TutoringAssignmentActive || assignment.TeacherID != firstTeacher.UserID {
		t.Fatalf("unexpected assignment: %#v", assignment)
	}
	if got := store.Students(firstTeacher, learning.StudentQuery{}); len(got) != 1 || got[0].ID != "stu-001" {
		t.Fatalf("teacher should only see assigned student, got %#v", got)
	}

	transferred, err := store.TransferTutoringAssignment("教务老师", admin, assignment.ID, learning.TutoringAssignmentTransferRequest{
		TeacherID: "user-teacher-002", StartsAt: "2026-09-01", Reason: "调整授课安排",
	})
	if err != nil {
		t.Fatalf("transfer tutoring assignment: %v", err)
	}
	if transferred.TeacherID != secondTeacher.UserID || transferred.Status != learning.TutoringAssignmentActive {
		t.Fatalf("unexpected transferred assignment: %#v", transferred)
	}
	if got := store.Students(firstTeacher, learning.StudentQuery{}); len(got) != 0 {
		t.Fatalf("previous teacher must lose current student access, got %#v", got)
	}
	if got := store.Students(secondTeacher, learning.StudentQuery{}); len(got) != 1 || got[0].ID != "stu-001" {
		t.Fatalf("new teacher should receive current student access, got %#v", got)
	}

	history, err := store.StudentTutoringAssignments(admin, "stu-001")
	if err != nil {
		t.Fatalf("read tutoring assignment history: %v", err)
	}
	if len(history) != 2 || history[0].Status != learning.TutoringAssignmentActive || history[1].Status != learning.TutoringAssignmentEnded {
		t.Fatalf("transfer must preserve active and ended history, got %#v", history)
	}
}

func TestStudentsExposeOnlyActiveTutoringAssignments(t *testing.T) {
	store := NewMemoryStore()
	admin := learning.Principal{UserID: "user-super", Roles: []learning.Role{learning.RoleSuperAdmin}, CampusID: "campus-main"}

	assignment, err := store.CreateTutoringAssignment("教务老师", admin, "stu-001", learning.TutoringAssignmentCreateRequest{
		TeacherID: "user-teacher", SubjectID: "english", LevelCode: "S", StartsAt: "2026-08-30",
	})
	if err != nil {
		t.Fatalf("create tutoring assignment: %v", err)
	}
	if _, err := store.EndTutoringAssignment("教务老师", admin, assignment.ID, learning.TutoringAssignmentEndRequest{
		EndsAt: "2026-08-31", Reason: "已结课", Version: assignment.Version,
	}); err != nil {
		t.Fatalf("end tutoring assignment: %v", err)
	}
	if _, err := store.CreateTutoringAssignment("教务老师", admin, "stu-001", learning.TutoringAssignmentCreateRequest{
		TeacherID: "user-teacher", SubjectID: "english", LevelCode: "S", Role: learning.TutoringAssignmentAssistant, StartsAt: "2026-09-01",
	}); err != nil {
		t.Fatalf("create active tutoring assignment: %v", err)
	}

	students := store.Students(admin, learning.StudentQuery{})
	if len(students) == 0 {
		t.Fatal("expected seeded student")
	}
	student := students[0]
	if len(student.ActiveTutoringAssignments) != 1 {
		t.Fatalf("student list should contain exactly one active tutoring assignment, got %#v", student.ActiveTutoringAssignments)
	}
	got := student.ActiveTutoringAssignments[0]
	if got.TeacherID != "user-teacher" || got.TeacherName != "英语老师" || got.SubjectName != "英文" || got.Role != learning.TutoringAssignmentAssistant {
		t.Fatalf("unexpected active tutoring assignment summary: %#v", got)
	}
}

func TestUpdateTeacherRejectsRemovingScopeUsedByActiveAssignment(t *testing.T) {
	store := NewMemoryStore()
	admin := learning.Principal{UserID: "user-super", Roles: []learning.Role{learning.RoleSuperAdmin}, CampusID: "campus-main"}
	if _, err := store.CreateTutoringAssignment("教务老师", admin, "stu-001", learning.TutoringAssignmentCreateRequest{
		TeacherID: "user-teacher", SubjectID: "english", LevelCode: "S", StartsAt: "2026-08-30",
	}); err != nil {
		t.Fatalf("create tutoring assignment: %v", err)
	}

	_, err := store.UpdateTeacher("教务老师", admin, "user-teacher", learning.TeacherUpsertRequest{
		Name: "英语老师", Phone: "13900000003", CampusID: "campus-main",
		LearningSpaceIDs: []string{"space-g05-math-s1-q1"},
		CanUploadHandout: true, CanUploadQuestion: true, CanReview: true, AccountStatus: "正常",
	})
	if err == nil {
		t.Fatal("expected scope update to reject an active tutoring assignment")
	}
}
