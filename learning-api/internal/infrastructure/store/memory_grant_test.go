package store

import (
	"strings"
	"testing"
	"time"

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
	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{StudentID: "stu-001", PackageID: pkg.ID}); err != nil {
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
	foundPackageRef := false
	for _, ref := range student.OpenedPackageRefs {
		if ref.PackageID == pkg.ID && ref.PackageName == pkg.Name {
			foundPackageRef = true
			break
		}
	}
	if !foundPackageRef {
		t.Fatalf("student view should expose a stable package link target, got %#v", student.OpenedPackageRefs)
	}
}

func TestCreateDirectGrantWithCourseOpensFullLearningContent(t *testing.T) {
	store := NewMemoryStore()

	result, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        "stu-001",
		LearningSpaceIDs: []string{"space-g05-math-s1-q1"},
		ContentTypeCodes: []string{"course", "question"},
	})
	if err != nil {
		t.Fatalf("expected direct grant to succeed: %v", err)
	}
	if len(result.OpenCourses) == 0 || len(result.OpenMaterials) == 0 || len(result.OpenHomework) == 0 {
		t.Fatalf("course grant should open the complete learning content, got %#v", result)
	}
	if !containsString(result.ContentTypes, "下载讲义") {
		t.Fatalf("course grant should include download permission, got %#v", result.ContentTypes)
	}
	if !containsString(result.LearningSpaces, "五年级数学S1Q1S") {
		t.Fatalf("expected the selected learning space in the result, got %#v", result.LearningSpaces)
	}
	for _, pkg := range store.Packages() {
		if strings.HasPrefix(pkg.ID, "direct-") {
			t.Fatalf("direct grant implementation detail must not appear in course plan list: %#v", pkg)
		}
	}
}

func TestReplaceDirectGrantWithCourseKeepsFullLearningContent(t *testing.T) {
	store := NewMemoryStore()
	studentID := "stu-001"
	spaceID := "space-g05-math-s1-q1"
	startsAt := time.Now().Add(-time.Hour).Format("2006-01-02T15:04")
	endsAt := time.Now().Add(48 * time.Hour).Format("2006-01-02T15:04")
	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID: studentID, LearningSpaceIDs: []string{spaceID}, ContentTypeCodes: []string{"course", "handout", "question"}, StartsAt: startsAt, EndsAt: endsAt,
	}); err != nil {
		t.Fatalf("create direct grant: %v", err)
	}
	grantIndex, ok := store.findGrantIndex(studentID, directGrantPackageID(studentID, spaceID))
	if !ok {
		t.Fatal("expected direct grant after creation")
	}
	initialGrant := store.grants[grantIndex]

	if _, err := store.ReplaceDirectGrant("运营教务", learning.DirectGrantReplaceRequest{
		StudentID:  studentID,
		Selections: []learning.DirectGrantSelection{{LearningSpaceID: spaceID, ContentTypeCodes: []string{"course"}}},
	}); err != nil {
		t.Fatalf("replace direct grant: %v", err)
	}

	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("student principal: %v", err)
	}
	study, err := store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("student study: %v", err)
	}
	if index, ok := store.findGrantIndex(studentID, directGrantPackageID(studentID, spaceID)); !ok || store.grants[index].StartsAt != initialGrant.StartsAt || store.grants[index].EndsAt != initialGrant.EndsAt {
		t.Fatalf("content-only replacement must retain the existing period: %#v", store.grants)
	}
	mathCourse := findCourseByLearningSpace(study.Courses, spaceID)
	if mathCourse == nil || mathCourse.MaterialNum == 0 || mathCourse.HomeworkNum == 0 {
		t.Fatalf("retaining a course grant should keep all course content accessible: %#v", mathCourse)
	}
	hasMaterial := false
	for _, material := range store.materialsForStudent(studentID) {
		if material.LearningSpaceID == spaceID {
			hasMaterial = true
			break
		}
	}
	if !hasMaterial {
		t.Fatal("course grant should retain handout access")
	}
	hasHomework := false
	for _, homework := range store.homeworkForStudent(studentID) {
		if homework.LearningSpaceID == spaceID {
			hasHomework = true
			break
		}
	}
	if !hasHomework {
		t.Fatal("course grant should retain exercise access")
	}
}

