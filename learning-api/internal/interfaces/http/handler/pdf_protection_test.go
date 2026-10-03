package handler

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const protectedPDFMetadata = `{"encrypt":{"encrypted":true,"userpasswordmatched":true,"parameters":{"bits":256,"method":"AESv3"},"capabilities":{"extract":false,"modify":false,"modifyannotations":false,"modifyassembly":false,"modifyforms":false,"modifyother":false,"printhigh":true,"printlow":true}}}`

func TestProtectPDFWarningAndFailureHandling(t *testing.T) {
	for _, tc := range []struct {
		name                                    string
		generationExit, checkExit, metadataExit int
		content, metadata, wantError            string
	}{
		{name: "warning with valid protected output", generationExit: 3, content: "protected", metadata: protectedPDFMetadata},
		{name: "warning during output inspection", generationExit: 3, checkExit: 3, metadataExit: 3, content: "protected", metadata: protectedPDFMetadata},
		{name: "genuine generation error", generationExit: 2, content: "partial", wantError: "fixture warning"},
		{name: "invalid output structure", generationExit: 3, checkExit: 2, content: "partial", wantError: "结构校验失败"},
		{name: "unprotected output", generationExit: 3, content: "plain", metadata: `{"encrypt":{"encrypted":false}}`, wantError: "AES-256"},
		{name: "extraction allowed", generationExit: 3, content: "protected", metadata: strings.Replace(protectedPDFMetadata, `"extract":false`, `"extract":true`, 1), wantError: "权限校验失败"},
		{name: "missing capability", generationExit: 3, content: "protected", metadata: strings.Replace(protectedPDFMetadata, `"extract":false,`, "", 1), wantError: "权限校验失败"},
		{name: "metadata command failure", generationExit: 3, metadataExit: 2, content: "protected", wantError: "读取受保护 PDF 权限失败"},
		{name: "invalid metadata", generationExit: 3, content: "protected", metadata: "broken", wantError: "解析受保护 PDF 权限失败"},
		{name: "empty output", content: "", wantError: "输出为空"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			script := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\n--check) exit %d;;\n--json) printf '%%s' '%s'; exit %d;;\nesac\nfor arg in \"$@\"; do target=\"$arg\"; done\nprintf '%%s' '%s' > \"$target\"\nprintf 'fixture warning' >&2\nexit %d\n", tc.checkExit, tc.metadata, tc.metadataExit, tc.content, tc.generationExit)
			if err := os.WriteFile(filepath.Join(root, "qpdf"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", root)
			err := protectPDF(context.Background(), filepath.Join(root, "source.pdf"), filepath.Join(root, "target.pdf"))
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestPDFDiagnosticsRedactOwnerPassword(t *testing.T) {
	if got := safePDFDiagnostic("failed: sensitive-password", "sensitive-password"); strings.Contains(got, "sensitive-password") {
		t.Fatal("owner password leaked")
	}
}

// 用真实工具生成可恢复的 xref 警告，验证恢复后的输出依旧满足安全权限。
func TestProtectPDFRealQPDFRecoverableWarning(t *testing.T) {
	for _, name := range []string{"gs", "qpdf"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip("requires real gs and qpdf")
		}
	}
	root := t.TempDir()
	source := filepath.Join(root, "source.pdf")
	cmd := exec.Command("gs", "-q", "-dBATCH", "-dNOPAUSE", "-sDEVICE=pdfwrite", "-sOutputFile="+source, "-c", "/Helvetica findfont 12 scalefont setfont 72 720 moveto (Protection regression) show showpage")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, output)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	data = regexp.MustCompile(`startxref\s+\d+`).ReplaceAll(data, []byte("startxref\n1"))
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "protected.pdf")
	output, err := exec.Command("qpdf", protectPDFArgs("fixture-only", source, target)...).CombinedOutput()
	if !qpdfWarningsOnly(err) {
		t.Fatalf("fixture must cause warning exit 3: %v %s", err, output)
	}
	if err := protectPDF(context.Background(), source, target); err != nil {
		t.Fatal(err)
	}
	if err := validateProtectedPDF(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command("qpdf", "--check", target).Output(); err != nil {
		t.Fatalf("protected output structure: %v", err)
	}
}
