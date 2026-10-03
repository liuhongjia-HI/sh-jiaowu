package store

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func downloadFixture(t *testing.T) (*MemoryStore, learning.Principal, learning.MaterialDownloadScope) {
	t.Helper()
	s := NewMemoryStore()
	p, course, _ := chapterPlanFixture(t, s)
	s.materials = []learning.Material{
		{ID: "download-material-a", CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: course.Curriculum[0].ID, FileID: "download-file-a", Status: learning.StatusEnabled, PublishStatus: "已发布"},
		{ID: "download-material-b", CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: course.Curriculum[1].ID, FileID: "download-file-b", Status: learning.StatusEnabled, PublishStatus: "已发布"},
		{ID: "download-draft", CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: course.Curriculum[0].ID, FileID: "missing-draft", Status: learning.StatusEnabled, PublishStatus: "草稿"},
	}
	for _, id := range []string{"download-file-a", "download-file-b"} {
		s.fileAssets[id] = learning.FileAsset{ID: id, FileName: strings.Repeat("长文件名", 40) + ".pdf", OriginalPath: "/controlled/" + id, FileSize: 3}
	}
	return s, p, learning.MaterialDownloadScope{Subject: course.Subject, CourseIDs: []string{course.ID}}
}

func TestMaterialDownloadSelectionAndStateRecovery(t *testing.T) {
	s, p, scope := downloadFixture(t)
	quote, err := s.MaterialDownloadSelection(p, scope)
	if err != nil || quote.Count != 2 || quote.Size != 6 || len(quote.Courses) != 1 {
		t.Fatalf("bad selection: %#v %v", quote, err)
	}
	if len(s.materialDownloads) != 0 {
		t.Fatal("selection mutated jobs")
	}
	job, err := s.CreateMaterialDownload(p, scope)
	if err != nil {
		t.Fatal(err)
	}
	scope.CourseIDs[0] = "mutated"
	if s.materialDownloads[0].Scope.CourseIDs[0] == "mutated" {
		t.Fatal("request scope aliased stored task")
	}
	for _, item := range job.Items {
		if !strings.HasSuffix(item.Name, ".pdf") {
			t.Fatalf("extension lost: %s", item.Name)
		}
	}
	if job.Items[0].Name == job.Items[1].Name {
		t.Fatal("duplicate filenames collided")
	}
	raw, _ := json.Marshal(job)
	for _, secret := range []string{"ownerId", "archivePath", "items", "/controlled"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("private field leaked: %s", raw)
		}
	}
	claimed, files, found, err := s.ClaimMaterialDownload()
	if err != nil || !found || claimed.ID != job.ID || len(files) != 2 {
		t.Fatalf("claim: %#v %v %v", claimed, found, err)
	}
	if err := s.RecoverMaterialDownloads(); err != nil {
		t.Fatal(err)
	}
	if s.MaterialDownloads(p)[0].Status != "准备中" {
		t.Fatal("interrupted task not recovered")
	}
	_, _, found, err = s.ClaimMaterialDownload()
	if err != nil || !found {
		t.Fatal(err)
	}
	if err := s.FinishMaterialDownload(job.ID, "/controlled/material-downloads/test.zip", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MaterialDownloadArchive(p, job.ID); err != nil {
		t.Fatal(err)
	}
	other := p
	other.UserID = "different-user"
	if _, err := s.MaterialDownloadArchive(other, job.ID); err == nil {
		t.Fatal("other owner downloaded archive")
	}
	if len(s.MaterialDownloads(other)) != 0 {
		t.Fatal("other owner listed task")
	}
	if err := s.InvalidateMaterialDownload(other, job.ID); err == nil {
		t.Fatal("other owner invalidated task")
	}
	if err := s.InvalidateMaterialDownload(p, job.ID); err != nil {
		t.Fatal(err)
	}
	retry, err := s.RetryMaterialDownload(p, job.ID)
	if err != nil || retry.CourseIDs[0] == "mutated" {
		t.Fatalf("retry: %#v %v", retry, err)
	}
	next, err := s.CreateMaterialDownload(p, retry)
	if err != nil || next.ID == job.ID {
		t.Fatalf("regenerate: %#v %v", next, err)
	}
}