func TestStudentStudyCountsFullContentForCourseGrant(t *testing.T) {
	store := NewMemoryStore()
	request := learning.DirectGrantCreateRequest{
		StudentID:        "stu-001",
		LearningSpaceIDs: []string{"space-g05-math-s1-q1"},
		ContentTypeCodes: []string{"course"},
	}
	if _, err := store.CreateDirectGrant("运营教务", request); err != nil {
		t.Fatalf("expected course-only direct grant to succeed: %v", err)
	}

	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	study, err := store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("expected study board: %v", err)
	}
	mathCourse := findCourseByLearningSpace(study.Courses, "space-g05-math-s1-q1")
	if mathCourse == nil {
		t.Fatalf("expected opened math course, got %#v", study.Courses)
	}
	if mathCourse.MaterialNum == 0 || mathCourse.HomeworkNum == 0 {
		t.Fatalf("course card should count the content included with a course grant, got %#v", mathCourse)
	}

	request.ContentTypeCodes = []string{"handout", "question"}
	if _, err := store.CreateDirectGrant("运营教务", request); err != nil {
		t.Fatalf("expected content permissions to merge into the direct grant: %v", err)
	}
	study, err = store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("expected refreshed study board: %v", err)
	}
	mathCourse = findCourseByLearningSpace(study.Courses, "space-g05-math-s1-q1")
	if mathCourse == nil || mathCourse.MaterialNum == 0 || mathCourse.HomeworkNum == 0 {
		t.Fatalf("course card should count newly accessible content, got %#v", mathCourse)
	}
}

func TestCourseGrantEnablesSecureHandoutDownload(t *testing.T) {
	store := NewMemoryStore()
	studentID := "stu-001"
	spaceID := "space-g05-math-s1-q1"

	// Remove the permanent first-lesson preview for this scope so the test
	// proves visibility comes from the remaining handout/question grant.
	filteredHomework := make([]learning.Homework, 0, len(store.homework))
	for _, item := range store.homework {
		if item.LearningSpaceID != spaceID {
			filteredHomework = append(filteredHomework, item)
		}
	}
	store.homework = filteredHomework
	for index := range store.materials {
		if store.materials[index].LearningSpaceID == spaceID {
			store.materials[index].FileID = "file-course-permission"
			store.materials[index].FileName = "lesson.pdf"
		}
	}
	store.settings["downloadPolicy"] = "允许下载带水印PDF"

	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        studentID,
		LearningSpaceIDs: []string{spaceID},
		ContentTypeCodes: []string{"handout"},
	}); err != nil {
		t.Fatalf("expected handout-only direct grant to succeed: %v", err)
	}

	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("student principal: %v", err)
	}
	study, err := store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("student study: %v", err)
	}
	if findCourseByLearningSpace(study.Courses, spaceID) == nil {
		t.Fatalf("course should remain visible when only handouts are opened: %#v", study.Courses)
	}
	mathMaterial := findMaterialByLearningSpace(study.Materials, spaceID)
	if mathMaterial == nil || mathMaterial.DownloadURL != "" {
		t.Fatalf("handout should be previewable but not downloadable without course permission: %#v", study.Materials)
	}

	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        studentID,
		LearningSpaceIDs: []string{spaceID},
		ContentTypeCodes: []string{"course"},
	}); err != nil {
		t.Fatalf("expected course permission to succeed: %v", err)
	}
	study, err = store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("student study after course permission: %v", err)
	}
	mathMaterial = findMaterialByLearningSpace(study.Materials, spaceID)
	if mathMaterial == nil || mathMaterial.DownloadURL == "" {
		t.Fatalf("course grant should enable secure handout download: %#v", study.Materials)
	}

	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        studentID,
		LearningSpaceIDs: []string{spaceID},
		ContentTypeCodes: []string{"download"},
	}); err == nil {
		t.Fatal("download permission must not be granted without a course")
	}
}

func findCourseByLearningSpace(courses []learning.StudentCourseCard, learningSpaceID string) *learning.StudentCourseCard {
	for index := range courses {
		if courses[index].LearningSpaceID == learningSpaceID {
			return &courses[index]
		}
	}
	return nil
}

func findMaterialByLearningSpace(materials []learning.Material, learningSpaceID string) *learning.Material {
	for index := range materials {
		if materials[index].LearningSpaceID == learningSpaceID {
			return &materials[index]
		}
	}
	return nil
}

