package handler

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
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

func TestPreviewWorkerRealGhostscriptRejectsThreeHundredOnePages(t *testing.T) {
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("actual Ghostscript required")
	}
	root := t.TempDir()
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	writeObject := func(id int, body string) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", id, body)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	kids := ""
	for i := 0; i < 301; i++ {
		kids += fmt.Sprintf("%d 0 R ", i+3)
	}
	writeObject(2, "<< /Type /Pages /Count 301 /Kids ["+kids+"] >>")
	for i := 0; i < 301; i++ {
		writeObject(i+3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> >>")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	source := filepath.Join(root, "many-pages.pdf")
	if err := os.WriteFile(source, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := NewPreviewWorker(nil, root).generate(context.Background(), learning.FileAsset{ID: "file-page-limit", OriginalPath: source})
	if err == nil || !strings.Contains(err.Error(), "301") || !strings.Contains(err.Error(), "300") || result.PreviewPath != "" {
		t.Fatalf("page limit ignored: %#v %v", result, err)
	}
	original, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(original, out.Bytes()) {
		t.Fatal("page limit changed original", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "pages", "file-page-limit"))
	if err != nil || len(entries) != 0 {
		t.Fatal("page limit rasterized pages before rejection", err)
	}
}
