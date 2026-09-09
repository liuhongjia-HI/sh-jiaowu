package store

import (
	"regexp"
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
			store.materials[index].AllowDownload = true
			store.materials[index].FileName = "lesson.pdf"
			break
		}
	}
	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        principal.StudentID,
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"course"},
		StartsAt:         "2026-01-01",
		EndsAt:           "2027-12-31",
	}); err != nil {
		t.Fatalf("enable secure course download: %v", err)
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

func TestStudentHomeworkDownloadURLOnlyExistsDuringActiveGrant(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	for index := range store.homework {
		if store.homework[index].ID == "hw-g05-english-s1-q1" {
			store.homework[index].FileID = "file-student-homework-download"
			store.homework[index].FileName = "homework.pdf"
			break
		}
	}
	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        principal.StudentID,
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"course"},
		StartsAt:         "2026-01-01",
		EndsAt:           "2027-12-31",
	}); err != nil {
		t.Fatalf("enable secure homework download: %v", err)
	}
	homework, err := store.StudentHomework(principal, "hw-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("expected active homework access: %v", err)
	}
	if homework.DownloadURL != "/api/student/homework/hw-g05-english-s1-q1/download" {
		t.Fatalf("expected student homework download url, got %q", homework.DownloadURL)
	}
	for index := range store.grants {
		if store.grants[index].StudentID == principal.StudentID {
			store.grants[index].EndsAt = "2000-01-01"
			store.grants[index].EffectiveUntil = "2000-01-01"
		}
	}
	if _, err := store.StudentHomework(principal, "hw-g05-english-s1-q1"); err == nil {
		t.Fatal("expected homework source to become inaccessible after grant expiry")
	}
}

func TestStudentHomeworkPreviewFileIncludesServerRenderableWatermarkTrace(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	for index := range store.homework {
		if store.homework[index].ID == "hw-g05-english-s1-q1" {
			store.homework[index].FileID = "file-homework-watermark-trace"
			break
		}
	}
	store.fileAssets["file-homework-watermark-trace"] = learning.FileAsset{
		ID: "file-homework-watermark-trace", PreviewPath: "preview.pdf", PreviewStatus: "可预览",
	}

	asset, err := store.StudentHomeworkPreviewFile(principal, "hw-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("expected homework preview file: %v", err)
	}
	for _, expected := range []string{"小明", "STARLINE"} {
		if !strings.Contains(asset.WatermarkStampText, expected) {
			t.Fatalf("expected server-renderable trace %q, got %q", expected, asset.WatermarkStampText)
		}
	}
	if strings.Contains(asset.WatermarkStampText, "P-") || strings.Contains(asset.WatermarkStampText, "T-") {
		t.Fatalf("visible watermark should not contain trace fields, got %q", asset.WatermarkStampText)
	}
	if !regexp.MustCompile(`^小明 STARLINE \d{4}-\d{2}-\d{2}$`).MatchString(asset.WatermarkStampText) {
		t.Fatalf("visible watermark should contain name, STARLINE, and date, got %q", asset.WatermarkStampText)
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
	for _, expected := range []string{"STARLINE"} {
		if !strings.Contains(asset.WatermarkStampText, expected) {
			t.Fatalf("expected server-renderable trace %q, got %q", expected, asset.WatermarkStampText)
		}
	}
	if strings.Contains(asset.WatermarkStampText, "P-") || strings.Contains(asset.WatermarkStampText, "T-") {
		t.Fatalf("visible watermark should not contain trace fields, got %q", asset.WatermarkStampText)
	}
	if !regexp.MustCompile(`^小明 STARLINE \d{4}-\d{2}-\d{2}$`).MatchString(asset.WatermarkStampText) {
		t.Fatalf("visible watermark should contain name, STARLINE, and date, got %q", asset.WatermarkStampText)
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

func TestStudentMaterialHidesDownloadWhenOnlyHandoutPermissionExists(t *testing.T) {
	store := NewMemoryStore()
	store.grants = nil
	store.spaceAccess = nil
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
	if _, err := store.CreateDirectGrant("运营教务", learning.DirectGrantCreateRequest{
		StudentID:        principal.StudentID,
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"handout"},
		StartsAt:         "2026-01-01",
		EndsAt:           "2027-12-31",
	}); err != nil {
		t.Fatalf("set handout-only grant: %v", err)
	}

	material, err := store.StudentMaterial(principal, "mat-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("expected active material access: %v", err)
	}
	if material.DownloadURL != "" {
		t.Fatalf("handout-only grant must hide student download URL, got %q", material.DownloadURL)
	}
}

func TestStudentMaterialDownloadRecomputesPermission(t *testing.T) {
	for _, tc := range []struct {
		name    string
		allow   bool
		grant   bool
		expired bool
		file    bool
		want    bool
	}{
		{"authorized", true, true, false, true, true},
		{"material disallows", false, true, false, true, false},
		{"no grant", true, false, false, true, false},
		{"expired", true, true, true, true, false},
		{"missing file", true, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := NewMemoryStore()
			store.grants = nil
			store.spaceAccess = nil
			principal, err := store.PrincipalByUserID("user-student-001")
			if err != nil {
				t.Fatal(err)
			}
			if tc.grant {
				_, err = store.CreateDirectGrant("测试", learning.DirectGrantCreateRequest{
					StudentID: principal.StudentID, LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
					ContentTypeCodes: []string{"course"}, StartsAt: "2026-01-01", EndsAt: "2099-12-31",
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.expired {
				for i := range store.grants {
					store.grants[i].EndsAt = "2000-01-01"
					store.grants[i].EffectiveUntil = "2000-01-01"
				}
			}
			material := learning.Material{ID: "permission-check", LearningSpaceID: "space-g05-english-s1-q1", AllowDownload: tc.allow, DownloadURL: "https://old.example/unsafe.pdf"}
			if tc.file {
				material.FileID = "file"
			}
			result := store.decorateStudentMaterial(principal, material)
			if (result.DownloadURL != "") != tc.want {
				t.Fatalf("download URL = %q, want allowed=%v", result.DownloadURL, tc.want)
			}
			if tc.want && result.DownloadURL != "/api/student/materials/permission-check/download" {
				t.Fatalf("must use secure student route: %q", result.DownloadURL)
			}
		})
	}
}
