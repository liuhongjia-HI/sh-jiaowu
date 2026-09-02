package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestCleanupTestStudentsRemovesOnlyMarkedStudents(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false, SkipBaseData: true})
	admin := learning.Principal{UserID: "admin", CampusID: "campus-main", Roles: []learning.Role{learning.RoleSuperAdmin}}
	testStudent, err := store.CreateStudent("管理员", admin, learning.StudentUpsertRequest{Name: "接口测试学生", Phone: "13900000001", Grade: "五年级", Remark: "接口测试"})
	if err != nil {
		t.Fatal(err)
	}
	prodStudent, err := store.CreateStudent("管理员", admin, learning.StudentUpsertRequest{Name: "正式学生", Phone: "13900000002", Grade: "五年级"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.CleanupTestStudents("管理员", admin)
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedCount != 1 || len(result.DeletedIDs) != 1 || result.DeletedIDs[0] != testStudent.ID {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	if _, ok := store.findStudent(testStudent.ID); ok {
		t.Fatal("test student should be removed")
	}
	if _, ok := store.findStudent(prodStudent.ID); !ok {
		t.Fatal("formal student should be kept")
	}
	if _, ok := store.findUserByStudentID(testStudent.ID); ok {
		t.Fatal("test student user should be removed")
	}
}

func TestBatchDeleteStudentsRemovesSelectedStudentsAndReferences(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false, SkipBaseData: true})
	admin := learning.Principal{UserID: "admin", CampusID: "campus-main", Roles: []learning.Role{learning.RoleSuperAdmin}}
	first, err := store.CreateStudent("管理员", admin, learning.StudentUpsertRequest{Name: "批量删除学生一", Phone: "13900000011", Grade: "五年级"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateStudent("管理员", admin, learning.StudentUpsertRequest{Name: "批量删除学生二", Phone: "13900000012", Grade: "五年级"})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := store.CreateStudent("管理员", admin, learning.StudentUpsertRequest{Name: "保留学生", Phone: "13900000013", Grade: "五年级"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.BatchDeleteStudents("管理员", admin, []string{first.ID, second.ID, first.ID})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedCount != 2 || len(result.DeletedIDs) != 2 {
		t.Fatalf("unexpected batch delete result: %#v", result)
	}
	for _, id := range []string{first.ID, second.ID} {
		if _, ok := store.findStudent(id); ok {
			t.Fatalf("student %s should be removed", id)
		}
		if _, ok := store.findUserByStudentID(id); ok {
			t.Fatalf("student user %s should be removed", id)
		}
	}
	if _, ok := store.findStudent(kept.ID); !ok {
		t.Fatal("unselected student should be kept")
	}
}

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

func TestCompletedReviewNoticeLinksToStudentSubmission(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}

	submission, err := store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          90,
		TeacherComment: "阅读依据找得很准，继续保持。",
	})
	if err != nil {
		t.Fatalf("complete review: %v", err)
	}
	home, err := store.StudentHome(student)
	if err != nil {
		t.Fatalf("load student home: %v", err)
	}
	for _, notice := range home.Notices {
		if notice.RelatedType == "review" {
			if notice.RelatedID != submission.ID {
				t.Fatalf("review notice should link to submission %q, got %#v", submission.ID, notice)
			}
			return
		}
	}
	t.Fatal("expected a review notice after completing the review")
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

func TestFollowUpIncludesEveryStudentWithoutAnOpenedPackage(t *testing.T) {
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
	miniProgramProfile, ok := store.findStudent(miniProgramStudent.StudentID)
	if !ok {
		t.Fatal("expected mini-program student profile")
	}

	manual, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{
		Name: "后台新增学生", Phone: "13600009999", Grade: "五年级", SchoolName: "星河小学", AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("create manual student: %v", err)
	}

	followUps := store.Students(admin, learning.StudentQuery{FollowUpState: "待跟进"})
	if len(followUps) != 2 {
		t.Fatalf("expected every student without a package to need follow-up, got %#v", followUps)
	}
	if miniProgramProfile.FollowUpStatus != "待跟进" || manual.FollowUpStatus != "待跟进" {
		t.Fatalf("students without a package must be marked for follow-up, got mini-program=%#v manual=%#v", miniProgramProfile, manual)
	}
	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID: miniProgramStudent.StudentID, LearningSpaceIDs: []string{"space-g05-english-s1-q1"}, ContentTypeCodes: []string{"course"},
	}); err != nil {
		t.Fatalf("open direct learning content: %v", err)
	}
	if followUps = store.Students(admin, learning.StudentQuery{FollowUpState: "待跟进"}); len(followUps) != 1 || followUps[0].ID != manual.ID {
		t.Fatalf("students with direct learning access must leave follow-up, got %#v", followUps)
	}

	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{StudentID: miniProgramStudent.StudentID, PackageID: "pkg-g05-english-s1-full"}); err != nil {
		t.Fatalf("open formal package: %v", err)
	}
	if followUps = store.Students(admin, learning.StudentQuery{FollowUpState: "待跟进"}); len(followUps) != 1 || followUps[0].ID != manual.ID {
		t.Fatalf("only the still-unopened student must remain for follow-up, got %#v", followUps)
	}
}

