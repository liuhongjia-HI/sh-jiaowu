package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWatermarkPageScriptStampsBeforeEveryPageOutput(t *testing.T) {
	script := watermarkPageScript("STARLINE | U-001 | O'Reilly (9069)\\path")

	for _, expected := range []string{
		"STARLINE | U-001 | O'Reilly \\(9069\\)\\\\path",
		"/OriginalShowpage /showpage load def",
		"/showpage { StarlineWatermark OriginalShowpage } bind def",
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
	if strings.Contains(script, "/EndPage") {
		t.Fatalf("watermark script must not use EndPage because PDF page setup resets it: %s", script)
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
