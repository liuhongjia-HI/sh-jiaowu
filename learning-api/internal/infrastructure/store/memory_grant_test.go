package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestCreatePackageSupportsGrantPreview(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语专项题包",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		Summary:          "只开放 S1 Q1 英语练习。",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}
	if !containsString(pkg.LearningSpaceIDs, "space-g05-english-s1-q1") {
		t.Fatalf("expected package response to include learning space ids: %#v", pkg.LearningSpaceIDs)
	}
	if !containsString(pkg.ContentTypeCodes, "question") {
		t.Fatalf("expected package response to include content type codes: %#v", pkg.ContentTypeCodes)
	}

	preview, err := store.GrantPreview("stu-001", pkg.ID)
	if err != nil {
		t.Fatalf("expected grant preview for created package: %v", err)
	}
	if len(preview.OpenHomework) == 0 {
		t.Fatalf("expected preview to include opened homework: %#v", preview)
	}
	if len(preview.OpenCourses) != 0 || len(preview.OpenMaterials) != 0 {
		t.Fatalf("question-only package should not open courses/materials: %#v", preview)
	}
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].LearningStatus = "待开通"
		}
	}
	if _, err := store.CreateGrant("运营教务", "stu-001", pkg.ID); err != nil {
		t.Fatalf("expected grant creation to succeed: %v", err)
	}
	student, ok := store.findRawStudent("stu-001")
	if !ok || student.LearningStatus != "已开通" {
		t.Fatalf("grant should update pending student status, got %#v", student)
	}
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].LearningStatus = "待开通"
		}
	}
	student, ok = store.findStudent("stu-001")
	if !ok || student.LearningStatus != "已开通" {
		t.Fatalf("active grant should correct stale pending status in student view, got %#v", student)
	}
}

func TestCreatePackageRejectsCrossAcademicYearSpace(t *testing.T) {
	store := NewMemoryStore()
	store.learningSpaces = append(store.learningSpaces, learningSpace{
		ID:           "space-g05-english-s1-next-year",
		AcademicYear: "2026.2027学年",
		Grade:        "五年级",
		Semester:     "S1",
		Subject:      "英语",
		Phase:        "Q1",
		Name:         "2026.2027学年 五年级 S1 英语 Q1",
		Status:       learning.StatusEnabled,
	})

	if _, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语跨学年套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-next-year"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	}); err == nil || !strings.Contains(err.Error(), "学年") {
		t.Fatalf("expected cross academic year package space to be rejected, got %v", err)
	}
}

func TestUpdatePackageRefreshesExistingGrantAccess(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语可编辑套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}
	if _, err := store.CreateGrant("运营教务", "stu-001", pkg.ID); err != nil {
		t.Fatalf("expected grant creation to succeed: %v", err)
	}

	_, err = store.UpdatePackage("运营教务", pkg.ID, learning.PackageUpsertRequest{
		Name:             "五年级英语可编辑套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "全学期",
		PackageType:      "题+学习资料",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1", "space-g05-english-s1-q2"},
		ContentTypeCodes: []string{"question", "handout"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package update to succeed: %v", err)
	}

	spaceIDs := store.studentAccessibleSpaceIDs("stu-001")
	if !containsString(spaceIDs, "space-g05-english-s1-q2") {
		t.Fatalf("expected existing grant access to refresh after package update, got %#v", spaceIDs)
	}
	materials := store.materialsForStudent("stu-001")
	foundFinalMaterial := false
	for _, material := range materials {
		if material.LearningSpaceID == "space-g05-english-s1-q2" {
			foundFinalMaterial = true
			break
		}
	}
	if !foundFinalMaterial {
		t.Fatalf("expected updated package to open final material, got %#v", materials)
	}
}

func TestGrantPreviewMarksExistingActiveGrant(t *testing.T) {
	store := NewMemoryStore()

	preview, err := store.GrantPreview("stu-001", "pkg-g05-english-s1-full")
	if err != nil {
		t.Fatalf("expected grant preview to succeed: %v", err)
	}
	if !preview.AlreadyOpened {
		t.Fatalf("expected existing active grant to be marked, got %#v", preview)
	}
	if preview.ExistingUntil != "2027-05-22" {
		t.Fatalf("expected existing grant expiry, got %q", preview.ExistingUntil)
	}

	created, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语未开通套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}
	nextPreview, err := store.GrantPreview("stu-001", created.ID)
	if err != nil {
		t.Fatalf("expected new package preview to succeed: %v", err)
	}
	if nextPreview.AlreadyOpened || nextPreview.ExistingUntil != "" {
		t.Fatalf("expected unopened package preview, got %#v", nextPreview)
	}
}

func TestGrantPreviewRejectsPackageOutsideStudentGrade(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "六年级英语错配套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "六年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g06-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}

	if _, err := store.GrantPreview("stu-001", pkg.ID); err == nil || !strings.Contains(err.Error(), "不能给五年级学生开通") {
		t.Fatalf("expected grade mismatch preview error, got %v", err)
	}
	if _, err := store.CreateGrant("运营教务", "stu-001", pkg.ID); err == nil || !strings.Contains(err.Error(), "不能给五年级学生开通") {
		t.Fatalf("expected grade mismatch grant error, got %v", err)
	}
}

func TestGrantPreviewRejectsDisabledPackage(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语停用套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusDisabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}

	if _, err := store.GrantPreview("stu-001", pkg.ID); err == nil || !strings.Contains(err.Error(), "套餐当前未启用") {
		t.Fatalf("expected disabled package preview error, got %v", err)
	}
	if _, err := store.CreateGrant("运营教务", "stu-001", pkg.ID); err == nil || !strings.Contains(err.Error(), "套餐当前未启用") {
		t.Fatalf("expected disabled package grant error, got %v", err)
	}
}

func TestCreateCourseRespectsTeacherScope(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	course, err := store.CreateCourse("英语老师", teacher, learning.CourseUpsertRequest{
		Name:            "五年级英语阅读拓展课",
		LearningSpaceID: "space-g05-english-s1-q1",
		ChapterCount:    6,
		Status:          learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected teacher to create course in own scope: %v", err)
	}
	if course.Subject != "英文" || course.Grade != "五年级" || course.MaterialNum != 0 || course.HomeworkNum != 0 {
		t.Fatalf("unexpected course: %#v", course)
	}

	if _, err := store.CreateCourse("英语老师", teacher, learning.CourseUpsertRequest{
		Name:            "五年级数学拓展课",
		LearningSpaceID: "space-g05-math-s1-q1",
		ChapterCount:    6,
		Status:          learning.StatusEnabled,
	}); err == nil {
		t.Fatal("expected teacher to be blocked from another subject")
	}
}

func TestUpdateCourseSyncsContentReferences(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	updated, err := store.UpdateCourse("超级管理员", admin, "course-g05-english-s1-q1", learning.CourseUpsertRequest{
		Name:            "五年级英语期中阅读精讲课",
		LearningSpaceID: "space-g05-english-s1-q1",
		ChapterCount:    10,
		Status:          learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected course update to succeed: %v", err)
	}
	if updated.ChapterCount != 10 || updated.MaterialNum == 0 || updated.HomeworkNum == 0 {
		t.Fatalf("unexpected updated course counts: %#v", updated)
	}
	for _, material := range store.materials {
		if material.CourseID == updated.ID && material.Course != updated.Name {
			t.Fatalf("expected material course name to sync: %#v", material)
		}
	}
	for _, homework := range store.homework {
		if homework.CourseID == updated.ID && homework.Course != updated.Name {
			t.Fatalf("expected homework course name to sync: %#v", homework)
		}
	}
}