func TestMaterialDownloadRejectsPermissionChangesAndExpiredPickup(t *testing.T) {
	s, p, scope := downloadFixture(t)
	student := learning.Principal{UserID: "student", Roles: []learning.Role{learning.RoleStudent}}
	if _, err := s.MaterialDownloadSelection(student, scope); err == nil {
		t.Fatal("student downloaded teacher bulk materials")
	}
	invalid := scope
	invalid.CourseIDs = []string{"missing-course"}
	if _, err := s.MaterialDownloadSelection(p, invalid); err == nil {
		t.Fatal("silently dropped invalid explicit course")
	}
	invalid = scope
	invalid.LessonIDs = []string{"missing-lesson"}
	if _, err := s.MaterialDownloadSelection(p, invalid); err == nil {
		t.Fatal("silently dropped invalid explicit chapter")
	}
	job, err := s.CreateMaterialDownload(p, scope)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = s.ClaimMaterialDownload()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishMaterialDownload(job.ID, "/controlled/a.zip", ""); err != nil {
		t.Fatal(err)
	}
	s.materials[0].PublishStatus = "草稿"
	if _, err := s.MaterialDownloadArchive(p, job.ID); err == nil {
		t.Fatal("unpublished material still downloadable")
	}
	s.materials[0].PublishStatus = "已发布"
	s.materialDownloads[0].ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	if s.MaterialDownloads(p)[0].Status != "已过期" {
		t.Fatal("expired task advertised ready")
	}
	if _, err := s.MaterialDownloadArchive(p, job.ID); err == nil {
		t.Fatal("expired archive downloadable")
	}
	if _, err := s.RetryMaterialDownload(p, job.ID); err != nil {
		t.Fatal(err)
	}
	paths, err := s.ExpiredMaterialArchives()
	if err != nil || len(paths) != 1 {
		t.Fatalf("cleanup: %#v %v", paths, err)
	}
	// Failed or interrupted filesystem cleanup must keep the path for retry.
	again, err := s.ExpiredMaterialArchives()
	if err != nil || len(again) != 1 {
		t.Fatal("cleanup path lost before deletion")
	}
	if err := s.AcknowledgeMaterialArchiveRemoval(paths[0]); err != nil {
		t.Fatal(err)
	}
	paths, err = s.ExpiredMaterialArchives()
	if err != nil || len(paths) != 0 {
		t.Fatal("cleanup not idempotent")
	}
}

