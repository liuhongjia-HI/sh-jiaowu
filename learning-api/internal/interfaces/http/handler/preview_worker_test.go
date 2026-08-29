package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestPreviewWorkerKeepsPDFPreviewWhenGhostscriptMissing(t *testing.T) {
	withEmptyPath(t)
	root := t.TempDir()
	originalDir := filepath.Join(root, "original")
	if err := os.MkdirAll(originalDir, 0755); err != nil {
		t.Fatalf("mkdir original: %v", err)
	}
	originalPath := filepath.Join(originalDir, "lesson.pdf")
	if err := os.WriteFile(originalPath, []byte("%PDF-1.4 preview"), 0644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	worker := NewPreviewWorker(nil, root)
	result, err := worker.generate(context.Background(), learning.FileAsset{ID: "file-pdf", OriginalPath: originalPath})
	if err != nil {
		t.Fatalf("generate preview: %v", err)
	}
	if result.PreviewPath == "" || result.PreviewPageCount != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !strings.Contains(result.PreviewWarning, "Ghostscript") {
		t.Fatalf("warning = %q", result.PreviewWarning)
	}
}

func TestPreviewWorkerBackfillsPagesFromExistingPreviewWhenOriginalIsMissing(t *testing.T) {
	withEmptyPath(t)
	root := t.TempDir()
	previewDir := filepath.Join(root, "preview")
	if err := os.MkdirAll(previewDir, 0755); err != nil {
		t.Fatalf("mkdir preview: %v", err)
	}
	previewPath := filepath.Join(previewDir, "existing.pdf")
	if err := os.WriteFile(previewPath, []byte("%PDF-1.4 existing preview"), 0644); err != nil {
		t.Fatalf("write preview: %v", err)
	}

	worker := NewPreviewWorker(nil, root)
	result, err := worker.generate(context.Background(), learning.FileAsset{
		ID:            "file-pdf-only",
		OriginalPath:  filepath.Join(root, "original", "missing.pdf"),
		PreviewPath:   previewPath,
		PreviewStatus: "可预览",
	})
	if err != nil {
		t.Fatalf("backfill existing preview: %v", err)
	}
	if result.PreviewPath != previewPath {
		t.Fatalf("preview path = %q, want %q", result.PreviewPath, previewPath)
	}
	if !strings.Contains(result.PreviewWarning, "Ghostscript") {
		t.Fatalf("warning = %q", result.PreviewWarning)
	}
}
