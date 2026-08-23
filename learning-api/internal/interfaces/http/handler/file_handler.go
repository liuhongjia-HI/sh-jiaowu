package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

const maxUploadSize = 50 * 1024 * 1024

var allowedUploadTypes = map[string]struct {
	label       string
	contentType string
}{
	".pdf":  {label: "PDF", contentType: "application/pdf"},
	".ppt":  {label: "PPT", contentType: "application/vnd.ms-powerpoint"},
	".pptx": {label: "PPT", contentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	".doc":  {label: "Word", contentType: "application/msword"},
	".docx": {label: "Word", contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
}

func (h *LearningHandler) CreateMaterial(c *gin.Context) {
	asset, ok := h.saveUploadedLearningFile(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateMaterial(operator.(string), principal, learning.MaterialUploadRequest{
		Title:           strings.TrimSpace(c.PostForm("title")),
		LearningSpaceID: strings.TrimSpace(c.PostForm("learningSpaceId")),
		CourseID:        strings.TrimSpace(c.PostForm("courseId")),
		Chapter:         strings.TrimSpace(c.PostForm("chapter")),
		File:            asset,
	})
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}

func (h *LearningHandler) UpdateMaterial(c *gin.Context) {
	var req learning.MaterialUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateMaterial(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) CreateHomework(c *gin.Context) {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		var req learning.HomeworkUploadRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			BadRequest(c, "请求格式不正确")
			return
		}
		principal, _ := middleware.CurrentPrincipal(c)
		operator, _ := c.Get(middleware.OperatorNameKey)
		created, err := h.service.CreateHomework(operator.(string), principal, req)
		if err != nil {
			BadRequest(c, err.Error())
			return
		}
		OK(c, created)
		return
	}
	asset, ok := h.saveUploadedLearningFile(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateHomework(operator.(string), principal, learning.HomeworkUploadRequest{
		Title:           strings.TrimSpace(c.PostForm("title")),
		LearningSpaceID: strings.TrimSpace(c.PostForm("learningSpaceId")),
		CourseID:        strings.TrimSpace(c.PostForm("courseId")),
		Deadline:        strings.TrimSpace(c.PostForm("deadline")),
		File:            asset,
	})
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}

func (h *LearningHandler) UpdateHomework(c *gin.Context) {
	var req learning.HomeworkUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateHomework(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) PreviewFile(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	asset, err := h.service.ContentFile(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	if asset.PreviewStatus != "可预览" || asset.PreviewPath == "" {
		BadRequest(c, "预览文件还没有生成，请下载原文件查看")
		return
	}
	if _, err := os.Stat(asset.PreviewPath); err != nil {
		// 预览文件已经从磁盘消失（例如历史发布把它写在被清理的 release 目录里）。
		// 回写成转换失败，列表里才会出现「重新生成预览」入口。
		_ = h.service.MarkPreviewFileMissing(asset.ID, "预览文件已丢失，请重新生成预览")
		BadRequest(c, "预览文件不存在，请重新生成预览或下载原文件查看")
		return
	}
	c.Header("Content-Disposition", "inline; filename=\"preview.pdf\"")
	c.File(asset.PreviewPath)
}

func (h *LearningHandler) StudentMaterialPreview(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	asset, err := h.service.StudentMaterialPreviewFile(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	if asset.PreviewStatus != "可预览" || asset.PreviewPath == "" {
		BadRequest(c, previewUnavailableMessage(asset))
		return
	}
	if _, err := os.Stat(asset.PreviewPath); err != nil {
		BadRequest(c, "历史课件文件不可用，请联系老师重新上传")
		return
	}
	c.Header("Content-Disposition", "inline; filename=\"secure-preview.pdf\"")
	c.File(asset.PreviewPath)
}

// StudentMaterialPreviewPages 返回上传后预生成的分页图片元信息。
func (h *LearningHandler) StudentMaterialPreviewPages(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	asset, err := h.service.StudentMaterialPreviewFile(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	if asset.PreviewStatus != "可预览" {
		BadRequest(c, previewUnavailableMessage(asset))
		return
	}
	if asset.PreviewPageCount <= 0 || strings.TrimSpace(asset.PreviewPageDir) == "" {
		OK(c, gin.H{"imageMode": false, "pageCount": 0})
		return
	}
	if _, err := os.Stat(filepath.Join(asset.PreviewPageDir, "page-0001.jpg")); err != nil {
		BadRequest(c, "历史课件分页文件不可用，请联系老师重新上传")
		return
	}
	OK(c, gin.H{"imageMode": true, "pageCount": asset.PreviewPageCount})
}

// StudentMaterialPreviewPage 返回单页栅格化图片，学生水印由小程序覆盖显示。
func (h *LearningHandler) StudentMaterialPreviewPage(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	page, err := strconv.Atoi(c.Param("page"))
	if err != nil || page < 1 {
		BadRequest(c, "页码不正确")
		return
	}
	asset, err := h.service.StudentMaterialPreviewFile(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	if asset.PreviewStatus != "可预览" {
		BadRequest(c, previewUnavailableMessage(asset))
		return
	}
	if page > asset.PreviewPageCount || strings.TrimSpace(asset.PreviewPageDir) == "" {
		BadRequest(c, "页码超出课件范围")
		return
	}
	imagePath := filepath.Join(asset.PreviewPageDir, fmt.Sprintf("page-%04d.jpg", page))
	if _, err := os.Stat(imagePath); err != nil {
		BadRequest(c, "本页课件文件不可用，请联系老师重新生成")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.File(imagePath)
}

func previewUnavailableMessage(asset learning.FileAsset) string {
	switch asset.PreviewStatus {
	case "转换失败":
		return "课件生成失败，请联系老师处理"
	case "待转换", "处理中":
		return "课件正在生成，请稍后再试"
	default:
		return "历史课件文件不可用，请联系老师重新上传"
	}
}

func (h *LearningHandler) DownloadFile(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	asset, err := h.service.ContentFile(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	if _, err := os.Stat(asset.OriginalPath); err != nil {
		BadRequest(c, "原文件不存在")
		return
	}
	c.FileAttachment(asset.OriginalPath, asset.FileName)
}

// StudentMaterialDownload 复用学生资料访问校验；套餐授权到期后下载地址立即失效。
func (h *LearningHandler) StudentMaterialDownload(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	asset, err := h.service.StudentMaterialPreviewFile(principal, c.Param("id"))
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	if _, err := os.Stat(asset.OriginalPath); err != nil {
		BadRequest(c, "历史课件文件缺失，请联系老师重新上传")
		return
	}
	c.FileAttachment(asset.OriginalPath, asset.FileName)
}

func (h *LearningHandler) RetryFilePreview(c *gin.Context) {
	operator, _ := c.Get(middleware.OperatorNameKey)
	principal, _ := middleware.CurrentPrincipal(c)
	if err := h.service.RetryPreviewJob(operator.(string), principal, c.Param("id")); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"fileId": c.Param("id"), "previewStatus": "待转换"})
}

func (h *LearningHandler) saveUploadedLearningFile(c *gin.Context) (learning.FileAsset, bool) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择要上传的文件")
		return learning.FileAsset{}, false
	}
	asset, err := saveLearningFileAt(file, h.fileStorageRoot)
	if err != nil {
		BadRequest(c, err.Error())
		return learning.FileAsset{}, false
	}
	return asset, true
}

func saveLearningFile(file *multipart.FileHeader) (learning.FileAsset, error) {
	return saveLearningFileAt(file, "uploads")
}

func saveLearningFileAt(file *multipart.FileHeader, storageRoot string) (learning.FileAsset, error) {
	if file.Size <= 0 {
		return learning.FileAsset{}, errors.New("文件内容为空，请重新选择")
	}
	if file.Size > maxUploadSize {
		return learning.FileAsset{}, errors.New("文件太大，请上传 50MB 以内的文件")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	spec, ok := allowedUploadTypes[ext]
	if !ok {
		return learning.FileAsset{}, errors.New("仅支持 PDF、PPT、Word 文件")
	}
	if !uploadSignatureMatches(file, ext) {
		return learning.FileAsset{}, errors.New("文件内容与扩展名不一致，请重新选择正确的课件文件")
	}
	uploadRoot, err := filepath.Abs(storageRoot)
	if err != nil {
		return learning.FileAsset{}, errors.New("上传目录初始化失败")
	}
	originalDir := filepath.Join(uploadRoot, "original")
	if err := os.MkdirAll(originalDir, 0755); err != nil {
		return learning.FileAsset{}, errors.New("上传目录创建失败")
	}
	id := "file-" + time.Now().Format("20060102150405.000000000")
	originalPath := filepath.Join(originalDir, id+ext)
	if err := copyUpload(file, originalPath); err != nil {
		return learning.FileAsset{}, errors.New("文件保存失败")
	}
	return learning.FileAsset{
		ID:            id,
		FileName:      filepath.Base(file.Filename),
		FileSize:      file.Size,
		FileType:      spec.label,
		ContentType:   spec.contentType,
		OriginalPath:  originalPath,
		PreviewStatus: "待转换",
	}, nil
}

func uploadSignatureMatches(file *multipart.FileHeader, ext string) bool {
	source, err := file.Open()
	if err != nil {
		return false
	}
	defer source.Close()
	header := make([]byte, 8)
	count, err := source.Read(header)
	if err != nil && err != io.EOF {
		return false
	}
	header = header[:count]
	if ext == ".pdf" {
		return len(header) >= 5 && string(header[:5]) == "%PDF-"
	}
	if ext == ".docx" || ext == ".pptx" {
		return len(header) >= 4 && header[0] == 'P' && header[1] == 'K' && header[2] == 3 && header[3] == 4
	}
	return len(header) >= 8 && header[0] == 0xd0 && header[1] == 0xcf && header[2] == 0x11 && header[3] == 0xe0 && header[4] == 0xa1 && header[5] == 0xb1 && header[6] == 0x1a && header[7] == 0xe1
}

func copyUpload(file *multipart.FileHeader, target string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(target)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func copyFile(sourcePath, targetPath string) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func buildPreview(ctx context.Context, originalPath, previewDir, ext string) (string, error) {
	if ext == ".pdf" {
		// 预览文件必须与原始文件在磁盘上物理分离：即使内容一开始相同，
		// 学生端也永远只可能拿到 preview/ 目录下的文件路径，任何代码路径的
		// 疏漏都不会意外把 original/ 目录下的原文件暴露出去。
		previewPath := filepath.Join(previewDir, filepath.Base(originalPath))
		if err := copyFile(originalPath, previewPath); err != nil {
			return "", fmt.Errorf("复制 PDF 预览文件失败: %w", err)
		}
		return previewPath, nil
	}
	if _, err := exec.LookPath("soffice"); err != nil {
		return "", errors.New("服务器未安装 LibreOffice，无法转换 Word/PPT")
	}
	profileDir, err := os.MkdirTemp("", "starline-soffice-")
	if err != nil {
		return "", fmt.Errorf("创建 LibreOffice 临时目录失败: %w", err)
	}
	defer os.RemoveAll(profileDir)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	profileURI := "file://" + filepath.ToSlash(profileDir)
	cmd := exec.CommandContext(ctx, "soffice", "-env:UserInstallation="+profileURI, "--headless", "--convert-to", "pdf", "--outdir", previewDir, originalPath)
	if _, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errors.New("LibreOffice 转换超时，请检查文件大小或内容")
		}
		return "", errors.New("LibreOffice 转换失败，请检查 Word/PPT 是否损坏或已加密")
	}
	previewPath := filepath.Join(previewDir, strings.TrimSuffix(filepath.Base(originalPath), filepath.Ext(originalPath))+".pdf")
	if _, err := os.Stat(previewPath); err != nil {
		return "", errors.New("LibreOffice 未生成 PDF，请检查 Word/PPT 是否损坏或已加密")
	}
	return previewPath, nil
}
