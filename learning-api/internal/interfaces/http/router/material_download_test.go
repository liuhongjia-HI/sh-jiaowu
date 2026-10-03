package router_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/handler"
)

func TestMaterialDownloadHTTPMidFileShutdownRecoversOriginalJob(t *testing.T) {
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	p, _ := app.store.PrincipalByUserID("user-super")
	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: "中断恢复验收", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: apiTestCurriculum("recover-download"), Status: learning.StatusEnabled}, http.StatusOK, &course)
	contents := make([]byte, 32*1024*1024)
	if _, err := rand.Read(contents); err != nil {
		t.Fatal(err)
	}
	copy(contents, "%PDF-1.4\n")
	var material learning.Material
	doMultipart(t, app, http.MethodPost, "/api/materials", token, map[string]string{"title": "大文件中断", "courseId": course.ID, "learningSpaceId": course.LearningSpaceID, "lessonId": "recover-download-lesson-1"}, "file", "大文件.pdf", contents, http.StatusOK, &material)
	app.doJSON(t, http.MethodPut, "/api/materials/"+material.ID, token, learning.MaterialUpdateRequest{Title: material.Title, CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: material.LessonID, Status: learning.StatusEnabled}, http.StatusOK, nil)
	var job learning.MaterialDownloadJob
	app.doJSON(t, http.MethodPost, "/api/material-downloads", token, learning.MaterialDownloadScope{Subject: course.Subject, CourseIDs: []string{course.ID}}, http.StatusOK, &job)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		handler.NewMaterialDownloadWorker(learningapp.NewService(app.store), root).Run(ctx)
		close(done)
	}()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(10 * time.Second)
	interrupted := false
	for time.Now().Before(deadline) {
		files, _ := filepath.Glob(filepath.Join(root, "material-downloads", "*.tmp"))
		if len(files) > 0 {
			if info, err := os.Stat(files[0]); err == nil && info.Size() >= 64*1024 {
				cancel()
				<-done
				interrupted = true
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if !interrupted {
		t.Fatal("did not observe mid-file shutdown")
	}
	jobs := app.store.MaterialDownloads(p)
	if len(jobs) != 1 || jobs[0].ID != job.ID || jobs[0].Status != "打包中" {
		t.Fatalf("shutdown did not retain original job: %#v", jobs)
	}
	files, err := os.ReadDir(filepath.Join(root, "material-downloads"))
	if err != nil || len(files) != 0 {
		t.Fatalf("partial archive left behind: %#v %v", files, err)
	}
	app.doJSON(t, http.MethodGet, "/api/material-downloads/"+job.ID+"/archive", token, nil, http.StatusForbidden, nil)
	restarted := handler.NewMaterialDownloadWorker(learningapp.NewService(app.store), root)
	if err := restarted.Recover(); err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan struct{})
	go func() { restarted.Run(ctx2); close(done2) }()
	defer func() { cancel2(); <-done2 }()
	deadline = time.Now().Add(10 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		jobs = app.store.MaterialDownloads(p)
		if len(jobs) == 1 && jobs[0].ID == job.ID && jobs[0].Status == "可下载" {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("original job not recovered: %#v", jobs)
	}
	request, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/material-downloads/"+job.ID+"/archive", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := app.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("recovered pickup: %d %v", response.StatusCode, err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 {
		t.Fatal("recovered manifest incomplete")
	}
	file, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	original, err := io.ReadAll(file)
	file.Close()
	if err != nil || !bytes.Equal(original, contents) {
		t.Fatal("recovered file bytes changed")
	}
	// Exercise a real TCP response with a slow reader. Removing an archive
	// after the response has opened it must not truncate the active transfer.
	server := httptest.NewServer(app.server.transport.handler)
	defer server.Close()
	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/material-downloads/"+job.ID+"/archive", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("TCP pickup: %d", response.StatusCode)
	}
	buffer := make([]byte, 32*1024)
	var streamed bytes.Buffer
	n, err := response.Body.Read(buffer)
	if err != nil || n == 0 {
		t.Fatalf("stream never opened: %d %v", n, err)
	}
	streamed.Write(buffer[:n])
	archive, err := app.store.MaterialDownloadArchive(p, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	for {
		n, err = response.Body.Read(buffer)
		streamed.Write(buffer[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("active pickup interrupted by cleanup: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !bytes.Equal(streamed.Bytes(), data) {
		t.Fatal("slow TCP pickup lost archive bytes after cleanup")
	}
	app.doJSON(t, http.MethodGet, "/api/material-downloads/"+job.ID+"/archive", token, nil, http.StatusBadRequest, nil)
}

func TestMaterialDownloadHTTPCompletePickupAndMissingArchiveRetry(t *testing.T) {
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	adminToken := app.loginAdmin(t, "13800000001")
	teacherToken := app.loginAdmin(t, "13800000004")
	studentToken := app.loginStudent(t)
	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", adminToken, learning.CourseUpsertRequest{Name: "下载验收", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: apiTestCurriculum("download-http"), Status: learning.StatusEnabled}, http.StatusOK, &course)
	contents := []byte("%PDF-1.4 real archive test contents")
	var material learning.Material
	doMultipart(t, app, http.MethodPost, "/api/materials", adminToken, map[string]string{"title": "打包文件", "courseId": course.ID, "learningSpaceId": course.LearningSpaceID, "lessonId": "download-http-lesson-1"}, "file", "讲义.pdf", contents, http.StatusOK, &material)
	app.doJSON(t, http.MethodPut, "/api/materials/"+material.ID, adminToken, learning.MaterialUpdateRequest{Title: material.Title, CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: material.LessonID, Status: learning.StatusEnabled}, http.StatusOK, nil)
	scope := learning.MaterialDownloadScope{Subject: course.Subject, CourseIDs: []string{course.ID}}
	app.doJSON(t, http.MethodPost, "/api/material-downloads/selection", studentToken, scope, http.StatusForbidden, nil)
	var selection learning.MaterialDownloadSelection
	app.doJSON(t, http.MethodPost, "/api/material-downloads/selection", adminToken, scope, http.StatusOK, &selection)
	if selection.Count != 1 || selection.Size != int64(len(contents)) {
		t.Fatalf("selection mismatch: %#v", selection)
	}
	var job learning.MaterialDownloadJob
	app.doJSON(t, http.MethodPost, "/api/material-downloads", adminToken, scope, http.StatusOK, &job)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		handler.NewMaterialDownloadWorker(learningapp.NewService(app.store), root).Run(ctx)
		close(done)
	}()
	defer func() { cancel(); <-done }()
	p, _ := app.store.PrincipalByUserID("user-super")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		jobs := app.store.MaterialDownloads(p)
		if len(jobs) > 0 && jobs[0].Status == "可下载" {
			break
		}
		if len(jobs) > 0 && jobs[0].Status == "失败" {
			t.Fatalf("worker failed: %#v", jobs[0])
		}
		time.Sleep(20 * time.Millisecond)
	}
	archiveURL := "/api/material-downloads/" + job.ID + "/archive"
	app.doJSON(t, http.MethodGet, archiveURL, teacherToken, nil, http.StatusForbidden, nil)
	req, _ := http.NewRequest(http.MethodGet, app.server.URL+archiveURL, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	response, err := app.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("pickup response: %d %s", response.StatusCode, data)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("incomplete ZIP: %d", len(reader.File))
	}
	file, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	original, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || !bytes.Equal(original, contents) {
		t.Fatalf("incorrect archive bytes: %q %v", original, err)
	}
	cancel()
	<-done
	teacherForReadOnly, _ := app.store.PrincipalByUserID("user-teacher")
	_, err = app.store.UpdateTeacher("管理员", p, teacherForReadOnly.UserID, learning.TeacherUpsertRequest{Name: teacherForReadOnly.Name, Phone: teacherForReadOnly.Phone, TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "english", Grade: "五年级"}}, CanDownload: true}})
	if err != nil {
		t.Fatal(err)
	}
	app.doJSON(t, http.MethodGet, "/api/material-downloads", teacherToken, nil, http.StatusOK, nil)
	app.doJSON(t, http.MethodPost, "/api/material-downloads/selection", teacherToken, scope, http.StatusOK, nil)
	var teacherJob learning.MaterialDownloadJob
	app.doJSON(t, http.MethodPost, "/api/material-downloads", teacherToken, scope, http.StatusOK, &teacherJob)
	_, _, found, claimErr := app.store.ClaimMaterialDownload()
	if claimErr != nil || !found {
		t.Fatalf("teacher claim: %v %v", found, claimErr)
	}
	archivePath := filepath.Join(root, "material-downloads", job.ID+".zip")
	if err := app.store.FinishMaterialDownload(teacherJob.ID, archivePath, ""); err != nil {
		t.Fatal(err)
	}
	pickupReq, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/material-downloads/"+teacherJob.ID+"/archive", nil)
	pickupReq.Header.Set("Authorization", "Bearer "+teacherToken)
	pickupResponse, pickupErr := app.server.Client().Do(pickupReq)
	if pickupErr != nil {
		t.Fatal(pickupErr)
	}
	_ = pickupResponse.Body.Close()
	if pickupResponse.StatusCode != http.StatusOK {
		t.Fatalf("read-only downloader rejected: %d", pickupResponse.StatusCode)
	}
	teacher, _ := app.store.PrincipalByUserID("user-teacher")
	_, err = app.store.UpdateTeacher("管理员", p, teacher.UserID, learning.TeacherUpsertRequest{Name: teacher.Name, Phone: teacher.Phone, LearningSpaceIDs: teacher.LearningSpaceIDs, TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "english", Grade: "五年级"}}, CanDownload: false}})
	if err != nil {
		t.Fatal(err)
	}
	app.doJSON(t, http.MethodGet, "/api/material-downloads/"+teacherJob.ID+"/archive", teacherToken, nil, http.StatusForbidden, nil)
	if err := os.Remove(filepath.Join(root, "material-downloads", job.ID+".zip")); err != nil {
		t.Fatal(err)
	}
	app.doJSON(t, http.MethodGet, archiveURL, adminToken, nil, http.StatusBadRequest, nil)
	var retry learning.MaterialDownloadJob
	app.doJSON(t, http.MethodPost, "/api/material-downloads/"+job.ID+"/retry", adminToken, nil, http.StatusOK, &retry)
	if retry.ID == job.ID || retry.Status != "准备中" {
		t.Fatalf("missing archive cannot regenerate: %#v", retry)
	}
	// Replacing the archive directory must not allow an old pickup link to
	// serve content from a different storage location.
	claimed, _, found, claimErr := app.store.ClaimMaterialDownload()
	if claimErr != nil || !found || claimed.ID != retry.ID {
		t.Fatalf("retry claim: %#v %v", claimed, claimErr)
	}
	retryPath := filepath.Join(root, "material-downloads", retry.ID+".zip")
	if err := os.WriteFile(retryPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := app.store.FinishMaterialDownload(retry.ID, retryPath, ""); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "material-downloads")
	if err := os.Rename(directory, directory+"-original"); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, retry.ID+".zip")
	if err := os.WriteFile(outsideFile, []byte("unrelated private archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, directory); err != nil {
		t.Fatal(err)
	}
	app.doJSON(t, http.MethodGet, "/api/material-downloads/"+retry.ID+"/archive", adminToken, nil, http.StatusBadRequest, nil)
	if body, err := os.ReadFile(outsideFile); err != nil || string(body) != "unrelated private archive" {
		t.Fatal("redirected archive modified")
	}
	for _, current := range app.store.MaterialDownloads(p) {
		if current.ID == retry.ID && current.Status != "失败" {
			t.Fatal("invalid archive remained ready")
		}
	}
}
