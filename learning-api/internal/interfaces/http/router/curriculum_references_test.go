package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestCurriculumReferencesHTTPRejectsDeletionAndStudentAccess(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: "章节引用 HTTP 验收", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: apiTestCurriculum("reference-http"), Status: learning.StatusEnabled}, http.StatusOK, &course)
	var material learning.Material
	doMultipart(t, app, http.MethodPost, "/api/materials", token, map[string]string{"title": "待关联讲义", "courseId": course.ID, "learningSpaceId": course.LearningSpaceID, "lessonId": "reference-http-lesson-1"}, "file", "章节.pdf", []byte("%PDF-reference"), http.StatusOK, &material)
	path := "/api/courses/" + course.ID + "/curriculum-references"
	request := learning.CurriculumReferencesRequest{NodeIDs: []string{"reference-http-lesson-1"}}
	var refs []learning.CurriculumReference
	app.doJSON(t, http.MethodPost, path, token, request, http.StatusOK, &refs)
	if len(refs) != 1 || refs[0].ID != material.ID || refs[0].Title != "待关联讲义" {
		t.Fatalf("HTTP preview: %#v", refs)
	}
	app.doJSON(t, http.MethodPost, path, app.loginStudent(t), request, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPut, "/api/courses/"+course.ID, token, learning.CourseUpsertRequest{Name: course.Name, LearningSpaceID: course.LearningSpaceID, Curriculum: []learning.CurriculumNode{{ID: "replacement-unit", Type: learning.CurriculumUnit, Name: "替代章节", SortOrder: 1}}, Status: course.Status}, http.StatusBadRequest, nil)
}
