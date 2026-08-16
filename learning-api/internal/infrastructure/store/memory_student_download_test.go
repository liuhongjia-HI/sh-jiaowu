package store

import "testing"

func TestStudentMaterialDownloadURLOnlyExistsDuringActiveGrant(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	for index := range store.materials {
		if store.materials[index].ID == "mat-g05-english-s1-q1" {
			store.materials[index].FileID = "file-student-download"
			store.materials[index].FileName = "lesson.pdf"
			break
		}
	}
	material, err := store.StudentMaterial(principal, "mat-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("expected active material access: %v", err)
	}
	if material.DownloadURL != "/api/student/materials/mat-g05-english-s1-q1/download" {
		t.Fatalf("expected student download url, got %q", material.DownloadURL)
	}
	for index := range store.grants {
		if store.grants[index].StudentID == principal.StudentID {
			store.grants[index].EndsAt = "2000-01-01"
			store.grants[index].EffectiveUntil = "2000-01-01"
		}
	}
	if _, err := store.StudentMaterial(principal, "mat-g05-english-s1-q1"); err == nil {
		t.Fatal("expected download source material to become inaccessible after grant expiry")
	}
}
