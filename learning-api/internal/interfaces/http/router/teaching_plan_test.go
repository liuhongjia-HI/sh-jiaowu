package router_test

import (
	"net/http"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestTeachingPlanUploadVisibilityAndDirectFileAccess(t *testing.T) {
	app := newTestAppWithStorageRoot(t, t.TempDir())
	defer app.close()
	adminToken := app.loginAdmin(t, "13800000001")
	teacherToken := app.loginAdmin(t, "13800000004")
	studentToken := app.loginStudent(t)
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", studentToken, nil, http.StatusForbidden, nil)
	fields := map[string]string{"grade": "五年级", "subject": "英文"}
	var fromAdmin, fromTeacher learning.TeachingPlan
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", adminToken, fields, "file", "管理员教案.pdf", []byte("%PDF-1.4 admin plan"), http.StatusOK, &fromAdmin)
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", teacherToken, fields, "file", "教师教案.pdf", []byte("%PDF-1.4 teacher plan"), http.StatusOK, &fromTeacher)
	if fromAdmin.ID == "" || fromTeacher.ID == "" || fromAdmin.ID == fromTeacher.ID {
		t.Fatal("plans were not created independently")
	}
	for _, token := range []string{adminToken, teacherToken} {
		var list learning.TeachingPlanList
		app.doJSON(t, http.MethodGet, "/api/teaching-plans", token, nil, http.StatusOK, &list)
		if len(list.Plans) != 2 {
			t.Fatalf("expected both uploaders' plans, got %d", len(list.Plans))
		}
		for _, plan := range []learning.TeachingPlan{fromAdmin, fromTeacher} {
			app.doJSON(t, http.MethodGet, "/api/teaching-plans/"+plan.ID, token, nil, http.StatusOK, nil)
			request, _ := http.NewRequest(http.MethodGet, app.server.URL+plan.DownloadURL, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("authorized download: %d", response.StatusCode)
			}
		}
	}
	for _, plan := range []learning.TeachingPlan{fromAdmin, fromTeacher} {
		for _, path := range []string{"/api/teaching-plans/" + plan.ID, plan.PreviewURL, plan.DownloadURL} {
			app.doJSON(t, http.MethodGet, path, studentToken, nil, http.StatusForbidden, nil)
		}
	}
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", studentToken, fields, "file", "student.pdf", []byte("%PDF-1.4 student"), http.StatusForbidden, nil)
	// Scope changes apply to existing tokens and to direct file links immediately.
	admin, _ := app.store.PrincipalByUserID("user-super")
	teacher, _ := app.store.PrincipalByUserID("user-teacher")
	_, err := app.store.UpdateTeacher("admin", admin, teacher.UserID, learning.TeacherUpsertRequest{Name: teacher.Name, Phone: teacher.Phone, TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "math", Grade: "五年级"}}}})
	if err != nil {
		t.Fatal(err)
	}
	var list learning.TeachingPlanList
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", teacherToken, nil, http.StatusOK, &list)
	if len(list.Plans) != 0 {
		t.Fatal("scope change did not hide plans")
	}
	for _, path := range []string{"/api/teaching-plans/" + fromAdmin.ID, fromAdmin.PreviewURL, fromAdmin.DownloadURL} {
		app.doJSON(t, http.MethodGet, path, teacherToken, nil, http.StatusForbidden, nil)
	}
}

func TestReadOnlyTeacherCanFindTeachingPlans(t *testing.T) {
	app := newTestAppWithStorageRoot(t, t.TempDir())
	defer app.close()
	admin, _ := app.store.PrincipalByUserID("user-super")
	teacher, _ := app.store.PrincipalByUserID("user-teacher")
	adminToken := app.loginAdmin(t, "13800000001")
	var plan learning.TeachingPlan
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", adminToken, map[string]string{"grade": "五年级", "subject": "英文"}, "file", "教案.pdf", []byte("%PDF-1.4 plan"), http.StatusOK, &plan)
	_, err := app.store.UpdateTeacher("admin", admin, teacher.UserID, learning.TeacherUpsertRequest{Name: teacher.Name, Phone: teacher.Phone, LearningSpaceIDs: teacher.LearningSpaceIDs, TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Grade: "五年级", Subject: "英文"}}, CanManageCourses: false}})
	if err != nil {
		t.Fatal(err)
	}
	token := app.loginAdmin(t, "13800000004")
	var list learning.TeachingPlanList
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", token, nil, http.StatusOK, &list)
	if len(list.Plans) != 1 || list.CanUpload {
		t.Fatalf("unexpected read-only list: %#v", list)
	}
	app.doJSON(t, http.MethodGet, "/api/teaching-plans/"+plan.ID, token, nil, http.StatusOK, nil)
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", token, map[string]string{"grade": "五年级", "subject": "英文"}, "file", "教案.pdf", []byte("%PDF-1.4 plan"), http.StatusForbidden, nil)
}
