package store

import (
	"fmt"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestTeacherUnreadVersionsSurviveRecentLimitAndPermissionChanges(t *testing.T) {
	s, _, _ := downloadFixture(t)
	p := libraryTeacher(t, s)
	library, err := s.TeacherLibrary(p)
	if err != nil || len(library.UnreadMaterialIDs) != 2 || len(library.Materials) != 2 {
		t.Fatalf("published unread selection: %#v %v", library, err)
	}
	id := library.Materials[0].ID
	if err := s.RecordTeacherMaterialView(p, id); err != nil {
		t.Fatal(err)
	}
	library, _ = s.TeacherLibrary(p)
	if containsString(library.UnreadMaterialIDs, id) {
		t.Fatal("viewed version remains unread")
	}
	for i := 0; i < 25; i++ {
		m := s.materials[0]
		m.ID = fmt.Sprintf("other-read-%d", i)
		s.materials = append(s.materials, m)
		if err := s.RecordTeacherMaterialView(p, m.ID); err != nil {
			t.Fatal(err)
		}
	}
	fresh, _ := s.PrincipalByUserID(p.UserID)
	library, _ = s.TeacherLibrary(fresh)
	if containsString(library.RecentMaterialIDs, id) || containsString(library.UnreadMaterialIDs, id) {
		t.Fatal("evicted recent entry became unread again")
	}
	for i := range s.materials {
		if s.materials[i].ID == id {
			s.materials[i].PreviewStatus = "可预览"
			s.materials[i].ViewCount++
		}
	}
	library, _ = s.TeacherLibrary(fresh)
	if containsString(library.UnreadMaterialIDs, id) {
		t.Fatal("preview conversion marked read material unread")
	}
	for i := range s.materials {
		if s.materials[i].ID == id {
			s.materials[i].FileID = "new-file-version"
		}
	}
	library, _ = s.TeacherLibrary(fresh)
	if !containsString(library.UnreadMaterialIDs, id) {
		t.Fatal("replacement file not unread")
	}
	fresh.TeacherLibrary = &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "math"}}}
	library, _ = s.TeacherLibrary(fresh)
	if len(library.UnreadMaterialIDs) != 0 {
		t.Fatal("unread IDs leaked revoked English materials")
	}
	if err := s.RecordTeacherMaterialView(fresh, id); err == nil {
		t.Fatal("revoked material marked read")
	}
	if len(s.teacherMaterialReads) != 26 {
		t.Fatalf("read records duplicated or lost: %d", len(s.teacherMaterialReads))
	}
	admin, _ := s.PrincipalByUserID("user-super")
	if _, err := s.UpdateTeacher("管理员", admin, p.UserID, learning.TeacherUpsertRequest{Name: p.Name, Phone: p.Phone, TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "english", Grade: "五年级"}}}}); err != nil {
		t.Fatal(err)
	}
	if len(s.teacherMaterialReads) != 26 {
		t.Fatal("permission update erased reads")
	}
}
