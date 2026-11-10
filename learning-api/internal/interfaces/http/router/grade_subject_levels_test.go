package router_test

import (
	"net/http"
	"reflect"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestGradeSubjectLevelsAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	admin := app.loginAdmin(t, "13800000002")
	student := app.loginStudent(t)
	path := "/api/grade-subjects/g5-science/levels"
	req := learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S", "S+"}}
	app.doJSON(t, http.MethodPut, path, student, req, http.StatusForbidden, nil)
	for _, levels := range [][]string{nil, {}, {"SS"}, {"S", "S"}} {
		app.doJSON(t, http.MethodPut, path, admin, learning.GradeSubjectLevelsUpdateRequest{Levels: levels}, http.StatusBadRequest, nil)
	}
	var saved learning.GradeSubjectMetadata
	app.doJSON(t, http.MethodPut, path, admin, req, http.StatusOK, &saved)
	if !reflect.DeepEqual(saved.Levels, req.Levels) {
		t.Fatal(saved)
	}
	var spaces []learning.LearningSpace
	app.doJSON(t, http.MethodGet, "/api/learning-spaces", admin, nil, http.StatusOK, &spaces)
	found := 0
	for _, space := range spaces {
		if space.Grade == "五年级" && space.Subject == "科学" && space.Level == "S+" && space.Selectable {
			found++
		}
	}
	if found != 4 {
		t.Fatalf("expected 4 new selectable spaces, got %d", found)
	}
	app.doJSON(t, http.MethodPut, path, admin, learning.GradeSubjectLevelsUpdateRequest{Levels: []string{"S"}}, http.StatusOK, nil)
	app.doJSON(t, http.MethodPost, "/api/courses", admin, learning.CourseUpsertRequest{Name: "禁止新增取消班型", LearningSpaceID: "space-g05-science-s1-q1-splus", Curriculum: apiTestCurriculum("cancelled-level")}, http.StatusBadRequest, nil)
	app.doJSON(t, http.MethodPost, "/api/packages", admin, learning.PackageUpsertRequest{Name: "禁止新增取消方案", Grade: "五年级", Subject: "科学", Semester: "S1", LearningSpaceIDs: []string{"space-g05-science-s1-q1-splus"}, ContentTypeCodes: []string{"handout"}}, http.StatusBadRequest, nil)
}
