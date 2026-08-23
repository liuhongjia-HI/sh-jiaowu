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
