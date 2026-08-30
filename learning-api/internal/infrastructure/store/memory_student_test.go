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

func TestFollowUpOnlyIncludesUnpurchasedMiniProgramStudents(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}

	miniProgramStudent, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "follow-up-mini-program", Phone: "13600008888", StudentName: "待跟进学生", SchoolName: "星河小学", Grade: "五年级",
	})
	if err != nil {
		t.Fatalf("create mini-program student: %v", err)
	}
	if miniProgramStudent.StudentID == "" {
		t.Fatal("expected a student account after mini-program registration")
	}

	manual, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{
		Name: "后台新增学生", Phone: "13600009999", Grade: "五年级", SchoolName: "星河小学", AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("create manual student: %v", err)
	}

	followUps := store.Students(admin, learning.StudentQuery{FollowUpState: "待跟进"})
	if len(followUps) != 1 || followUps[0].ID != miniProgramStudent.StudentID || followUps[0].FollowUpStatus != "待跟进" {
		t.Fatalf("expected only the unpurchased mini-program student to need follow-up, got %#v", followUps)
	}
	if manual.FollowUpStatus != "" {
		t.Fatalf("manual student must not enter follow-up automatically, got %#v", manual)
	}
	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID: miniProgramStudent.StudentID, LearningSpaceIDs: []string{"space-g05-english-s1-q1"}, ContentTypeCodes: []string{"course"},
	}); err != nil {
		t.Fatalf("open direct learning content: %v", err)
	}
	if followUps = store.Students(admin, learning.StudentQuery{FollowUpState: "待跟进"}); len(followUps) != 1 || followUps[0].ID != miniProgramStudent.StudentID {
		t.Fatalf("direct learning access is not a package purchase, got %#v", followUps)
	}

	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{StudentID: miniProgramStudent.StudentID, PackageID: "pkg-g05-english-s1-full"}); err != nil {
		t.Fatalf("open formal package: %v", err)
	}
	if followUps = store.Students(admin, learning.StudentQuery{FollowUpState: "待跟进"}); len(followUps) != 0 {
		t.Fatalf("student with a formal package must leave follow-up, got %#v", followUps)
	}
}
