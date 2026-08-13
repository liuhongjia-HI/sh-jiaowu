package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEscapePostScriptStringPreventsBreakingOutOfLiteral(t *testing.T) {
	cases := map[string]string{
		"张三 · 1234 · 2026-01-01": "张三 · 1234 · 2026-01-01",
		"a(b)c":                  `a\(b\)c`,
		`back\slash`:             `back\\slash`,
		"注入) show (evil":         `注入\) show \(evil`,
	}
	for input, want := range cases {
		if got := escapePostScriptString(input); got != want {
			t.Fatalf("escapePostScriptString(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWatermarkPostScriptEmbedsEscapedTextOnly(t *testing.T) {
	script := watermarkPostScript("学员(测试) · 1234")
	if strings.Contains(script, "学员(测试)") {
		t.Fatalf("unescaped watermark text leaked into PostScript, would break out of string literal:\n%s", script)
	}
	if !strings.Contains(script, `学员\(测试\)`) {
		t.Fatalf("expected escaped watermark text in script:\n%s", script)
	}
	if !strings.Contains(script, "/BeginPage") {
		t.Fatalf("watermark script must hook BeginPage to stamp every page:\n%s", script)
	}
}

func TestStampWatermarkPDFDegradesWhenGhostscriptMissing(t *testing.T) {
	withEmptyPath(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(src, []byte("%PDF-1.4 fake"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	target := filepath.Join(dir, "out.pdf")

	err := stampWatermarkPDF(context.Background(), src, target, "学员 · 1234")
	if err != errGhostscriptUnavailable {
		t.Fatalf("expected errGhostscriptUnavailable, got %v", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatalf("no output file should be produced when ghostscript is unavailable")
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