func TestContentPermissionsDoNotLeakIntoOtherContentTypes(t *testing.T) {
	store := NewMemoryStore()
	studentID := "stu-001"
	spaceID := "space-g05-math-s1-q1"
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("student principal: %v", err)
	}
	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID: studentID, LearningSpaceIDs: []string{spaceID}, ContentTypeCodes: []string{"question"},
	}); err != nil {
		t.Fatalf("question-only grant: %v", err)
	}
	study, err := store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("student study: %v", err)
	}
	mathCourse := findCourseByLearningSpace(study.Courses, spaceID)
	if mathCourse == nil || mathCourse.MaterialNum != 0 || mathCourse.HomeworkNum == 0 {
		t.Fatalf("question-only grant should keep course and questions, not handouts: %#v", mathCourse)
	}
}

func TestDirectGrantSupportsTimedPeriodAndHighlightsNewCourse(t *testing.T) {
	store := NewMemoryStore()
	startsAt := time.Now().Add(-5 * time.Minute).Format("2006-01-02T15:04")
	endsAt := time.Now().Add(time.Hour).Format("2006-01-02T15:04")

	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        "stu-001",
		LearningSpaceIDs: []string{"space-g05-math-s1-q1"},
		ContentTypeCodes: []string{"course"},
		StartsAt:         startsAt,
		EndsAt:           endsAt,
	}); err != nil {
		t.Fatalf("expected timed direct grant to succeed: %v", err)
	}

	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	study, err := store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("expected study board: %v", err)
	}
	if len(study.Courses) == 0 || study.Courses[0].Subject != "数学" {
		t.Fatalf("newly opened course should be first, got %#v", study.Courses)
	}
	if !study.Courses[0].IsNew || study.Courses[0].OpenedAt == "" || study.Courses[0].HighlightUntil == "" {
		t.Fatalf("newly opened course should expose its one-hour highlight, got %#v", study.Courses[0])
	}
}

func TestDirectGrantKeepsFutureCourseHiddenUntilItsStartTime(t *testing.T) {
	store := NewMemoryStore()
	startsAt := time.Now().Add(10 * time.Minute).Format("2006-01-02T15:04")
	endsAt := time.Now().Add(2 * time.Hour).Format("2006-01-02T15:04")

	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        "stu-001",
		LearningSpaceIDs: []string{"space-g05-math-s1-q1"},
		ContentTypeCodes: []string{"course"},
		StartsAt:         startsAt,
		EndsAt:           endsAt,
	}); err != nil {
		t.Fatalf("expected future direct grant to be accepted: %v", err)
	}

	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	study, err := store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("expected study board: %v", err)
	}
	for _, course := range study.Courses {
		if course.Subject == "数学" {
			t.Fatalf("future course must stay hidden before start time, got %#v", study.Courses)
		}
	}
}

func TestDirectGrantMarksCourseAsNewWhenItIsAddedToAnExistingContentGrant(t *testing.T) {
	store := NewMemoryStore()
	startsAt := time.Now().Add(-2 * time.Hour).Format("2006-01-02T15:04")
	endsAt := time.Now().Add(time.Hour).Format("2006-01-02T15:04")
	request := learning.DirectGrantCreateRequest{
		StudentID:        "stu-001",
		LearningSpaceIDs: []string{"space-g05-math-s1-q1"},
		StartsAt:         startsAt,
		EndsAt:           endsAt,
	}
	request.ContentTypeCodes = []string{"question"}
	if _, err := store.CreateDirectGrant("运营教务", request); err != nil {
		t.Fatalf("expected initial content grant: %v", err)
	}
	for index := range store.grants {
		if store.grants[index].StudentID == "stu-001" && strings.HasPrefix(store.grants[index].PackageID, "direct-") {
			store.grants[index].OpenedAt = time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
		}
	}
	request.ContentTypeCodes = []string{"course"}
	if _, err := store.CreateDirectGrant("运营教务", request); err != nil {
		t.Fatalf("expected course to be added to existing grant: %v", err)
	}

	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	study, err := store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("expected study board: %v", err)
	}
	if len(study.Courses) == 0 || study.Courses[0].Subject != "数学" || !study.Courses[0].IsNew {
		t.Fatalf("course added to an existing grant should be newly highlighted, got %#v", study.Courses)
	}
}

