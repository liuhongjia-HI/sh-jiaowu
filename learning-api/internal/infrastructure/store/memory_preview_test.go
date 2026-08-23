package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestPreviewJobLifecycleUpdatesFileAndMaterial(t *testing.T) {
	store := NewMemoryStore()
	store.fileAssets["file-preview"] = learning.FileAsset{ID: "file-preview", FileName: "lesson.pdf", PreviewStatus: "待转换"}
	store.materials = append(store.materials, learning.Material{ID: "material-preview", Course: store.courses[0].Name, CourseID: store.courses[0].ID, LearningSpaceID: store.courses[0].LearningSpaceID, FileID: "file-preview", PreviewStatus: "待转换"})
	store.enqueuePreviewJobUnlocked("file-preview")

	job, ok, err := store.ClaimPreviewJob()
	if err != nil || !ok {
		t.Fatalf("claim preview job: ok=%v err=%v", ok, err)
	}
	if job.Status != "处理中" || job.AttemptCount != 1 {
		t.Fatalf("claimed job = %#v", job)
	}
	result := learning.PreviewResult{PreviewPath: "/data/preview/file-preview.pdf", PreviewPageDir: "/data/pages/file-preview", PreviewPageCount: 4, PreviewWarning: "分页图片暂不可用"}
	if err := store.CompletePreviewJob(job.ID, result); err != nil {
		t.Fatalf("complete preview job: %v", err)
	}
	asset := store.fileAssets["file-preview"]
	if asset.PreviewStatus != "可预览" || asset.PreviewPageCount != 4 || asset.PreviewPageDir != result.PreviewPageDir {
		t.Fatalf("completed asset = %#v", asset)
	}
	if asset.PreviewError != result.PreviewWarning {
		t.Fatalf("preview warning = %q", asset.PreviewError)
	}
	if got := store.materials[len(store.materials)-1].PreviewStatus; got != "可预览" {
		t.Fatalf("material preview status = %q", got)
	}
}

func TestPreviewJobRetriesThreeTimesAndCanBeReset(t *testing.T) {
	store := NewMemoryStore()
	store.fileAssets["file-retry"] = learning.FileAsset{ID: "file-retry", FileName: "retry.pdf", PreviewStatus: "待转换"}
	store.materials = append(store.materials, learning.Material{ID: "material-retry", Course: store.courses[0].Name, CourseID: store.courses[0].ID, LearningSpaceID: store.courses[0].LearningSpaceID, FileID: "file-retry", PreviewStatus: "待转换"})
	store.enqueuePreviewJobUnlocked("file-retry")

	var job learning.PreviewJob
	for attempt := 1; attempt <= maxPreviewAttempts; attempt++ {
		claimed, ok, err := store.ClaimPreviewJob()
		if err != nil || !ok {
			t.Fatalf("claim attempt %d: ok=%v err=%v", attempt, ok, err)
		}
		job = claimed
		if err := store.FailPreviewJob(job.ID, "ghostscript failed"); err != nil {
			t.Fatalf("fail attempt %d: %v", attempt, err)
		}
	}
	if got := store.fileAssets["file-retry"].PreviewStatus; got != "转换失败" {
		t.Fatalf("asset preview status = %q", got)
	}
	principal := learning.Principal{UserID: "user-super", Roles: []learning.Role{learning.RoleSuperAdmin}}
	if err := store.RetryPreviewJob("超级管理员", principal, "file-retry"); err != nil {
		t.Fatalf("manual retry: %v", err)
	}
	retried, ok, err := store.ClaimPreviewJob()
	if err != nil || !ok || retried.AttemptCount != 1 {
		t.Fatalf("claim after manual retry: job=%#v ok=%v err=%v", retried, ok, err)
	}
}

func TestRecoverPreviewJobsReturnsInterruptedWorkToQueue(t *testing.T) {
	store := NewMemoryStore()
	store.fileAssets["file-recover"] = learning.FileAsset{ID: "file-recover"}
	store.previewJobs = []learning.PreviewJob{{ID: "job-recover", FileID: "file-recover", Status: "处理中", AttemptCount: 1, StartedAt: "2026-08-16 10:00:00"}}
	if err := store.RecoverPreviewJobs(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if got := store.previewJobs[0]; got.Status != "待处理" || got.StartedAt != "" {
		t.Fatalf("recovered job = %#v", got)
	}
}

func TestMarkPreviewFileMissingReopensRetryForLostPreview(t *testing.T) {
	store := NewMemoryStore()
	store.fileAssets["file-lost"] = learning.FileAsset{ID: "file-lost", FileName: "lost.pdf", PreviewStatus: "可预览", PreviewPath: "/opt/starline/releases/old/uploads/preview/lost.pdf", PreviewPageDir: "/opt/starline/releases/old/uploads/pages/file-lost", PreviewPageCount: 3}
	store.materials = append(store.materials, learning.Material{ID: "material-lost", Course: store.courses[0].Name, CourseID: store.courses[0].ID, LearningSpaceID: store.courses[0].LearningSpaceID, FileID: "file-lost", PreviewStatus: "可预览"})

	if err := store.MarkPreviewFileMissing("file-lost", "预览文件已丢失，请重新生成预览"); err != nil {
		t.Fatalf("mark preview file missing: %v", err)
	}
	asset := store.fileAssets["file-lost"]
	if asset.PreviewStatus != "转换失败" || asset.PreviewPath != "" || asset.PreviewPageCount != 0 {
		t.Fatalf("asset after mark = %#v", asset)
	}
	if got := store.materials[len(store.materials)-1].PreviewStatus; got != "转换失败" {
		t.Fatalf("material preview status = %q", got)
	}

	// 早期课件没有预览任务，标记时补建的失败任务必须能被重试入口重置。
	principal := learning.Principal{UserID: "user-super", Roles: []learning.Role{learning.RoleSuperAdmin}}
	if err := store.RetryPreviewJob("超级管理员", principal, "file-lost"); err != nil {
		t.Fatalf("manual retry after mark: %v", err)
	}
	claimed, ok, err := store.ClaimPreviewJob()
	if err != nil || !ok || claimed.FileID != "file-lost" {
		t.Fatalf("claim after retry: job=%#v ok=%v err=%v", claimed, ok, err)
	}
}
