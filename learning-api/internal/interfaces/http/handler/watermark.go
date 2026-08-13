package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// errGhostscriptUnavailable 表示服务器没有安装 Ghostscript，无法烧录水印或分页栅格化。
// 调用方应当在这种情况下降级为下发未加动态水印的干净预览文件，而不是报错中断。
var errGhostscriptUnavailable = errors.New("ghostscript unavailable")

// execCommandContext 可在测试中替换，用于在不安装 Ghostscript 的环境下验证降级逻辑。
var execCommandContext = exec.CommandContext

// watermarkStampTimeout 是单次水印烧录/栅格化允许的最长时间，避免超大文件拖垮请求。
const watermarkStampTimeout = 20 * time.Second

func ghostscriptAvailable() bool {
	_, err := exec.LookPath("gs")
	return err == nil
}

// stampWatermarkPDF 把 text 以对角平铺的形式烧录进 sourcePath 每一页的内容流，
// 输出到 targetPath。烧录后的文字是 PDF 内容本身的一部分，不是覆盖层，无法通过
// 删除某个图层或注释被去掉。sourcePath 不会被修改。
func stampWatermarkPDF(ctx context.Context, sourcePath, targetPath, text string) error {
	if !ghostscriptAvailable() {
		return errGhostscriptUnavailable
	}
	script := watermarkPostScript(text)
	scriptPath, err := writeTempFile(filepath.Dir(targetPath), "watermark-*.ps", script)
	if err != nil {
		return err
	}
	defer os.Remove(scriptPath)

	ctx, cancel := context.WithTimeout(ctx, watermarkStampTimeout)
	defer cancel()
	cmd := execCommandContext(ctx, "gs",
		"-q", "-dBATCH", "-dNOPAUSE", "-dSAFER",
		"-sDEVICE=pdfwrite",
		"-sOutputFile="+targetPath,
		scriptPath,
		"-f", sourcePath,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("水印烧录失败: %w", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("水印文件未生成: %w", err)
	}
	return nil
}

// rasterizePDFPage 把 sourcePath 的第 page 页（从 1 开始）栅格化成 JPEG 图片写入 targetPath。
// 配合 stampWatermarkPDF 先烧录水印再栅格化，输出图片里的水印是像素点，
// 无法通过提取 PDF 文本层或复制内容的方式剥离。
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

// watermarkPostScript 生成一段 PostScript：在 Ghostscript 处理 pdfwrite 输出的每一页时，
// 用 BeginPage 钩子叠加一层浅灰色、旋转 30 度、按网格平铺的文字水印。
func watermarkPostScript(text string) string {
	escaped := escapePostScriptString(text)
	return `%!PS
<< /BeginPage {
  gsave
  0.82 0.82 0.82 setrgbcolor
  /Helvetica findfont 13 scalefont setfont
  currentpagedevice /PageSize get aload pop /pageH exch def /pageW exch def
  pageW 2 div pageH 2 div translate
  30 rotate
  pageW neg pageH neg translate
  0 60 pageH 120 add {
    /y exch def
    0 40 pageW 260 add {
      /x exch def
      x y moveto (` + escaped + `) show
    } for
  } for
  grestore
} >> setpagedevice
`
}

// escapePostScriptString 转义 PostScript 字符串字面量里的反斜杠和括号，
// 防止水印文本（可能包含手机号、时间等用户输入拼接结果）break 出字符串或注入指令。
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
