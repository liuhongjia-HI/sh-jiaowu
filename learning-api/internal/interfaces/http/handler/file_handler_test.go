package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestStudentPreviewPagesMetadataUsesStructuredStatuses(t *testing.T) {
	tests := []struct {
		name      string
		asset     learning.FileAsset
		wantState string
		wantImage bool
	}{
		{name: "processing", asset: learning.FileAsset{PreviewStatus: "待转换"}, wantState: "processing"},
		{name: "failed", asset: learning.FileAsset{PreviewStatus: "转换失败"}, wantState: "failed"},
		{name: "pdf fallback", asset: learning.FileAsset{PreviewStatus: "可预览", PreviewPath: "preview.pdf"}, wantState: "ready"},
		{name: "paged image", asset: learning.FileAsset{PreviewStatus: "可预览", PreviewPath: "preview.pdf", PreviewPageDir: "pages", PreviewPageCount: 3}, wantState: "ready", wantImage: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := studentPreviewPagesMetadata(tt.asset)
			if got.PreviewStatus != tt.wantState || got.ImageMode != tt.wantImage {
				t.Fatalf("metadata = %#v, want state=%q imageMode=%v", got, tt.wantState, tt.wantImage)
			}
		})
	}
}

func TestStudentPreviewPagesMetadataUsesExistingPDFWhileBackfillIsPending(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	gsPath := filepath.Join(binDir, "gs")
	if err := os.WriteFile(gsPath, []byte("#!/bin/sh\necho 4\n"), 0755); err != nil {
		t.Fatalf("write fake gs: %v", err)
	}
	t.Setenv("PATH", binDir)
	previewPath := filepath.Join(root, "preview.pdf")
	if err := os.WriteFile(previewPath, []byte("%PDF-1.4 existing preview"), 0644); err != nil {
		t.Fatalf("write preview: %v", err)
	}

	got := studentPreviewPagesMetadataWithPDFFallback(context.Background(), learning.FileAsset{
		PreviewStatus: "可预览",
		PreviewPath:   previewPath,
	})

	if got.PreviewStatus != "ready" || !got.ImageMode || got.PageCount != 4 {
		t.Fatalf("metadata = %#v, want ready image mode with 4 pages", got)
	}
}

func TestBuildPreviewNeverReusesOriginalPathForPDF(t *testing.T) {
	dir := t.TempDir()
	originalDir := filepath.Join(dir, "original")
	previewDir := filepath.Join(dir, "preview")
	if err := os.MkdirAll(originalDir, 0755); err != nil {
		t.Fatalf("mkdir original: %v", err)
	}
	if err := os.MkdirAll(previewDir, 0755); err != nil {
		t.Fatalf("mkdir preview: %v", err)
	}
	originalPath := filepath.Join(originalDir, "file-001.pdf")
	want := []byte("%PDF-1.4 original bytes")
	if err := os.WriteFile(originalPath, want, 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	previewPath, err := buildPreview(context.Background(), originalPath, previewDir, ".pdf")

	if err != nil {
		t.Fatalf("build preview: %v", err)
	}
	if previewPath == originalPath {
		t.Fatalf("previewPath must never equal originalPath, got the same path %q", previewPath)
	}
	if !filepathHasPrefix(previewPath, previewDir) {
		t.Fatalf("previewPath %q should live under previewDir %q", previewPath, previewDir)
	}
	got, err := os.ReadFile(previewPath)
	if err != nil {
		t.Fatalf("read preview copy: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("preview copy content = %q, want %q", got, want)
	}

	// 修改原文件不应影响已经生成的预览副本——两者是物理独立的文件。
	if err := os.WriteFile(originalPath, []byte("mutated"), 0644); err != nil {
		t.Fatalf("mutate original: %v", err)
	}
	stillOriginalCopy, err := os.ReadFile(previewPath)
	if err != nil {
		t.Fatalf("re-read preview copy: %v", err)
	}
	if string(stillOriginalCopy) != string(want) {
		t.Fatalf("preview copy changed after mutating original, physical separation is broken")
	}
}

func TestBuildPreviewExplainsMissingLibreOffice(t *testing.T) {
	withEmptyPath(t)
	_, err := buildPreview(context.Background(), "/tmp/lesson.docx", t.TempDir(), ".docx")
	if err == nil || err.Error() != "服务器未安装 LibreOffice，无法转换 Word/PPT" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func filepathHasPrefix(path, prefix string) bool {
	rel, err := filepath.Rel(prefix, path)
	if err != nil {
		return false
	}
	return rel != ".." && len(rel) > 0 && rel[0] != '.'
}
