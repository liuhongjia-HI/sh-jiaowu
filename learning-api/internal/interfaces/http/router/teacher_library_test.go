package router_test

import (
	"net/http"
	"os"
	"path/filepath"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestTeacherLibraryReadOnlyAndLiveRevocation(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	admin, _ := app.store.PrincipalByUserID("user-super")
	teacher, _ := app.store.PrincipalByUserID("user-teacher")
	source := filepath.Join(t.TempDir(), "handout.pdf")
	if err := os.WriteFile(source, []byte("test preview"), 0600); err != nil {
		t.Fatal(err)
	}
	material, err := app.store.CreateMaterial("admin", admin, learning.MaterialUploadRequest{Title: "教师资料权限验收", CourseID: "course-g05-english-s1-q1", LessonID: "course-g05-english-s1-q1-lesson-1", File: learning.FileAsset{ID: "teacher-library-file", FileName: "handout.pdf", FileType: "PDF", OriginalPath: source, PreviewPath: source, PreviewStatus: "可预览"}})
	if err != nil {
		t.Fatal(err)
	}
	// Login before changing the scope: the existing token must obey the new policy.
	token := app.loginAdmin(t, "13800000004")
	req := learning.TeacherUpsertRequest{Name: "英语老师", Phone: "13800000004", LearningSpaceIDs: teacher.LearningSpaceIDs, TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "english"}}}}
	if _, err := app.store.UpdateTeacher("admin", admin, teacher.UserID, req); err != nil {
		t.Fatal(err)
	}
	var library learning.TeacherLibrary
	app.doJSON(t, http.MethodGet, "/api/teacher/library", token, nil, http.StatusOK, &library)
	if library.CanDownload {
		t.Fatal("unexpected download permission")
	}
	app.doJSON(t, http.MethodGet, material.DownloadURL, token, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodDelete, "/api/courses/course-g05-english-s1-q1", token, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, "/api/notices", token, map[string]string{}, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodGet, "/api/students", token, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, "/api/teacher/materials/"+material.ID+"/view", token, map[string]string{}, http.StatusOK, nil)
	req.TeacherLibrary.Scopes = []learning.TeacherLibraryScope{{Subject: "math"}}
	if _, err := app.store.UpdateTeacher("admin", admin, teacher.UserID, req); err != nil {
		t.Fatal(err)
	}
	app.doJSON(t, http.MethodGet, "/api/teacher/library", token, nil, http.StatusOK, &library)
	if len(library.RecentMaterialIDs) != 0 {
		t.Fatal("revoked history visible")
	}
	app.doJSON(t, http.MethodGet, material.PreviewURL, token, nil, http.StatusBadRequest, nil)
	req.AccountStatus = "停用"
	if _, err := app.store.UpdateTeacher("admin", admin, teacher.UserID, req); err != nil {
		t.Fatal(err)
	}
	app.doJSON(t, http.MethodGet, "/api/teacher/library", token, nil, http.StatusUnauthorized, nil)
}
