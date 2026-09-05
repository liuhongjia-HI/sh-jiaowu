package handler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWatermarkBeginPageScriptDrawsBehindContent(t *testing.T) {
	script := watermarkPageScript("STARLINE | U-001 | O'Reilly (9069)\\path")

	for _, expected := range []string{
		"(STARLINE | U-001 | O'Reilly \\(9069\\)\\\\path)",
		"<< /BeginPage {",
		"pop\n  StarlineWatermark",
		"initgraphics",
		"clippath pathbbox",
		"/Helvetica findfont",
		"8 scalefont",
		"/row",
		"/column",
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
	if !strings.Contains(script, "0.92 setgray") {
		t.Fatalf("watermark should use a light gray shade, got %s", script)
	}
	if strings.Contains(script, "15 scalefont") {
		t.Fatalf("watermark should not use the old oversized font, got %s", script)
	}
	if count := strings.Count(script, "WatermarkText show"); count != 1 {
		t.Fatalf("watermark should draw from a tiled loop, got %d show calls", count)
	}
}

func TestWatermarkScriptEncodesChineseStudentName(t *testing.T) {
	script := watermarkPageScript("小明")
	if !strings.Contains(script, "(U+5C0FU+660E)") {
		t.Fatalf("watermark should encode non-ASCII names as readable code points, got %s", script)
	}
}

func TestWatermarkCIDFontMapUsesUnicodeTrueTypeFont(t *testing.T) {
	mapText := watermarkCIDFontMap("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	for _, expected := range []string{
		"/StarlineNotoSansCJKsc",
		"/FileType /TrueType",
		"/Path (/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc)",
		"/SubfontID 2",
		"/CSI [(GB1) 2]",
	} {
		if !strings.Contains(mapText, expected) {
			t.Fatalf("watermark cidfmap should contain %q, got %s", expected, mapText)
		}
	}
	if strings.Contains(mapText, "/StarlineNotoSansCJKsc-Regular") {
		t.Fatalf("watermark cidfmap must not use the old hyphenated CID font alias, got %s", mapText)
	}
}

func TestWatermarkGhostscriptFontArgsUseResourceSearchPath(t *testing.T) {
	args, cleanup, err := watermarkGhostscriptFontArgs()
	if err != nil {
		t.Fatalf("create watermark cidfmap: %v", err)
	}
	defer cleanup()

	var searchDir string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-I") {
			searchDir = strings.TrimPrefix(arg, "-I")
			break
		}
	}
	if searchDir == "" {
		t.Fatalf("Ghostscript args should add a resource search directory, got %#v", args)
	}
	mapText, err := os.ReadFile(filepath.Join(searchDir, "Resource", "Init", "cidfmap"))
	if err != nil {
		t.Fatalf("read generated cidfmap: %v", err)
	}
	if !strings.Contains(string(mapText), "/CSI [(GB1) 2]") {
		t.Fatalf("generated cidfmap should use GB1 CSI, got %s", mapText)
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-sCIDFMAP=") {
			t.Fatalf("-sCIDFMAP is only an mkcidfm.ps output option, got %#v", args)
		}
	}
}

func TestWatermarkPDFPassesCIDFontMapToGhostscript(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	argsPath := filepath.Join(root, "gs-args")
	gsScript := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > \"" + argsPath + "\"\n" +
		"for arg in \"$@\"; do case \"$arg\" in -I*) /bin/cat \"${arg#-I}/Resource/Init/cidfmap\" > \"" + argsPath + ".map\";; esac; done\n" +
		"output=''\n" +
		"for arg in \"$@\"; do case \"$arg\" in -sOutputFile=*) output=\"${arg#-sOutputFile=}\";; esac; done\n" +
		"[ -n \"$output\" ] || exit 1\n" +
		"printf 'watermarked' > \"$output\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "gs"), []byte(gsScript), 0755); err != nil {
		t.Fatalf("write fake gs: %v", err)
	}
	t.Setenv("PATH", binDir)

	source := filepath.Join(root, "source.pdf")
	target := filepath.Join(root, "watermarked.pdf")
	if err := os.WriteFile(source, []byte("source"), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := watermarkPDF(context.Background(), source, target, "小明 | U-001"); err != nil {
		t.Fatalf("watermark PDF: %v", err)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read gs args: %v", err)
	}
	var searchDir string
	for _, arg := range strings.Split(strings.TrimSpace(string(args)), "\n") {
		if strings.HasPrefix(arg, "-I") {
			searchDir = strings.TrimPrefix(arg, "-I")
			break
		}
	}
	if searchDir == "" {
		t.Fatalf("Ghostscript should receive a cidfmap search directory, got %s", args)
	}
	mapText, err := os.ReadFile(argsPath + ".map")
	if err != nil {
		t.Fatalf("read Ghostscript cidfmap: %v", err)
	}
	if !strings.Contains(string(mapText), "/CSI [(GB1) 2]") {
		t.Fatalf("Ghostscript cidfmap should use GB1 CSI, got %s", mapText)
	}
}

func TestProtectPDFUsesEncryptionAndDisablesExtractionAndModification(t *testing.T) {
	args := protectPDFArgs("owner-secret", "/tmp/watermarked.pdf", "/tmp/protected.pdf")

	want := []string{"--encrypt", "", "owner-secret", "256", "--print=none", "--modify=none", "--extract=n", "--", "/tmp/watermarked.pdf", "/tmp/protected.pdf"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("protect PDF args = %#v, want %#v", args, want)
	}
}

func TestProtectPDFInvokesQPDFAndCreatesProtectedOutput(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	argsFile := filepath.Join(root, "qpdf-args")
	qpdfScript := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > \"" + argsFile + "\"\n" +
		"target=\"\"\n" +
		"for arg in \"$@\"; do target=\"$arg\"; done\n" +
		"printf 'protected' > \"$target\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "qpdf"), []byte(qpdfScript), 0755); err != nil {
		t.Fatalf("write fake qpdf: %v", err)
	}
	t.Setenv("PATH", binDir)

	source := filepath.Join(root, "source.pdf")
	target := filepath.Join(root, "protected.pdf")
	if err := os.WriteFile(source, []byte("watermarked"), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := protectPDF(context.Background(), source, target); err != nil {
		t.Fatalf("protect PDF: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read protected output: %v", err)
	}
	if string(content) != "protected" {
		t.Fatalf("protected output = %q, want qpdf output", content)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read qpdf args: %v", err)
	}
	for _, expected := range []string{"--encrypt", "256", "--print=none", "--modify=none", "--extract=n", source, target} {
		if !strings.Contains(string(args), expected) {
			t.Fatalf("qpdf args missing %q: %s", expected, args)
		}
	}
}

func TestProtectPDFFailsClosedWhenQPDFMissing(t *testing.T) {
	t.Setenv("PATH", "")
	err := protectPDF(context.Background(), "/tmp/source.pdf", "/tmp/target.pdf")
	if !errors.Is(err, errQPDFUnavailable) {
		t.Fatalf("protect PDF error = %v, want errQPDFUnavailable", err)
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
