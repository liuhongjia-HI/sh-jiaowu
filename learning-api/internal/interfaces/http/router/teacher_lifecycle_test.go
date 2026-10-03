package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestTeacherLifecycleRoutesAndSessionRevocation(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	adminToken := app.loginAdmin(t, "13800000001")
	teacherToken := app.loginAdmin(t, "13800000004")
	app.doJSON(t, http.MethodDelete, "/api/teachers/user-teacher", teacherToken, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPut, "/api/teachers/user-teacher/status", teacherToken, map[string]string{"accountStatus": "停用"}, http.StatusForbidden, nil)
	var disabled learning.Teacher
	app.doJSON(t, http.MethodPut, "/api/teachers/user-teacher/status", adminToken, map[string]string{"accountStatus": "停用"}, http.StatusOK, &disabled)
	if disabled.AccountStatus != "停用" {
		t.Fatal("status not updated")
	}
	app.doJSON(t, http.MethodGet, "/api/auth/me", teacherToken, nil, http.StatusUnauthorized, nil)
	app.doJSON(t, http.MethodPut, "/api/teachers/user-teacher/status", adminToken, map[string]string{"accountStatus": "正常"}, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/auth/me", teacherToken, nil, http.StatusUnauthorized, nil)
	var created learning.Teacher
	app.doJSON(t, http.MethodPost, "/api/teachers", adminToken, learning.TeacherUpsertRequest{Name: "测试教师", Phone: "13911229988", CampusID: "campus-main", LearningSpaceIDs: []string{"space-g05-english-s1-q1"}}, http.StatusOK, &created)
	app.doJSON(t, http.MethodDelete, "/api/teachers/"+created.ID, adminToken, nil, http.StatusOK, nil)
	app.doJSON(t, http.MethodDelete, "/api/teachers/"+created.ID, adminToken, nil, http.StatusBadRequest, nil)
	app.doJSON(t, http.MethodPut, "/api/teachers/user-teacher/status", adminToken, map[string]string{"accountStatus": "invalid"}, http.StatusBadRequest, nil)
}
