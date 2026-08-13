package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestStudentAverageScoreIsDerivedFromGradedSubmissions(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}

	before, ok := store.findStudent("stu-001")
	if !ok {
		t.Fatal("expected seeded student stu-001")
	}
	if before.AverageScore != 0 {
		t.Fatalf("expected average score to start at 0 with no graded submissions, got %d", before.AverageScore)
	}

	if _, err := store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          90,
		TeacherComment: "阅读依据找得很准，继续保持。",
	}); err != nil {
		t.Fatalf("expected review completion to succeed: %v", err)
	}
	afterOne, ok := store.findStudent("stu-001")
	if !ok {
		t.Fatal("expected student stu-001 after grading")
	}
	if afterOne.AverageScore != 90 {
		t.Fatalf("expected average score 90 after a single graded submission, got %d", afterOne.AverageScore)
	}

	// 待批改（"待复核"）不计入平均分。
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	if _, err := store.CreateSubmission("小明", student, learning.SubmissionRequest{
		HomeworkID: "hw-g05-english-s1-q1",
		Answers:    []learning.SubmissionAnswer{{QuestionID: "q1", Text: "another attempt"}},
	}); err != nil {
		t.Fatalf("expected second submission to succeed: %v", err)
	}
	afterPending, ok := store.findStudent("stu-001")
	if !ok {
		t.Fatal("expected student stu-001 after pending submission")
	}
	if afterPending.AverageScore != 90 {
		t.Fatalf("pending submission must not affect average score, got %d", afterPending.AverageScore)
	}
}

// TestUpdateStudentRebasesEnrollmentWhenAdminCorrectsGrade 确认管理端修改年级
// 是一次订正：以订正时点为新的入学基准，而不是直接写死一个以后再也不会滚动的值。
func TestUpdateStudentRebasesEnrollmentWhenAdminCorrectsGrade(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}

	updated, err := store.UpdateStudent("超级管理员", admin, "stu-001", learning.StudentUpsertRequest{
		Name: "小明", Phone: "18500009069", Grade: "六年级", SchoolName: "星河小学", AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("expected grade correction to succeed: %v", err)
	}
	if updated.Grade != "六年级" {
		t.Fatalf("expected corrected grade 六年级, got %q", updated.Grade)
	}
	raw, ok := store.findRawStudent("stu-001")
	if !ok {
		t.Fatal("expected stu-001 to exist")
	}
	if raw.EnrollmentGrade != "六年级" || raw.EnrollmentAcademicYear != currentAcademicYear() {
		t.Fatalf("expected enrollment basis rebased to now, got %#v", raw)
	}
}