func TestDirectGrantIsNotShownAsAnotherStudentsRecommendation(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        "stu-001",
		LearningSpaceIDs: []string{"space-g05-chinese-s1-q1"},
		ContentTypeCodes: []string{"course"},
	}); err != nil {
		t.Fatalf("expected direct grant to succeed: %v", err)
	}

	student, err := store.PrincipalByUserID("user-student-003")
	if err != nil {
		t.Fatalf("expected second student principal: %v", err)
	}
	recommendations, err := store.StudentRecommendations(student)
	if err != nil {
		t.Fatalf("expected recommendations: %v", err)
	}
	for _, item := range recommendations {
		if strings.HasPrefix(item.PackageID, "direct-") {
			t.Fatalf("direct grant must not be exposed as a recommendation: %#v", item)
		}
	}
}

// 学习空间是跨学年复用的课程目录，不参与学年匹配：套餐的学年可以和它绑定的
// 学习空间上标注的学年不一致，这是有意为之，不是缺陷。学年只属于套餐本身
// （见 memory.go 里 packageFromRequest 和 learningSpaceMatches 的注释）。
func TestCreatePackageAcademicYearIsIndependentFromLearningSpaceYear(t *testing.T) {
	store := NewMemoryStore()
	store.learningSpaces = append(store.learningSpaces, learningSpace{
		ID:           "space-g05-english-s1-other-year-label",
		AcademicYear: "2026.2027学年",
		Grade:        "五年级",
		Semester:     "S1",
		Subject:      "英语",
		Phase:        "Q1",
		Name:         "2026.2027学年 五年级 S1 英语 Q1",
		Status:       learning.StatusEnabled,
	})

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-other-year-label"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed regardless of the space's year label: %v", err)
	}
	if pkg.AcademicYear != "2025.2026学年" {
		t.Fatalf("package academic year should stay as requested, got %q", pkg.AcademicYear)
	}
}

// 学习空间匹配仍然要求年级/学科/学期一致——只是不再比较学年。
func TestCreatePackageStillRejectsMismatchedGradeSpace(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "年级不匹配套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "六年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	}); err == nil || !strings.Contains(err.Error(), "一致") {
		t.Fatalf("expected grade-mismatched package space to be rejected, got %v", err)
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
	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{StudentID: "stu-001", PackageID: pkg.ID}); err != nil {
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

func TestFutureGrantDoesNotOpenStudentContentBeforeStart(t *testing.T) {
	store := NewMemoryStore()
	startsAt := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	endsAt := time.Now().AddDate(0, 1, 1).Format("2006-01-02")
	packageID := packageID(4, "地理", 0, "full")

	preview, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{
		StudentID: "stu-001",
		PackageID: packageID,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
	})
	if err != nil {
		t.Fatalf("expected future grant to be created: %v", err)
	}
	if preview.ExistingStartsAt != startsAt || preview.ExistingUntil != endsAt {
		t.Fatalf("expected preview to expose selected grant period, got %#v", preview)
	}

	for _, course := range store.coursesForStudent("stu-001") {
		if course.Subject == "地理" {
			t.Fatalf("future grant should not expose geography course before start: %#v", course)
		}
	}
	permission := store.permissionForStudent(learning.Student{ID: "stu-001", Name: "小明", Grade: "五年级", AccountStatus: "正常"})
	if permission.PermissionState != "生效中" {
		t.Fatalf("existing active grant should keep overall state active, got %#v", permission)
	}

	student, ok := store.findStudent("stu-001")
	if !ok {
		t.Fatal("expected demo student")
	}
	store.grants = []packageGrant{{
		ID: "future-only", StudentID: student.ID, PackageID: packageID, StartsAt: startsAt, EndsAt: endsAt, Status: "active",
	}}
	store.spaceAccess = nil
	store.syncSpaceAccessForGrant(store.grants[0])
	permission = store.permissionForStudent(student)
	if permission.PermissionState != "未开始" || len(permission.OpenCourses) != 0 {
		t.Fatalf("future-only grant should be visible as not started without open content, got %#v", permission)
	}
}

