package store

import (
	"os"
	"reflect"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func gradeSubjectForTest(t *testing.T, s *MemoryStore, grade, subject string) learning.GradeSubjectMetadata {
	t.Helper()
	for _, item := range s.GradeSubjects() {
		if item.Grade == grade && subjectsMatch(item.Subject, subject) {
			return item
		}
	}
	t.Fatalf("missing grade subject %s/%s", grade, subject)
	return learning.GradeSubjectMetadata{}
}

func TestGradeSubjectLevelsLifecycle(t *testing.T) {
	s := NewMemoryStore()
	admin, err := s.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	original := gradeSubjectForTest(t, s, "五年级", "科学")
	unrelated := gradeSubjectForTest(t, s, "六年级", "科学")
	if !reflect.DeepEqual(original.Levels, []string{"S"}) {
		t.Fatal(original.Levels)
	}
	for _, levels := range [][]string{nil, {}, {"A"}, {"S", "S"}} {
		if _, err := s.UpdateGradeSubjectLevels("管理员", original.ID, learning.GradeSubjectLevelsUpdateRequest{Levels: levels}); err == nil {
			t.Fatalf("accepted invalid levels %v", levels)
		}
	}
	if _, err := s.UpdateGradeSubjectLevels("管理员", "missing", learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S"}}); err == nil {
		t.Fatal("accepted missing scope")
	}
	if _, err := s.UpdateGradeSubjectLevels("管理员", original.ID, learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S+", "S"}}); err != nil {
		t.Fatal(err)
	}
	for _, semester := range []string{"s1", "s2"} {
		for _, phase := range []string{"q1", "q2"} {
			space, ok := s.findLearningSpace("space-g05-science-" + semester + "-" + phase + "-splus")
			if !ok || !s.learningSpaceSelectable(space) {
				t.Fatalf("new scope missing: %s/%s", semester, phase)
			}
		}
	}
	spaceID := "space-g05-science-s1-q1-splus"
	courseReq := learning.CourseUpsertRequest{Name: "科学 S+", LearningSpaceID: spaceID, Curriculum: familyTestCurriculum("science-level"), Status: learning.StatusEnabled}
	course, err := s.CreateCourse("管理员", admin, courseReq)
	if err != nil {
		t.Fatal(err)
	}
	packageReq := learning.PackageUpsertRequest{Name: "科学 S+ 方案", Grade: "五年级", Subject: "科学", Semester: "S1", LearningSpaceIDs: []string{spaceID}, ContentTypeCodes: []string{"handout"}, Status: learning.StatusEnabled}
	pkg, err := s.CreatePackage("管理员", packageReq)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateGrant("管理员", learning.GrantCreateRequest{StudentID: "stu-001", PackageID: pkg.ID}); err != nil {
		t.Fatal(err)
	}
	if !s.studentHasActiveContentGrantForLearningSpace("stu-001", spaceID, "handout") {
		t.Fatal("newly configured level did not grant access")
	}
	family, err := s.CreateCourseFamily("管理员", admin, learning.CourseFamilyCreateRequest{Name: "科学共享目录", LearningSpaceIDs: []string{spaceID}, Curriculum: familyTestCurriculum("science-family")})
	if err != nil {
		t.Fatal(err)
	}
	grantsBefore := append([]packageGrant(nil), s.grants...)
	accessBefore := append([]learningSpaceAccess(nil), s.spaceAccess...)
	if _, err := s.UpdateGradeSubjectLevels("管理员", original.ID, learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S"}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(grantsBefore, s.grants) || !reflect.DeepEqual(accessBefore, s.spaceAccess) {
		t.Fatal("changed historical access")
	}
	if !s.studentHasActiveContentGrantForLearningSpace("stu-001", spaceID, "handout") {
		t.Fatal("cancelled level revoked existing student access")
	}
	if len(s.courses) == 0 {
		t.Fatal("removed courses")
	}
	for _, space := range s.LearningSpaces() {
		if space.ID == spaceID && (space.Selectable || space.Status != learning.StatusEnabled) {
			t.Fatal("historical space must remain enabled but not selectable")
		}
	}
	if _, err := s.UpdateCourseFamily("管理员", admin, family.ID, learning.CourseFamilyUpdateRequest{Name: "编辑历史共享目录", Curriculum: family.Curriculum}); err != nil {
		t.Fatalf("historical family edit: %v", err)
	}
	courseReq.Name = "编辑已有科学课程"
	if _, err := s.UpdateCourse("管理员", admin, course.ID, courseReq); err != nil {
		t.Fatalf("historical course edit: %v", err)
	}
	packageReq.Name = "编辑已有科学方案"
	if _, err := s.UpdatePackage("管理员", pkg.ID, packageReq); err != nil {
		t.Fatalf("historical package edit: %v", err)
	}
	if _, err := s.CreateCourse("管理员", admin, courseReq); err == nil {
		t.Fatal("created cancelled level course")
	}
	if _, err := s.CreatePackage("管理员", packageReq); err == nil {
		t.Fatal("created cancelled level package")
	}
	if _, err := s.CopyCourse("管理员", admin, course.ID, learning.CourseCopyRequest{LearningSpaceID: spaceID}); err == nil {
		t.Fatal("copied cancelled course")
	}
	if _, err := s.CopyPackage("管理员", pkg.ID, learning.PackageCopyRequest{}); err == nil {
		t.Fatal("copied cancelled package")
	}
	if _, err := s.CreateCourseFamily("管理员", admin, learning.CourseFamilyCreateRequest{Name: "已取消班型系列", LearningSpaceIDs: []string{spaceID}, Curriculum: courseReq.Curriculum}); err == nil {
		t.Fatal("created cancelled family")
	}
	// An old open metadata drawer must not overwrite the latest levels.
	original.Levels = []string{"S", "S+"} // A stale drawer still carries the removed level.
	original.Summary = "保留新班型配置"
	if _, err := s.UpsertGradeSubject("管理员", original.ID, original); err != nil {
		t.Fatal(err)
	}
	if got := gradeSubjectForTest(t, s, "五年级", "科学"); !reflect.DeepEqual(got.Levels, []string{"S"}) || got.Summary != original.Summary {
		t.Fatal(got)
	}
	if got := gradeSubjectForTest(t, s, "六年级", "科学"); !reflect.DeepEqual(got, unrelated) {
		t.Fatal("changed unrelated scope", got)
	}
	s.ensureLearningSpaces(currentAcademicYear())
	if space, _ := s.findLearningSpace(spaceID); s.learningSpaceSelectable(space) {
		t.Fatal("initialization reopened cancelled level")
	}
	if _, err := s.UpdateGradeSubjectLevels("管理员", original.ID, learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S", "S+"}}); err != nil {
		t.Fatal(err)
	}
	if space, _ := s.findLearningSpace(spaceID); !s.learningSpaceSelectable(space) {
		t.Fatal("could not reopen level")
	}
}

func TestGradeSubjectLevelsMySQLRestartAndRollback(t *testing.T) {
	dsn := os.Getenv("STARLINE_LEVELS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated levels test database required")
	}
	s := NewMemoryStoreWithOptions(Options{SeedDemoData: false, BootstrapAdminPhone: "13800000001", BootstrapAdminPassword: "123456"})
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	item := gradeSubjectForTest(t, s, "五年级", "科学")
	if _, err := s.UpdateGradeSubjectLevels("管理员", item.ID, learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"H+"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	restored := NewMemoryStoreWithOptions(Options{SeedDemoData: false, BootstrapAdminPhone: "13800000001", BootstrapAdminPassword: "123456"})
	if err := restored.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	if got := gradeSubjectForTest(t, restored, "五年级", "科学"); !reflect.DeepEqual(got.Levels, []string{"H+"}) {
		t.Fatal(got)
	}
	if base, ok := restored.findLearningSpace("space-g05-science-s1-q1"); !ok || restored.learningSpaceSelectable(base) || base.Status != learning.StatusEnabled {
		t.Fatal("startup reopened cancelled base level")
	}
	newID := "space-g05-science-s2-q2-hplus"
	if space, ok := restored.findLearningSpace(newID); !ok || !restored.learningSpaceSelectable(space) {
		t.Fatal("new space missing after restart")
	}
	if _, err := restored.UpdateGradeSubjectLevels("管理员", item.ID, learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S"}}); err != nil {
		t.Fatal(err)
	}
	restored.db.Close()
	again := NewMemoryStoreWithOptions(Options{SeedDemoData: false, BootstrapAdminPhone: "13800000001", BootstrapAdminPassword: "123456"})
	if err := again.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	if space, ok := again.findLearningSpace(newID); !ok || again.learningSpaceSelectable(space) {
		t.Fatal("restart lost cancellation or historical space")
	}
	before := again.settings[gradeSubjectCatalogSetting]
	beforeSpaces := append([]learningSpace(nil), again.learningSpaces...)
	if _, err := again.db.Exec(`CREATE TRIGGER levels_validation_reject BEFORE INSERT ON learning_spaces FOR EACH ROW BEGIN IF NEW.id = 'space-g05-science-s1-q1-h' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'levels validation failure'; END IF; END`); err != nil {
		t.Fatal(err)
	}
	if _, err := again.UpdateGradeSubjectLevels("管理员", item.ID, learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S", "H"}}); err == nil {
		t.Fatal("expected transaction rollback")
	}
	if _, err := again.db.Exec(`DROP TRIGGER levels_validation_reject`); err != nil {
		t.Fatal(err)
	}
	var persisted string
	if err := again.db.QueryRow(`SELECT setting_value FROM system_settings WHERE setting_key = ?`, gradeSubjectCatalogSetting).Scan(&persisted); err != nil || persisted != before {
		t.Fatal("transaction did not roll back settings", err)
	}
	var inserted int
	if err := again.db.QueryRow(`SELECT COUNT(*) FROM learning_spaces WHERE id = 'space-g05-science-s1-q1-h'`).Scan(&inserted); err != nil || inserted != 0 {
		t.Fatal("transaction did not roll back spaces", err)
	}
	if again.settings[gradeSubjectCatalogSetting] != before || !reflect.DeepEqual(beforeSpaces, again.learningSpaces) {
		t.Fatal("transaction failure leaked in memory")
	}
	again.db.Close() // Force connection failure; nothing may be published in memory.
	if _, err := again.UpdateGradeSubjectLevels("管理员", item.ID, learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S", "H"}}); err == nil {
		t.Fatal("expected persistence failure")
	}
	if again.settings[gradeSubjectCatalogSetting] != before || !reflect.DeepEqual(beforeSpaces, again.learningSpaces) {
		t.Fatal("failed write leaked into live state")
	}
}
