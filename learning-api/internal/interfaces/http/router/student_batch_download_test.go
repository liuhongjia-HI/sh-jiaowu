package router_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/handler"
	"strings"
	"testing"
	"time"
)

func TestStudentBatchDownloadHTTPComputerApprovalAndArchive(t *testing.T) {
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	admin := app.loginAdmin(t, "13800000001")
	student := app.loginStudent(t)
	p, _ := app.store.PrincipalByUserID("user-student-001")
	if _, err := app.store.CreateDirectGrant("test", learning.DirectGrantCreateRequest{StudentID: p.StudentID, LearningSpaceIDs: []string{"space-g05-english-s1-q1"}, ContentTypeCodes: []string{"course", "handout", "download"}, StartsAt: "2026-01-01", EndsAt: "2027-12-31"}); err != nil {
		t.Fatal(err)
	}
	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", admin, learning.CourseUpsertRequest{Name: "学生整包", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: apiTestCurriculum("student-pack"), Status: learning.StatusEnabled}, http.StatusOK, &course)
	var material learning.Material
	content := []byte("%PDF-1.4 student download contents")
	doMultipart(t, app, http.MethodPost, "/api/materials", admin, map[string]string{"title": "学生讲义", "courseId": course.ID, "lessonId": "student-pack-lesson-1"}, "file", "讲义.pdf", content, http.StatusOK, &material)
	update := learning.MaterialUpdateRequest{Title: material.Title, CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: material.LessonID, Status: learning.StatusEnabled, AllowDownload: true}
	app.doJSON(t, http.MethodPut, "/api/materials/"+material.ID, admin, update, http.StatusOK, nil)
	scope := learning.MaterialDownloadScope{CourseIDs: []string{course.ID}, MaterialIDs: []string{material.ID}}
	var quote learning.MaterialDownloadSelection
	app.doJSON(t, http.MethodPost, "/api/student/material-downloads/selection", student, scope, http.StatusOK, &quote)
	if quote.Count != 1 {
		t.Fatalf("selection: %#v", quote)
	}
	app.doJSON(t, http.MethodPost, "/api/material-downloads", student, scope, http.StatusForbidden, nil)
	var job learning.MaterialDownloadJob
	app.doJSON(t, http.MethodPost, "/api/student/material-downloads", student, scope, http.StatusOK, &job)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		handler.NewMaterialDownloadWorker(learningapp.NewService(app.store), root).Run(ctx)
		close(done)
	}()
	defer func() { cancel(); <-done }()
	end := time.Now().Add(5 * time.Second)
	for {
		jobs := app.store.MaterialDownloads(p)
		if len(jobs) > 0 && jobs[0].Status == "可下载" {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("worker did not finish: %#v", jobs)
		}
		time.Sleep(10 * time.Millisecond)
	}
	var session struct {
		Challenge  string `json:"challenge"`
		BrowserKey string `json:"browserKey"`
	}
	app.doJSON(t, http.MethodPost, "/api/download-pickups", "", map[string]string{"jobId": job.ID}, http.StatusOK, &session)
	base := "/api/download-pickups/" + session.Challenge
	app.doJSON(t, http.MethodGet, base, session.Challenge, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodGet, base+"/archive", session.BrowserKey, nil, http.StatusForbidden, nil)
	var waiting map[string]any
	app.doJSON(t, http.MethodGet, base, session.BrowserKey, nil, http.StatusOK, &waiting)
	if waiting["status"] != "waiting" || waiting["job"] != nil {
		t.Fatal("metadata exposed before phone confirmation")
	}
	app.doJSON(t, http.MethodGet, "/api/student/download-pickups/"+session.Challenge, student, nil, http.StatusOK, nil)
	app.doJSON(t, http.MethodPost, "/api/student/download-pickups/"+session.Challenge+"/confirm", admin, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, "/api/student/download-pickups/"+session.Challenge+"/confirm", student, nil, http.StatusOK, nil)
	var approved map[string]any
	app.doJSON(t, http.MethodGet, base, session.BrowserKey, nil, http.StatusOK, &approved)
	if approved["status"] != "approved" {
		t.Fatal("phone approval not reflected")
	}
	req, _ := http.NewRequest(http.MethodGet, app.server.URL+base+"/archive", nil)
	req.Header.Set("Authorization", "Bearer "+session.BrowserKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("archive status %d: %s", res.StatusCode, raw)
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	found, manifest, readableList := false, false, false
	for _, file := range archive.File {
		if file.Name == "资料清单.txt" {
			r, _ := file.Open()
			data, _ := io.ReadAll(r)
			r.Close()
			readableList = strings.Contains(string(data), job.StudentName) && strings.Contains(string(data), "讲义.pdf")
		}
		if file.Name == "manifest.json" {
			manifest = true
		}
		if strings.HasSuffix(file.Name, "讲义.pdf") {
			r, _ := file.Open()
			data, _ := io.ReadAll(r)
			r.Close()
			found = bytes.Equal(data, content)
		}
	}
	if !found || !manifest || !readableList {
		t.Fatal("archive incomplete")
	}
	// A formerly approved browser must fail immediately when download permission changes.
	update.AllowDownload = false
	app.doJSON(t, http.MethodPut, "/api/materials/"+material.ID, admin, update, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, base+"/archive", session.BrowserKey, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodGet, base, session.BrowserKey, nil, http.StatusForbidden, nil)
}
