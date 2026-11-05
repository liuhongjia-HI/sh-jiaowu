package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestMaterialReplacementMultipartAndStationDetail(t *testing.T) {
	app := newTestAppWithStorageRoot(t, t.TempDir())
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: "五年级英语讲义", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: apiTestCurriculum("version"), Status: learning.StatusEnabled}, http.StatusOK, &course)
	fields := map[string]string{"title": "第一课讲义", "courseId": course.ID, "lessonId": "version-lesson-1", "tagCode": "HD", "status": "已发布", "allowDownload": "true"}
	var first, updated learning.Material
	doMultipart(t, app, http.MethodPost, "/api/materials", token, fields, "file", "原版.pdf", teachingPlanPDF("Original"), http.StatusOK, &first)
	doMultipart(t, app, http.MethodPut, "/api/materials/"+first.ID, token, fields, "file", "新版.pdf", teachingPlanPDF("Updated"), http.StatusOK, &updated)
	if updated.ID != first.ID || updated.FileID == first.FileID || updated.FileName != "新版.pdf" || updated.PreviewStatus != "待转换" {
		t.Fatalf("invalid replacement: %+v", updated)
	}
	doMultipart(t, app, http.MethodPut, "/api/materials/"+first.ID, token, fields, "file", "伪造.pdf", []byte("not a pdf"), http.StatusBadRequest, nil)
	studentToken := app.loginStudent(t)
	var notices []learning.Notice
	app.doJSON(t, http.MethodGet, "/api/student/notices", studentToken, nil, http.StatusOK, &notices)
	found := false
	for _, notice := range notices {
		if notice.RelatedType != "business" || notice.Summary != course.Name+" / 共1份资料" {
			continue
		}
		var detail learning.BusinessNoticeDetail
		app.doJSON(t, http.MethodGet, "/api/student/business-notices/"+notice.RelatedID, studentToken, nil, http.StatusOK, &detail)
		if detail.NoticeID != notice.ID || len(detail.CurrentMaterials) != 1 || detail.Event.CreatedAt == "" {
			t.Fatalf("invalid station detail: %+v", detail)
		}
		app.doJSON(t, http.MethodPost, "/api/student/notices/"+notice.ID+"/read", studentToken, map[string]any{}, http.StatusOK, nil)
		found = true
	}
	if !found {
		t.Fatal("replacement missing from student inbox")
	}

	// JSON cannot inject server-owned file metadata.
	app.doJSON(t, http.MethodPut, "/api/materials/"+first.ID, token, map[string]any{"title": updated.Title, "courseId": course.ID, "lessonId": updated.LessonID, "tagCode": "HD", "status": "已发布", "file": map[string]any{"id": "injected", "fileName": "injected.pdf"}}, http.StatusOK, &updated)
	if updated.FileID == "injected" {
		t.Fatal("accepted unvalidated file metadata")
	}
}
