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
	"unicode/utf16"
)

// errGhostscriptUnavailable 表示服务器没有安装 Ghostscript，无法生成分页图片。
var errGhostscriptUnavailable = errors.New("ghostscript unavailable")

// watermarkFontPath 是 production provisioner 安装的 Noto Sans CJK 字体集合。
// fonts-noto-cjk 的简体中文子字体固定为 TTC 的第 2 个 face（JP=0、KR=1、SC=2）。
const watermarkFontPath = "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"

// Ghostscript 用 "CIDFont-CMap" 形式自动组合字体和 CMap。CID 字体别名不能
// 再包含连字符，否则会把 "Regular-Identity-UTF16-H" 误解析成 CMap 名的一部分，
// 最终虽然能生成 PDF，却会把水印字符映射成乱码。
const watermarkCIDFontName = "StarlineNotoSansCJKsc"

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

// rasterizeWatermarkedPDFPage 将学生专属水印直接写进单页 JPEG，返回给小程序的不是可移除的前端覆盖层。
func rasterizeWatermarkedPDFPage(ctx context.Context, sourcePath, targetPath string, page int, watermarkText string) error {
	if !ghostscriptAvailable() {
		return errGhostscriptUnavailable
	}
	if page < 1 {
		return errors.New("页码必须从 1 开始")
	}
	ctx, cancel := context.WithTimeout(ctx, watermarkStampTimeout)
	defer cancel()
	fontArgs, cleanupFontMap, err := watermarkGhostscriptFontArgs()
	if err != nil {
		return fmt.Errorf("准备中文水印字体失败: %w", err)
	}
	defer cleanupFontMap()
	pageArg := fmt.Sprintf("%d", page)
	args := append([]string{}, fontArgs...)
	args = append(args,
		"-q", "-dBATCH", "-dNOPAUSE", "-dSAFER",
		"-sDEVICE=jpeg", "-r100", "-dJPEGQ=82",
		"-dFirstPage="+pageArg, "-dLastPage="+pageArg,
		"-sOutputFile="+targetPath,
	)
	args = append(args,
		"-c", watermarkPageScript(watermarkText),
		"-f", sourcePath,
	)
	cmd := execCommandContext(ctx, "gs", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("生成带水印分页图片失败: %w", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("带水印分页图片未生成: %w", err)
	}
	return nil
}

// watermarkPDF 生成只供当前学生下载的 PDF；原文件和标准 preview.pdf 永不经学生接口下发。
func watermarkPDF(ctx context.Context, sourcePath, targetPath, watermarkText string) error {
	if !ghostscriptAvailable() {
		return errGhostscriptUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	fontArgs, cleanupFontMap, err := watermarkGhostscriptFontArgs()
	if err != nil {
		return fmt.Errorf("准备中文水印字体失败: %w", err)
	}
	defer cleanupFontMap()
	args := append([]string{}, fontArgs...)
	args = append(args,
		"-q", "-dBATCH", "-dNOPAUSE", "-dSAFER",
		"-sDEVICE=pdfwrite", "-sOutputFile="+targetPath,
	)
	args = append(args,
		"-c", watermarkPageScript(watermarkText),
		"-f", sourcePath,
	)
	cmd := execCommandContext(ctx, "gs", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("生成带水印 PDF 失败: %w", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("带水印 PDF 未生成: %w", err)
	}
	return nil
}

// watermarkPageScript 通过 BeginPage 钩子先绘制姓名水印，再由 PDF 内容覆盖，
// 避免水印压住正文。Ghostscript 的 PDF 解释器支持 BeginPage / EndPage 钩子：
// https://ghostscript.com/blog/pdfi.html
func watermarkPageScript(watermarkText string) string {
	encoded := encodePostScriptUTF16BE(watermarkText)
	fontResource := watermarkCIDFontName + "-Identity-UTF16-H"
	return `/StarlineWatermark {
  gsave
  initgraphics
  0.92 setgray
  /` + fontResource + ` findfont
  8 scalefont setfont
  /WatermarkText ` + encoded + ` def
  clippath pathbbox
  /pageTop exch def
  /pageRight exch def
  /pageBottom exch def
  /pageLeft exch def
  pageRight pageLeft sub 2 div /halfWidth exch def
  pageTop pageBottom sub 2 div /halfHeight exch def
  pageRight pageLeft add 2 div
  pageTop pageBottom add 2 div
  translate
  35 rotate
  halfWidth neg 260 sub
  230
  halfWidth 260 add
  {
    /column exch def
    halfHeight neg 180 sub
    150
    halfHeight 180 add
    {
      /row exch def
      column row moveto
      WatermarkText show
    } for
  } for
  grestore
} bind def
<< /BeginPage {
  pop
  StarlineWatermark
} bind >> setpagedevice`
}

// watermarkCIDFontMap 为 Ghostscript 显式声明 Unicode TrueType 字体。
// 不能直接把 Identity CMap 套在 Noto 的 GB1 CID 资源上，否则 ASCII 和中文都会被映射到错误的 CID。
func watermarkCIDFontMap(fontPath string) string {
	return fmt.Sprintf("%%!PS\n/%s << /FileType /TrueType /Path (%s) /SubfontID 2 /CSI [(Artifex) (Unicode) 0] >> ;\n", watermarkCIDFontName, escapePostScriptString(fontPath))
}

// watermarkGhostscriptFontArgs 为每次 Ghostscript 调用生成隔离的 cidfmap，
// 同时显式放行 map 和 TTC 文件，兼容 -dSAFER 模式。
func watermarkGhostscriptFontArgs() ([]string, func(), error) {
	mapDir, err := os.MkdirTemp(os.TempDir(), "starline-cidfmap-*")
	if err != nil {
		return nil, func() {}, err
	}
	mapPath := filepath.Join(mapDir, "cidfmap")
	if err := os.WriteFile(mapPath, []byte(watermarkCIDFontMap(watermarkFontPath)), 0600); err != nil {
		_ = os.RemoveAll(mapDir)
		return nil, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(mapDir) }
	return []string{
		"-I" + mapDir,
		"--permit-file-read=" + mapPath,
		"--permit-file-read=" + watermarkFontPath,
	}, cleanup, nil
}

// encodePostScriptUTF16BE 生成供 Identity-UTF16-H 使用的 UTF-16BE 十六进制字符串，
// 这样中文姓名可以由服务器端 Noto CJK 字体稳定渲染，不依赖 PostScript 源码编码。
func encodePostScriptUTF16BE(text string) string {
	units := utf16.Encode([]rune(text))
	var builder strings.Builder
	builder.Grow(len(units)*4 + 2)
	builder.WriteByte('<')
	for _, unit := range units {
		fmt.Fprintf(&builder, "%04X", unit)
	}
	builder.WriteByte('>')
	return builder.String()
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
	countCmd := execCommandContext(ctx, "gs", "-q", "-dBATCH", "-dNOPAUSE", "-dSAFER", "--permit-file-read="+sourcePath, "-dNODISPLAY", scriptPath)
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
