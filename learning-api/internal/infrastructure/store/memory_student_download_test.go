package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestStudentMaterialDownloadURLOnlyExistsDuringActiveGrant(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	for index := range store.materials {
		if store.materials[index].ID == "mat-g05-english-s1-q1" {
			store.materials[index].FileID = "file-student-download"
			store.materials[index].FileName = "lesson.pdf"
			break
		}
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "downloadPolicy", Value: "允许下载带水印PDF"}); err != nil {
		t.Fatalf("enable watermarked download: %v", err)
	}
	material, err := store.StudentMaterial(principal, "mat-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("expected active material access: %v", err)
	}
	if material.DownloadURL != "/api/student/materials/mat-g05-english-s1-q1/download" {
		t.Fatalf("expected student download url, got %q", material.DownloadURL)
	}
	for index := range store.grants {
		if store.grants[index].StudentID == principal.StudentID {
			store.grants[index].EndsAt = "2000-01-01"
			store.grants[index].EffectiveUntil = "2000-01-01"
		}
	}
	if _, err := store.StudentMaterial(principal, "mat-g05-english-s1-q1"); err == nil {
		t.Fatal("expected download source material to become inaccessible after grant expiry")
	}
}

func TestStudentPreviewFileIncludesServerRenderableWatermarkTrace(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	for index := range store.materials {
		if store.materials[index].ID == "mat-g05-english-s1-q1" {
			store.materials[index].FileID = "file-watermark-trace"
			break
		}
	}
	store.fileAssets["file-watermark-trace"] = learning.FileAsset{
		ID: "file-watermark-trace", PreviewPath: "preview.pdf", PreviewStatus: "可预览",
	}

	asset, err := store.StudentMaterialPreviewFile(principal, "mat-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("expected preview file: %v", err)
	}
	for _, expected := range []string{"STARLINE", "P-9069", "T-"} {
		if !strings.Contains(asset.WatermarkStampText, expected) {
			t.Fatalf("expected server-renderable trace %q, got %q", expected, asset.WatermarkStampText)
		}
	}
}

func TestStudentPreviewFileReturnsAuthorizedAssetWhilePreviewIsProcessing(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	for index := range store.materials {
		if store.materials[index].ID == "mat-g05-english-s1-q1" {
			store.materials[index].FileID = "file-preview-processing"
			break
		}
	}
	store.fileAssets["file-preview-processing"] = learning.FileAsset{
		ID: "file-preview-processing", PreviewStatus: "待转换",
	}

	asset, err := store.StudentMaterialPreviewFile(principal, "mat-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("authorized processing asset should be returned for status reporting: %v", err)
	}
	if asset.PreviewStatus != "待转换" {
		t.Fatalf("preview status = %q, want 待转换", asset.PreviewStatus)
	}
}

func TestStudentMaterialHidesDownloadWhenPolicyIsOnlinePreviewOnly(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	for index := range store.materials {
		if store.materials[index].ID == "mat-g05-english-s1-q1" {
			store.materials[index].FileID = "file-online-preview-only"
			break
		}
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "downloadPolicy", Value: "仅在线预览"}); err != nil {
		t.Fatalf("set online preview policy: %v", err)
	}

	material, err := store.StudentMaterial(principal, "mat-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("expected active material access: %v", err)
	}
	if material.DownloadURL != "" {
		t.Fatalf("online preview policy must hide student download URL, got %q", material.DownloadURL)
	}
}