func TestCreateGrantUpdatesExistingGrantPeriod(t *testing.T) {
	store := NewMemoryStore()
	packageID := packageID(4, "地理", 0, "full")
	initialStart := time.Now().Format("2006-01-02")
	initialEnd := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	updatedStart := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	updatedEnd := time.Now().AddDate(0, 2, 0).Format("2006-01-02")

	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{
		StudentID: "stu-001",
		PackageID: packageID,
		StartsAt:  initialStart,
		EndsAt:    initialEnd,
	}); err != nil {
		t.Fatalf("expected initial grant to be created: %v", err)
	}
	preview, err := store.GrantPreview("stu-001", packageID)
	if err != nil {
		t.Fatalf("expected preview for existing grant: %v", err)
	}
	if !preview.AlreadyOpened || preview.ExistingStartsAt != initialStart || preview.ExistingUntil != initialEnd {
		t.Fatalf("expected preview to show existing grant period, got %#v", preview)
	}

	updated, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{
		StudentID: "stu-001",
		PackageID: packageID,
		StartsAt:  updatedStart,
		EndsAt:    updatedEnd,
	})
	if err != nil {
		t.Fatalf("expected existing grant period to update: %v", err)
	}
	if !updated.AlreadyOpened || updated.ExistingStartsAt != updatedStart || updated.ExistingUntil != updatedEnd {
		t.Fatalf("expected updated grant period in response, got %#v", updated)
	}
	if _, startsAt, endsAt := store.grantState("stu-001", packageID); startsAt != updatedStart || endsAt != updatedEnd {
		t.Fatalf("expected stored grant period to update, got %s - %s", startsAt, endsAt)
	}
	for _, access := range store.spaceAccess {
		if access.StudentID == "stu-001" && access.StartsAt == initialStart && access.EndsAt == initialEnd {
			t.Fatalf("expected old learning space access period to be replaced, got %#v", access)
		}
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
	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{StudentID: "stu-001", PackageID: pkg.ID}); err == nil || !strings.Contains(err.Error(), "不能给五年级学生开通") {
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
	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{StudentID: "stu-001", PackageID: pkg.ID}); err == nil || !strings.Contains(err.Error(), "套餐当前未启用") {
		t.Fatalf("expected disabled package grant error, got %v", err)
	}
}

func TestCreateGrantRejectsDisabledStudent(t *testing.T) {
	store := NewMemoryStore()
	store.students[0].AccountStatus = "停用"
	packageID := packageID(4, "英文", 0, "full")

	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{
		StudentID: "stu-001",
		PackageID: packageID,
	}); err == nil || !strings.Contains(err.Error(), "学生账号已停用") {
		t.Fatalf("expected disabled student to be rejected, got %v", err)
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
		Curriculum:      testCurriculum("reading-extension"),
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
		Curriculum:      testCurriculum("math-extension"),
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
		Curriculum:      testCurriculum("course-g05-english-s1-q1"),
		Status:          learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected course update to succeed: %v", err)
	}
	if updated.LessonCount != 1 || updated.MaterialNum == 0 || updated.HomeworkNum == 0 {
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

func TestPackageLevelIsDerivedAndCannotMixLearningSpaceLevels(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
	hSpaceID := learningSpaceIDForLevel(6, "数学", 0, 0, "H")
	sSpaceID := learningSpaceIDForLevel(6, "数学", 0, 0, "S")

	pkg, err := store.packageFromRequest("pkg-level-h", learning.PackageUpsertRequest{
		Name:             "七年级数学 H 套餐",
		AcademicYear:     "2026.2027学年",
		Grade:            "七年级",
		Semester:         "S1",
		Subject:          "数学",
		LearningSpaceIDs: []string{hSpaceID},
		ContentTypeCodes: []string{"course"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected level to be derived from selected space: %v", err)
	}
	if pkg.Level != "H" {
		t.Fatalf("expected derived level H, got %q", pkg.Level)
	}

	_, err = store.packageFromRequest("pkg-level-mixed", learning.PackageUpsertRequest{
		Name:             "七年级数学混合等级套餐",
		AcademicYear:     "2026.2027学年",
		Grade:            "七年级",
		Semester:         "S1",
		Subject:          "数学",
		Level:            "H",
		LearningSpaceIDs: []string{hSpaceID, sSpaceID},
		ContentTypeCodes: []string{"course"},
		Status:           learning.StatusEnabled,
	})
	if err == nil || !strings.Contains(err.Error(), "等级") {
		t.Fatalf("expected mixed levels to be rejected, got %v", err)
	}
}
