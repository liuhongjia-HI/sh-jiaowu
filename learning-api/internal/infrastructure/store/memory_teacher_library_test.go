package store

import (
	"os"
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func libraryTeacher(t *testing.T, s *MemoryStore) learning.Principal {
	t.Helper()
	p, err := s.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	p.TeacherLibrary = &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "english", Grade: "五年级"}}}
	p.CanUploadHandout = false
	p.CanUploadQuestion = false
	p.CanReview = false
	return p
}
func TestTeacherLibraryScopeAndFileAuthorization(t *testing.T) {
	s := NewMemoryStore()
	p := libraryTeacher(t, s)
	visible, _ := s.TeacherLibrary(p)
	if len(visible.Materials) == 0 {
		t.Fatal("expected published English materials")
	}
	for _, m := range visible.Materials {
		if !subjectsMatch(m.Subject, "english") || m.DownloadURL != "" {
			t.Fatalf("unexpected visible material: %s", m.ID)
		}
	}
	c := visible.Courses[0]
	// Intentionally inconsistent legacy association: the name alone must never grant access.
	s.materials = append(s.materials, learning.Material{ID: "outside", Course: c.Name, CourseID: "outside", LearningSpaceID: "outside", FileID: "outside-file", Status: learning.StatusEnabled})
	s.fileAssets["outside-file"] = learning.FileAsset{ID: "outside-file"}
	s.materials = append(s.materials, learning.Material{ID: "draft", Course: c.Name, CourseID: c.ID, LearningSpaceID: c.LearningSpaceID, Status: learning.StatusDraft})
	visible, _ = s.TeacherLibrary(p)
	for _, m := range visible.Materials {
		if m.ID == "outside" || m.ID == "draft" {
			t.Fatalf("leaked %s", m.ID)
		}
	}
	if _, err := s.ContentFile(p, "outside-file"); err == nil {
		t.Fatal("out-of-scope file authorized")
	}
	// Subject-level read access must not grant teaching writes.
	if _, err := s.CreateCourse("teacher", p, learning.CourseUpsertRequest{Name: "unauthorized", LearningSpaceID: c.LearningSpaceID}); err == nil {
		t.Fatal("readonly teacher can create course")
	}
	if err := s.DeleteCourse("teacher", p, c.ID); err == nil {
		t.Fatal("readonly teacher can delete course")
	}
	// A new matching course is included without adding learning-space IDs.
	newCourse := c
	newCourse.ID = "new-course"
	newCourse.Name = "New English"
	s.courses = append(s.courses, newCourse)
	visible, _ = s.TeacherLibrary(p)
	found := false
	for _, item := range visible.Courses {
		if item.ID == newCourse.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("new matching course missing")
	}
	p.TeacherLibrary.Scopes = []learning.TeacherLibraryScope{{Subject: "math"}}
	visible, _ = s.TeacherLibrary(p)
	for _, m := range visible.Materials {
		if subjectsMatch(m.Subject, "english") {
			t.Fatal("revoked material visible")
		}
	}
}
func TestTeacherLibraryReadOnlyAccountAndHistory(t *testing.T) {
	s := NewMemoryStore()
	admin, _ := s.PrincipalByUserID("user-super")
	policy := &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "english"}}, CanDownload: true}
	teacher, err := s.CreateTeacher("admin", admin, learning.TeacherUpsertRequest{Name: "查阅老师", Phone: "13911992288", TeacherLibrary: policy})
	if err != nil {
		t.Fatal(err)
	}
	if teacher.CanUploadHandout || teacher.CanReview || teacher.TeacherLibrary.CanManageCourses {
		t.Fatal("unexpected management permissions")
	}
	p, _ := s.PrincipalByUserID(teacher.ID)
	library, _ := s.TeacherLibrary(p)
	if len(library.Materials) == 0 {
		t.Fatal("missing library")
	}
	id := library.Materials[0].ID
	if err := s.RecordTeacherMaterialView(p, id); err != nil {
		t.Fatal(err)
	}
	p, _ = s.PrincipalByUserID(teacher.ID)
	library, _ = s.TeacherLibrary(p)
	if len(library.RecentMaterialIDs) != 1 || library.RecentMaterialIDs[0] != id {
		t.Fatal("view not recorded")
	}
	if err := s.RecordTeacherMaterialView(p, "outside"); err == nil {
		t.Fatal("can record inaccessible material")
	}
	p.TeacherLibrary.Scopes = []learning.TeacherLibraryScope{{Subject: "math"}}
	library, _ = s.TeacherLibrary(p)
	if len(library.RecentMaterialIDs) != 0 {
		t.Fatal("revoked history leaked")
	}
	// Returned policy cannot mutate the saved account through a shared pointer.
	fresh, _ := s.PrincipalByUserID(teacher.ID)
	if !subjectsMatch(fresh.TeacherLibrary.Scopes[0].Subject, "english") {
		t.Fatal("principal aliases saved policy")
	}
}
func TestTeacherMaterialSearchIncludesFilenameAndCurriculum(t *testing.T) {
	s := NewMemoryStore()
	p, _ := s.PrincipalByUserID("user-teacher")
	m := s.Materials(p, learning.MaterialQuery{})[0]
	for i := range s.materials {
		if s.materials[i].ID == m.ID {
			s.materials[i].FileName = "unique-worksheet-filename.pdf"
		}
	}
	if got := s.Materials(p, learning.MaterialQuery{Keyword: "unique-worksheet-filename"}); len(got) != 1 {
		t.Fatalf("filename search: %d", len(got))
	}
}
func TestTeacherLibraryMySQLRestartAndFailure(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated test database required")
	}
	s := NewMemoryStore()
	for i := range s.grants {
		s.grants[i].OpenedAt = "2026-09-20 09:00:00"
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	p, _ := s.PrincipalByUserID("user-teacher")
	library, _ := s.TeacherLibrary(p)
	if len(library.Materials) == 0 {
		t.Fatal("missing seed materials")
	}
	id := library.Materials[0].ID
	if err := s.RecordTeacherMaterialView(p, id); err != nil {
		t.Fatal(err)
	}
	admin, _ := s.PrincipalByUserID("user-super")
	created, err := s.CreateTeacher("acceptance", admin, learning.TeacherUpsertRequest{Name: "数据库查阅教师", Phone: "179" + time.Now().Format("15040500"), TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "english", Grade: "五年级"}}, CanDownload: true}})
	if err != nil {
		t.Fatal(err)
	}
	s.db.Close()
	restored := NewMemoryStore()
	if err := restored.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	persisted, err := restored.PrincipalByUserID(created.ID)
	if err != nil || persisted.TeacherLibrary == nil || !persisted.TeacherLibrary.CanDownload || persisted.TeacherLibrary.CanManageCourses || len(persisted.TeacherLibrary.Scopes) != 1 || persisted.CanUploadHandout {
		t.Fatal("readonly scope or capabilities did not survive reload")
	}
	p, _ = restored.PrincipalByUserID("user-teacher")
	library, _ = restored.TeacherLibrary(p)
	if len(library.RecentMaterialIDs) == 0 || library.RecentMaterialIDs[0] != id {
		t.Fatal("history did not survive reconnect")
	}
	if !p.TeacherLibrary.CanDownload || !p.TeacherLibrary.CanManageCourses || !p.TeacherLibrary.CanViewDrafts {
		t.Fatal("legacy rights changed")
	}
	before := teacherLibraryJSON(p.TeacherLibrary)
	restored.db.Close()
	if err := restored.RecordTeacherMaterialView(p, id); err == nil {
		t.Fatal("write succeeded with closed database")
	}
	for _, u := range restored.users {
		if u.ID == p.UserID && teacherLibraryJSON(u.TeacherLibrary) != before {
			t.Fatal("failed DB write changed history")
		}
	}
}
