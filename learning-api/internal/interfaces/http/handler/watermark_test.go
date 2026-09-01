package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWatermarkBeginPageScriptDrawsBehindContent(t *testing.T) {
	script := watermarkPageScript("STARLINE | U-001 | O'Reilly (9069)\\path")

	for _, expected := range []string{
		"STARLINE | U-001 | O'Reilly \\(9069\\)\\\\path",
		"<< /BeginPage {",
		"pop\n    StarlineWatermark",
		"initgraphics",
		"clippath pathbbox",
		"stringwidth",
		"rotate",
		"show",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("watermark script should contain %q, got %s", expected, script)
		}
	}
	if strings.Contains(script, "/EndPage") || strings.Contains(script, "/showpage") {
		t.Fatalf("watermark must draw before the PDF content instead of overriding page output: %s", script)
	}
	if !strings.Contains(script, "0.93 setgray") {
		t.Fatalf("watermark should use the lighter 7%% black shade, got %s", script)
	}
	if count := strings.Count(script, "WatermarkText show"); count != 2 {
		t.Fatalf("watermark count = %d, want 2 to avoid obstructing courseware text", count)
	}
}

func TestCountPDFPagesPermitsGhostscriptToReadSourcePDF(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	previewPath := filepath.Join(root, "preview.pdf")
	if err := os.WriteFile(previewPath, []byte("%PDF-1.4 fixture"), 0644); err != nil {
		t.Fatalf("write preview: %v", err)
	}
	gsScript := "#!/bin/sh\n" +
		"for arg in \"$@\"; do\n" +
		"  if [ \"$arg\" = \"--permit-file-read=" + previewPath + "\" ]; then\n" +
		"    echo 4\n" +
		"    exit 0\n" +
		"  fi\n" +
		"done\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "gs"), []byte(gsScript), 0755); err != nil {
		t.Fatalf("write fake gs: %v", err)
	}
	t.Setenv("PATH", binDir)

	pageCount, err := countPDFPages(context.Background(), previewPath)
	if err != nil {
		t.Fatalf("count PDF pages: %v", err)
	}
	if pageCount != 4 {
		t.Fatalf("page count = %d, want 4", pageCount)
	}
}

func TestRasterizePDFPageDegradesWhenGhostscriptMissing(t *testing.T) {
	withEmptyPath(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(src, []byte("%PDF-1.4 fake"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	target := filepath.Join(dir, "page-1.jpg")

	err := rasterizePDFPage(context.Background(), src, target, 1)
	if err != errGhostscriptUnavailable {
		t.Fatalf("expected errGhostscriptUnavailable, got %v", err)
	}
}

func TestCountPDFPagesDegradesWhenGhostscriptMissing(t *testing.T) {
	withEmptyPath(t)
	_, err := countPDFPages(context.Background(), "irrelevant.pdf")
	if err != errGhostscriptUnavailable {
		t.Fatalf("expected errGhostscriptUnavailable, got %v", err)
	}
}

// withEmptyPath 清空 PATH，确保 exec.LookPath("gs") 在任何机器上都必然找不到，
// 从而稳定地测试“未安装 Ghostscript”这条降级路径，不受本机是否装了 gs 影响。
func withEmptyPath(t *testing.T) {
	t.Helper()
	original := os.Getenv("PATH")
	os.Setenv("PATH", "")
	t.Cleanup(func() { os.Setenv("PATH", original) })
}