func TestStudentDetailOpeningMatrixSeparatesDirectAndPackageSources(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	studentID := "stu-001"
	spaceID := "space-g05-math-s1-q1"

	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID: studentID, LearningSpaceIDs: []string{spaceID}, ContentTypeCodes: []string{"course"},
	}); err != nil {
		t.Fatalf("create direct grant: %v", err)
	}
	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name: "五年级数学讲义包", AcademicYear: "2026.2027学年", Grade: "五年级", Semester: "S1", Subject: "数学",
		LearningSpaceIDs: []string{spaceID}, ContentTypeCodes: []string{"course", "handout"}, Status: learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{StudentID: studentID, PackageID: pkg.ID}); err != nil {
		t.Fatalf("create package grant: %v", err)
	}

	detail, err := store.StudentDetail(admin, studentID)
	if err != nil {
		t.Fatalf("student detail: %v", err)
	}
	var courseCell *learning.StudentOpeningCell
	for index := range detail.OpeningMatrix {
		if detail.OpeningMatrix[index].LearningSpaceID != spaceID {
			continue
		}
		for cellIndex := range detail.OpeningMatrix[index].Content {
			cell := &detail.OpeningMatrix[index].Content[cellIndex]
			if cell.ContentTypeCode == "course" {
				courseCell = cell
			}
		}
	}
	if courseCell == nil {
		t.Fatalf("expected course cell for %s, got %#v", spaceID, detail.OpeningMatrix)
	}
	if !courseCell.Opened || !courseCell.DirectOpened || !courseCell.PackageOpened {
		t.Fatalf("expected effective course access with both sources, got %#v", courseCell)
	}
	if !containsString(courseCell.PackageNames, pkg.Name) || len(courseCell.Items) == 0 {
		t.Fatalf("expected source package and opened course details, got %#v", courseCell)
	}
}

func TestStudentsAreSortedByRegistrationTimeDescending(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}

	store.students = []learning.Student{
		{ID: "student-oldest", Name: "最早注册", CreatedAt: "2026-08-01 09:00:00"},
		{ID: "student-middle", Name: "中间注册", CreatedAt: "2026-08-15 09:00:00"},
		{ID: "student-newest", Name: "最近注册", CreatedAt: "2026-08-31 09:00:00"},
	}

	students := store.Students(admin, learning.StudentQuery{})
	if len(students) != 3 {
		t.Fatalf("expected 3 students, got %#v", students)
	}
	if students[0].ID != "student-newest" || students[1].ID != "student-middle" || students[2].ID != "student-oldest" {
		t.Fatalf("expected students sorted by registration time descending, got %#v", students)
	}
}
