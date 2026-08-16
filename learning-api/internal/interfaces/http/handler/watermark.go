package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// errGhostscriptUnavailable 表示服务器没有安装 Ghostscript，无法生成分页图片。
var errGhostscriptUnavailable = errors.New("ghostscript unavailable")

// execCommandContext 可在测试中替换，用于在不安装 Ghostscript 的环境下验证降级逻辑。
var execCommandContext = exec.CommandContext

// watermarkStampTimeout 是单页栅格化允许的最长时间，避免超大文件拖垮任务。
const watermarkStampTimeout = 20 * time.Second

func ghostscriptAvailable() bool {
	_, err := exec.LookPath("gs")
	return err == nil
}

// rasterizePDFPage 把 sourcePath 的第 page 页（从 1 开始）栅格化成 JPEG 图片写入 targetPath。
func rasterizePDFPage(ctx context.Context, sourcePath, targetPath string, page int) error {
	if !ghostscriptAvailable() {
		return errGhostscriptUnavailable
	}
	if page < 1 {
		return errors.New("页码必须从 1 开始")
	}
	ctx, cancel := context.WithTimeout(ctx, watermarkStampTimeout)
	defer cancel()
	pageArg := fmt.Sprintf("%d", page)
	cmd := execCommandContext(ctx, "gs",
		"-q", "-dBATCH", "-dNOPAUSE", "-dSAFER",
		"-sDEVICE=jpeg", "-r100", "-dJPEGQ=82",
		"-dFirstPage="+pageArg, "-dLastPage="+pageArg,
		"-sOutputFile="+targetPath,
		sourcePath,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("分页栅格化失败: %w", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("分页图片未生成: %w", err)
	}
	return nil
}

// countPDFPages 用 Ghostscript 内置的 runpdfbegin/pdfpagecount 探测页数，
// 不依赖 poppler 等额外工具。
func countPDFPages(ctx context.Context, sourcePath string) (int, error) {
	if !ghostscriptAvailable() {
		return 0, errGhostscriptUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, watermarkStampTimeout)
	defer cancel()
	countScript := `
%!PS
(` + escapePostScriptString(sourcePath) + `) (r) file runpdfbegin
pdfpagecount = quit
`
	scriptPath, err := writeTempFile(os.TempDir(), "pagecount-*.ps", countScript)
	if err != nil {
		return 0, err
	}
	defer os.Remove(scriptPath)
	countCmd := execCommandContext(ctx, "gs", "-q", "-dBATCH", "-dNOPAUSE", "-dSAFER", "-dNODISPLAY", scriptPath)
	out, err := countCmd.Output()
	if err != nil {
		return 0, fmt.Errorf("页数探测失败: %w", err)
	}
	count := 0
	if _, scanErr := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count); scanErr != nil || count <= 0 {
		return 0, errors.New("页数探测结果无法解析")
	}
	return count, nil
}

// escapePostScriptString 转义用于页数探测脚本的本地文件路径。
func escapePostScriptString(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return replacer.Replace(text)
}

func writeTempFile(dir, pattern, content string) (string, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
