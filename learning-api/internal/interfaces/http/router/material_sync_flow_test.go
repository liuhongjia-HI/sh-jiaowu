package router_test

import (
	"bytes"
	"io"
	"net/http"
	"reflect"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestMaterialSyncHTTPUploadPublishMultipleTargetsAndStaleRetry(t *testing.T) {
	app := newTestAppWithStorageRoot(t, t.TempDir())
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	createCourse := func(prefix, space string) learning.Course {
		var course learning.Course
		app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: prefix, LearningSpaceID: space, Curriculum: apiTestCurriculum(prefix), Status: learning.StatusEnabled}, http.StatusOK, &course)
		return course
	}
	source := createCourse("sync-source", "space-g05-english-s1-q1")
	first := createCourse("sync-splus", "space-g05-english-s1-q1-splus")
	second := createCourse("sync-h", "space-g05-english-s1-q1-h")
	contents := teachingPlanPDF("Material sync original")
	var material learning.Material
	doMultipart(t, app, http.MethodPost, "/api/materials", token, map[string]string{"title": "同步讲义", "courseId": source.ID, "lessonId": "sync-source-lesson-1", "tagCode": "HD", "allowDownload": "true"}, "file", "同步.pdf", contents, http.StatusOK, &material)
	req := learning.MaterialSyncRequest{SourceCourseID: source.ID, SourceLessonID: material.LessonID, MaterialIDs: []string{material.ID}, Targets: []learning.MaterialSyncTarget{{CourseID: first.ID, LessonID: "sync-splus-lesson-1"}, {CourseID: second.ID, LessonID: "sync-h-lesson-1"}}}
	// Explicitly unpublish the uploaded material before verifying draft rejection.
	app.doJSON(t, http.MethodPut, "/api/materials/"+material.ID, token, learning.MaterialUpdateRequest{Title: material.Title, CourseID: source.ID, LearningSpaceID: source.LearningSpaceID, LessonID: material.LessonID, TagCode: "HD", Status: learning.StatusDraft, AllowDownload: true}, http.StatusOK, &material)
	app.doJSON(t, http.MethodPost, "/api/materials/sync-preview", token, req, http.StatusBadRequest, nil)
	app.doJSON(t, http.MethodPut, "/api/materials/"+material.ID, token, learning.MaterialUpdateRequest{Title: material.Title, CourseID: source.ID, LearningSpaceID: source.LearningSpaceID, LessonID: material.LessonID, TagCode: "HD", Status: learning.StatusEnabled, AllowDownload: true}, http.StatusOK, &material)
	var preview learning.MaterialSyncPreview
	app.doJSON(t, http.MethodPost, "/api/materials/sync-preview", token, req, http.StatusOK, &preview)
	if len(preview.Targets) != 2 || len(preview.Targets[0].Items) != 1 || preview.Targets[0].Items[0].Action != "create" {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	req.Snapshot = preview.Snapshot
	// A second destination changes after confirmation. Neither destination may save.
	second.Curriculum[2].Name = "新课节名称"
	app.doJSON(t, http.MethodPut, "/api/courses/"+second.ID, token, learning.CourseUpsertRequest{Name: second.Name, LearningSpaceID: second.LearningSpaceID, Curriculum: second.Curriculum, Status: second.Status}, http.StatusOK, &second)
	app.doJSON(t, http.MethodPost, "/api/materials/sync", token, req, http.StatusBadRequest, nil)
	var before []learning.Material
	app.doJSON(t, http.MethodGet, "/api/materials", token, nil, http.StatusOK, &before)
	for _, item := range before {
		if item.CourseID == first.ID || item.CourseID == second.ID {
			t.Fatal("stale request partially saved a target")
		}
	}
	app.doJSON(t, http.MethodPost, "/api/materials/sync-preview", token, req, http.StatusOK, &preview)
	req.Snapshot = preview.Snapshot
	var result learning.MaterialSyncResult
	app.doJSON(t, http.MethodPost, "/api/materials/sync", token, req, http.StatusOK, &result)
	if result.AlreadySynced || len(result.Targets) != 2 || result.Targets[0].Created != 1 || result.Targets[1].Created != 1 {
		t.Fatalf("unexpected sync result: %#v", result)
	}
	var after []learning.Material
	app.doJSON(t, http.MethodGet, "/api/materials", token, nil, http.StatusOK, &after)
	for _, target := range result.Targets {
		if len(target.MaterialIDs) != 1 {
			t.Fatalf("incomplete target: %#v", target)
		}
		found := false
		for _, item := range after {
			if item.ID != target.MaterialIDs[0] {
				continue
			}
			found = true
			if item.FileID != material.FileID || item.CourseID != target.CourseID || item.ID == material.ID || item.PublishStatus != "已发布" {
				t.Fatalf("incorrect target file/publication: %#v", item)
			}
			if item.CourseID == second.ID && item.Curriculum.Lesson != "新课节名称" {
				t.Fatalf("fresh preview path not applied: %#v", item)
			}
		}
		if !found {
			t.Fatal("target material absent from API listing")
		}
	}
	var repeated learning.MaterialSyncResult
	app.doJSON(t, http.MethodPost, "/api/materials/sync", token, req, http.StatusOK, &repeated)
	if !repeated.AlreadySynced || !reflect.DeepEqual(result.Targets[0].MaterialIDs, repeated.Targets[0].MaterialIDs) || !reflect.DeepEqual(result.Targets[1].MaterialIDs, repeated.Targets[1].MaterialIDs) {
		t.Fatalf("repeat created different copies: %#v", repeated)
	}
	var final []learning.Material
	app.doJSON(t, http.MethodGet, "/api/materials", token, nil, http.StatusOK, &final)
	if len(final) != len(before)+2 || len(final) != len(after) {
		t.Fatal("retry duplicated or lost materials")
	}
	app.doJSON(t, http.MethodPost, "/api/materials/sync", app.loginStudent(t), req, http.StatusForbidden, nil)
	request, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/files/"+material.FileID+"/download", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := app.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	download, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || !bytes.Equal(download, contents) {
		t.Fatalf("shared original file changed: status=%d err=%v", response.StatusCode, err)
	}
}

