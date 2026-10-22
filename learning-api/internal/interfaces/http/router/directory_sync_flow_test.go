package router_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestDirectorySyncHTTPConflictPublishedMaterialsAndRetry(t *testing.T) {
	app := newTestAppWithStorageRoot(t, t.TempDir())
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	var source, target learning.Course
	nodes := []learning.CurriculumNode{{ID: "http-source-unit", Type: learning.CurriculumUnit, Name: "Fertile Crescent", SortOrder: 2}, {ID: "http-source-chapter", ParentID: "http-source-unit", Type: learning.CurriculumChapter, Name: "The Sumerians", SortOrder: 2}}
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: "目录同步源课程", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: nodes, Status: learning.StatusEnabled}, http.StatusOK, &source)
	targetNodes := []learning.CurriculumNode{{ID: "http-target-unit", Type: learning.CurriculumUnit, SortOrder: 2}, {ID: "http-target-chapter", ParentID: "http-target-unit", Type: learning.CurriculumChapter, Name: "The Sumerians", SortOrder: 2}}
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: "目录同步目标课程", LearningSpaceID: "space-g05-english-s1-q1-splus", Curriculum: targetNodes, Status: learning.StatusEnabled}, http.StatusOK, &target)
	contents := teachingPlanPDF("Unit sync published handout")
	var sourceMaterial, own learning.Material
	doMultipart(t, app, http.MethodPost, "/api/materials", token, map[string]string{"title": "文明起源讲义", "courseId": source.ID, "lessonId": "http-source-chapter", "tagCode": "HD", "allowDownload": "true"}, "file", "文明.pdf", contents, http.StatusOK, &sourceMaterial)
	doMultipart(t, app, http.MethodPost, "/api/materials", token, map[string]string{"title": "目标原有讲义", "courseId": target.ID, "lessonId": "http-target-chapter", "tagCode": "HD"}, "file", "原有.pdf", teachingPlanPDF("Original target"), http.StatusOK, &own)
	req := learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID}, UnitIDs: []string{"http-source-unit"}}
	var preview, result learning.CourseDirectorySyncResult
	app.doJSON(t, http.MethodPost, "/api/courses/directory-sync-preview", token, req, http.StatusOK, &preview)
	if len(preview.Targets[0].Conflicts) != 1 || preview.Targets[0].Snapshot != "" {
		t.Fatalf("missing conflict: %#v", preview)
	}
	app.doJSON(t, http.MethodPost, "/api/courses/directory-sync", token, req, http.StatusOK, &result)
	if result.Targets[0].Status != "失败" {
		t.Fatal("unresolved conflict accepted")
	}
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {"http-source-unit": {TargetNodeID: "http-target-unit"}}}
	app.doJSON(t, http.MethodPost, "/api/courses/directory-sync-preview", token, req, http.StatusOK, &preview)
	if len(preview.Targets[0].MaterialsAdded) != 1 {
		t.Fatalf("missing handout preview: %#v", preview)
	}
	req.Snapshots = map[string]string{target.ID: preview.Targets[0].Snapshot}
	app.doJSON(t, http.MethodPost, "/api/courses/directory-sync", token, req, http.StatusOK, &result)
	if result.Targets[0].Status != "已同步" {
		t.Fatalf("sync failed: %#v", result)
	}
	app.doJSON(t, http.MethodPost, "/api/courses/directory-sync", token, req, http.StatusOK, &result)
	if result.Targets[0].Status != "已同步" {
		t.Fatalf("retry failed: %#v", result)
	}
	var courses []learning.Course
	app.doJSON(t, http.MethodGet, "/api/courses", token, nil, http.StatusOK, &courses)
	for _, c := range courses {
		if c.ID == target.ID && (len(c.Curriculum) != 2 || c.Curriculum[0].ID != "http-target-unit") {
			t.Fatalf("target identity lost: %#v", c)
		}
	}
	var materials []learning.Material
	app.doJSON(t, http.MethodGet, "/api/materials", token, nil, http.StatusOK, &materials)
	count := 0
	for _, m := range materials {
		if m.CourseID != target.ID {
			continue
		}
		count++
		if m.ID == own.ID {
			if m.FileID != own.FileID {
				t.Fatal("target own file overwritten")
			}
		} else if m.FileID != sourceMaterial.FileID || m.LessonID != "http-target-chapter" {
			t.Fatalf("wrong synchronized handout: %#v", m)
		}
	}
	if count != 2 {
		t.Fatalf("want own and one synced handout, got %d", count)
	}
	app.doJSON(t, http.MethodPost, "/api/courses/directory-sync", app.loginStudent(t), req, http.StatusForbidden, nil)
	request, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/files/"+sourceMaterial.FileID+"/download", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := app.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	downloaded, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || !bytes.Equal(downloaded, contents) {
		t.Fatal("original file bytes changed")
	}
}