func TestMaterialDownloadQueueLimitAndFreshClaimCheck(t *testing.T) {
	s, p, scope := downloadFixture(t)
	for i := 0; i < 3; i++ {
		if _, err := s.CreateMaterialDownload(p, scope); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateMaterialDownload(p, scope); err == nil {
		t.Fatal("queue limit bypassed")
	}
	s.materials[0].FileID = "changed-file"
	_, _, found, err := s.ClaimMaterialDownload()
	if err != nil || found {
		t.Fatalf("changed file claimed: %v %v", found, err)
	}
	for _, job := range s.MaterialDownloads(p) {
		if job.Status != "失败" {
			t.Fatalf("changed task not failed: %#v", job)
		}
	}
}

func TestMaterialDownloadMySQLReloadAndInterruptedRecovery(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	if !strings.Contains(dsn, "/starline_download_cleanup_test?") {
		t.Fatal("requires dedicated starline_download_cleanup_test database")
	}
	s := NewMemoryStore()
	for i := range s.grants {
		s.grants[i].OpenedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	p, course, _ := chapterPlanFixture(t, s)
	material, err := s.CreateMaterial("管理员", p, learning.MaterialUploadRequest{Title: "持久化下载讲义", CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: course.Curriculum[0].ID, File: learning.FileAsset{ID: "download-persistence-file", FileName: "讲义.pdf", OriginalPath: "/controlled/download-source.pdf", FileSize: 7}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateMaterial("管理员", p, material.ID, learning.MaterialUpdateRequest{Title: material.Title, CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: material.LessonID, Status: learning.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	job, err := s.CreateMaterialDownload(p, learning.MaterialDownloadScope{Subject: course.Subject, CourseIDs: []string{course.ID}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, found, err := s.ClaimMaterialDownload()
	if err != nil || !found {
		t.Fatalf("claim: %v %v", found, err)
	}
	restart := NewMemoryStore()
	if err := restart.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer restart.db.Close()
	jobs := restart.MaterialDownloads(p)
	if len(jobs) != 1 || jobs[0].ID != job.ID || jobs[0].Status != "打包中" {
		t.Fatalf("task not persisted: %#v", jobs)
	}
	if err := restart.RecoverMaterialDownloads(); err != nil {
		t.Fatal(err)
	}
	claimed, files, found, err := restart.ClaimMaterialDownload()
	if err != nil || !found || claimed.ID != job.ID || len(files) != 1 || files[0].Path != "/controlled/download-source.pdf" {
		t.Fatalf("recovery lost private manifest: %#v %#v %v", claimed, files, err)
	}
	archive := "/controlled/material-downloads/persisted.zip"
	if err := restart.FinishMaterialDownload(job.ID, archive, ""); err != nil {
		t.Fatal(err)
	}
	if err := restart.loadMaterialDownloadsFromDB(); err != nil {
		t.Fatal(err)
	}
	actual, err := restart.MaterialDownloadArchive(p, job.ID)
	if err != nil || actual != archive {
		t.Fatalf("ready archive not persisted: %q %v", actual, err)
	}
	if err := restart.InvalidateMaterialDownload(p, job.ID); err != nil {
		t.Fatal(err)
	}
	if err := restart.loadMaterialDownloadsFromDB(); err != nil {
		t.Fatal(err)
	}
	if restart.MaterialDownloads(p)[0].Status != "失败" {
		t.Fatal("invalidated task state lost")
	}
	if _, err := restart.db.Exec(`CREATE TRIGGER starline_cleanup_ack_fail BEFORE INSERT ON material_download_jobs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'forced cleanup acknowledgement failure'`); err != nil {
		t.Fatal(err)
	}
	defer restart.db.Exec(`DROP TRIGGER IF EXISTS starline_cleanup_ack_fail`)
	if err := restart.AcknowledgeMaterialArchiveRemoval(archive); err == nil {
		t.Fatal("cleanup acknowledgement ignored database failure")
	}
	if restart.materialDownloads[0].ArchivePath != archive {
		t.Fatal("failed acknowledgement cleared memory path")
	}
	if err := restart.loadMaterialDownloadsFromDB(); err != nil {
		t.Fatal(err)
	}
	if restart.materialDownloads[0].ArchivePath != archive {
		t.Fatal("failed acknowledgement cleared persisted path")
	}
	if _, err := restart.db.Exec(`DROP TRIGGER starline_cleanup_ack_fail`); err != nil {
		t.Fatal(err)
	}
	if err := restart.AcknowledgeMaterialArchiveRemoval(archive); err != nil {
		t.Fatal(err)
	}
	if err := restart.loadMaterialDownloadsFromDB(); err != nil {
		t.Fatal(err)
	}
	if restart.materialDownloads[0].ArchivePath != "" {
		t.Fatal("archive cleanup not persisted")
	}
}

func TestMaterialDownloadConcurrentQueueAndClaim(t *testing.T) {
	s, p, scope := downloadFixture(t)
	start := make(chan struct{})
	accepted := make(chan learning.MaterialDownloadJob, 48)
	rejected := make(chan error, 48)
	var wg sync.WaitGroup
	for i := 0; i < 48; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			job, err := s.CreateMaterialDownload(p, scope)
			if err == nil {
				accepted <- job
			} else {
				rejected <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(accepted)
	close(rejected)
	for err := range rejected {
		if err.Error() != "已有下载任务正在准备，请完成后再创建" {
			t.Fatalf("unexpected creation failure: %v", err)
		}
	}
	ids := map[string]bool{}
	for job := range accepted {
		if ids[job.ID] {
			t.Fatal("duplicate job ID")
		}
		ids[job.ID] = true
	}
	if len(ids) != 3 || len(s.MaterialDownloads(p)) != 3 {
		t.Fatalf("concurrent requests bypassed limit: %d", len(ids))
	}
	claims := make(chan learning.MaterialDownloadJob, 24)
	failures := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job, _, found, err := s.ClaimMaterialDownload()
			if err != nil {
				failures <- err
			}
			if found {
				claims <- job
			}
		}()
	}
	wg.Wait()
	close(claims)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	claimed := map[string]bool{}
	for job := range claims {
		if claimed[job.ID] || !ids[job.ID] {
			t.Fatal("task claimed twice or unknown task")
		}
		claimed[job.ID] = true
	}
	if len(claimed) != 3 {
		t.Fatalf("claim count %d", len(claimed))
	}
	if _, err := s.CreateMaterialDownload(p, scope); err == nil {
		t.Fatal("packing tasks did not count toward limit")
	}
	for id := range claimed {
		if err := s.FinishMaterialDownload(id, "", "controlled failure"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := s.CreateMaterialDownload(p, scope); err != nil {
			t.Fatal("failed tasks retained queue slots", err)
		}
	}
	if len(s.MaterialDownloads(p)) != 6 {
		t.Fatal("failed task history lost")
	}
}

func TestMaterialDownloadSelectionSizeAndCountBoundaries(t *testing.T) {
	s, p, scope := downloadFixture(t)
	original := s.materials[0]
	s.materials = make([]learning.Material, 2000)
	for i := range s.materials {
		s.materials[i] = original
		s.materials[i].ID = fmt.Sprintf("bulk-material-%04d", i)
	}
	q, err := s.MaterialDownloadSelection(p, scope)
	if err != nil || q.Count != 2000 {
		t.Fatalf("2000 selection: %#v %v", q, err)
	}
	extra := original
	extra.ID = "bulk-over-limit"
	s.materials = append(s.materials, extra)
	if _, err := s.CreateMaterialDownload(p, scope); err == nil {
		t.Fatal("2001 files accepted")
	}
	if len(s.materialDownloads) != 0 {
		t.Fatal("rejected count created job")
	}
	s.materials = []learning.Material{original}
	asset := s.fileAssets[original.FileID]
	asset.FileSize = 2 * 1024 * 1024 * 1024
	s.fileAssets[original.FileID] = asset
	q, err = s.MaterialDownloadSelection(p, scope)
	if err != nil || q.Size != asset.FileSize {
		t.Fatalf("2GiB metadata selection: %#v %v", q, err)
	}
	asset.FileSize++
	s.fileAssets[original.FileID] = asset
	if _, err := s.CreateMaterialDownload(p, scope); err == nil {
		t.Fatal("over 2GiB accepted")
	}
	if len(s.materialDownloads) != 0 {
		t.Fatal("rejected size created job")
	}
}