func TestMaterialUploadBatchHTTPMergesStudentNotifications(t *testing.T) {
	app := newTestAppWithStorageRoot(t, t.TempDir())
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: "批量提醒接口验收", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: apiTestCurriculum("notice-batch"), Status: learning.StatusEnabled}, http.StatusOK, &course)
	upload := func(batch, lesson string, status int) {
		t.Helper()
		doMultipart(t, app, http.MethodPost, "/api/materials", token, map[string]string{"batchId": batch, "title": "批量文件", "courseId": course.ID, "lessonId": lesson, "tagCode": "HD"}, "file", "讲义.pdf", teachingPlanPDF("Notice batch file"), status, nil)
	}
	upload("batch-http-one", "notice-batch-lesson-1", http.StatusOK)
	upload("batch-http-one", "notice-batch-lesson-1", http.StatusOK)
	upload("batch-http-one", "missing-lesson", http.StatusBadRequest)
	var pending []learning.PendingMaterialNoticeBatch
	app.doJSON(t, http.MethodGet, "/api/materials/notification-batches", token, nil, http.StatusOK, &pending)
	if len(pending) != 1 || pending[0].ResourceCount != 2 || pending[0].BatchID != "batch-http-one" {
		t.Fatalf("successful files not recoverable: %+v", pending)
	}
	var completed learning.MaterialNoticeBatchResult
	app.doJSON(t, http.MethodPost, "/api/materials/notification-batches/batch-http-one/complete", token, learning.MaterialNoticeBatchRequest{CourseID: course.ID}, http.StatusOK, &completed)
	if completed.ResourceCount != 2 || completed.AlreadyCompleted {
		t.Fatalf("wrong first completion: %+v", completed)
	}
	app.doJSON(t, http.MethodPost, "/api/materials/notification-batches/batch-http-one/complete", token, learning.MaterialNoticeBatchRequest{CourseID: course.ID}, http.StatusOK, &completed)
	if !completed.AlreadyCompleted {
		t.Fatal("repeated completion not recognized")
	}
	app.doJSON(t, http.MethodGet, "/api/materials/notification-batches", token, nil, http.StatusOK, &pending)
	if len(pending) != 0 {
		t.Fatal("completed batch remained pending")
	}
	student, err := app.store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	count := func() int {
		t.Helper()
		home, err := app.store.StudentHome(student)
		if err != nil {
			t.Fatal(err)
		}
		result := 0
		for _, notice := range home.Notices {
			if notice.RelatedID == course.ID {
				if notice.RecipientStudentID != student.StudentID {
					t.Fatal("another student's batch notification leaked")
				}
				result++
			}
		}
		return result
	}
	if n := count(); n != 1 {
		t.Fatalf("one operation produced %d student notices", n)
	}
	upload("batch-http-two", "notice-batch-lesson-1", http.StatusOK)
	if n := count(); n != 2 {
		t.Fatalf("independent batch produced %d total notices", n)
	}
}
