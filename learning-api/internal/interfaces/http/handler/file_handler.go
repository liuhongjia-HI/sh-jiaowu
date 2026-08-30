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
		LessonID:        strings.TrimSpace(c.PostForm("lessonId")),
		TagCode:         strings.TrimSpace(c.PostForm("tagCode")),
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

func (h *LearningHandler) ReorderMaterials(c *gin.Context) {
	var req learning.MaterialReorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.ReorderMaterials(operator.(string), principal, req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"reordered": true})
}

func (h *LearningHandler) DeleteMaterial(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.DeleteMaterial(operator.(string), principal, c.Param("id")); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"deleted": true})
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
		LessonID:        strings.TrimSpace(c.PostForm("lessonId")),
		TagCode:         strings.TrimSpace(c.PostForm("tagCode")),
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

func (h *LearningHandler) DeleteHomework(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.DeleteHomework(operator.(string), principal, c.Param("id")); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"deleted": true})
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
	watermarkedPath, err := makeWatermarkedPDF(c.Request.Context(), asset)
	if err != nil {
		secureWatermarkFailure(c, err)
		return
	}
	defer os.Remove(watermarkedPath)
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", "inline; filename=\"secure-preview.pdf\"")
	c.File(watermarkedPath)
}

// StudentMaterialPreviewPages 返回上传后预生成的分页图片元信息。
func (h *LearningHandler) StudentMaterialPreviewPages(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	asset, err := h.service.StudentMaterialPreviewFile(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	metadata := studentPreviewPagesMetadataWithPDFFallback(c.Request.Context(), asset)
	if !metadata.ImageMode {
		OK(c, metadata)
		return
	}
	if strings.TrimSpace(asset.PreviewPageDir) != "" {
		if _, err := os.Stat(filepath.Join(asset.PreviewPageDir, "page-0001.jpg")); err == nil {
			OK(c, metadata)
			return
		}
		metadata.PreviewStatus = "ready"
		metadata.ImageMode = false
		metadata.PageCount = 0
		metadata.Message = "缩略图暂时不可用，点击打开完整课件"
		OK(c, metadata)
		return
	}
	OK(c, metadata)
}

type studentPreviewPagesResponse struct {
	PreviewStatus string `json:"previewStatus"`
	ImageMode     bool   `json:"imageMode"`
	PageCount     int    `json:"pageCount"`
	Message       string `json:"message,omitempty"`
}

func studentPreviewPagesMetadata(asset learning.FileAsset) studentPreviewPagesResponse {
	switch asset.PreviewStatus {
	case "待转换", "处理中":
		return studentPreviewPagesResponse{PreviewStatus: "processing", Message: "课件正在生成，请稍后再试"}
	case "转换失败":
		return studentPreviewPagesResponse{PreviewStatus: "failed", Message: "课件生成失败，请联系老师处理"}
	case "可预览":
		if strings.TrimSpace(asset.PreviewPath) == "" {
			return studentPreviewPagesResponse{PreviewStatus: "unavailable", Message: "历史课件文件不可用，请联系老师重新上传"}
		}
		imageMode := asset.PreviewPageCount > 0 && strings.TrimSpace(asset.PreviewPageDir) != ""
		message := ""
		if !imageMode {
			message = "暂未生成缩略图，点击打开完整课件"
		}
		return studentPreviewPagesResponse{PreviewStatus: "ready", ImageMode: imageMode, PageCount: asset.PreviewPageCount, Message: message}
	default:
		return studentPreviewPagesResponse{PreviewStatus: "unavailable", Message: "历史课件文件不可用，请联系老师重新上传"}
	}
}

// studentPreviewPagesMetadataWithPDFFallback 让历史 PDF 在后台分页回填完成前也能立即阅读。
// 单页接口本来就会从 PreviewPath 动态生成带水印图片，因此这里只需现场读取页数，
// 不必让用户等待 PreviewPageDir 先写完。
func studentPreviewPagesMetadataWithPDFFallback(ctx context.Context, asset learning.FileAsset) studentPreviewPagesResponse {
	metadata := studentPreviewPagesMetadata(asset)
	if metadata.PreviewStatus != "ready" || metadata.ImageMode || strings.TrimSpace(asset.PreviewPath) == "" {
		return metadata
	}
	pageCount, err := studentPreviewPageCount(ctx, asset)
	if err != nil {
		return metadata
	}
	metadata.ImageMode = true
	metadata.PageCount = pageCount
	metadata.Message = ""
	return metadata
}

func studentPreviewPageCount(ctx context.Context, asset learning.FileAsset) (int, error) {
	if asset.PreviewPageCount > 0 {
		return asset.PreviewPageCount, nil
	}
	previewPath := strings.TrimSpace(asset.PreviewPath)
	if previewPath == "" {
		return 0, errors.New("预览文件还没有生成")
	}
	if _, err := os.Stat(previewPath); err != nil {
		return 0, err
	}
	return countPDFPages(ctx, previewPath)
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
	pageCount, err := studentPreviewPageCount(c.Request.Context(), asset)
	if err != nil {
		BadRequest(c, "课件页面正在生成，请稍后重试")
		return
	}
	if page > pageCount {
		BadRequest(c, "页码超出课件范围")
		return
	}
	if strings.TrimSpace(asset.PreviewPageDir) != "" {
		sourceImagePath := filepath.Join(asset.PreviewPageDir, fmt.Sprintf("page-%04d.jpg", page))
		if _, err := os.Stat(sourceImagePath); err != nil {
			BadRequest(c, "本页课件文件不可用，请联系老师重新生成")
			return
		}
	}
	imageFile, err := os.CreateTemp("", "starline-material-watermark-*.jpg")
	if err != nil {
		secureWatermarkFailure(c, err)
		return
	}
	imagePath := imageFile.Name()
	if err := imageFile.Close(); err != nil {
		os.Remove(imagePath)
		secureWatermarkFailure(c, err)
		return
	}
	defer os.Remove(imagePath)
	if err := rasterizeWatermarkedPDFPage(c.Request.Context(), asset.PreviewPath, imagePath, page, asset.WatermarkStampText); err != nil {
		secureWatermarkFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
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
	material, err := h.service.StudentMaterial(principal, c.Param("id"))
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	if material.DownloadURL == "" {
		Forbidden(c, "当前资料仅支持在线预览")
		return
	}
	asset, err := h.service.StudentMaterialPreviewFile(principal, c.Param("id"))
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	if asset.PreviewStatus != "可预览" || strings.TrimSpace(asset.PreviewPath) == "" {
		BadRequest(c, previewUnavailableMessage(asset))
		return
	}
	if _, err := os.Stat(asset.PreviewPath); err != nil {
		BadRequest(c, "历史课件文件缺失，请联系老师重新上传")
		return
	}
	watermarkedPath, err := makeWatermarkedPDF(c.Request.Context(), asset)
	if err != nil {
		secureWatermarkFailure(c, err)
		return
	}
	defer os.Remove(watermarkedPath)
	c.Header("Cache-Control", "private, no-store")
	c.FileAttachment(watermarkedPath, studentWatermarkedFileName(asset.FileName))
}

func makeWatermarkedPDF(ctx context.Context, asset learning.FileAsset) (string, error) {
	if strings.TrimSpace(asset.WatermarkStampText) == "" {
		return "", errors.New("课件缺少专属水印信息")
	}
	file, err := os.CreateTemp("", "starline-material-watermark-*.pdf")
	if err != nil {
		return "", err
	}
	targetPath := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(targetPath)
		return "", err
	}
	if err := watermarkPDF(ctx, asset.PreviewPath, targetPath, asset.WatermarkStampText); err != nil {
		os.Remove(targetPath)
		return "", err
	}
	return targetPath, nil
}

func secureWatermarkFailure(c *gin.Context, err error) {
	if errors.Is(err, errGhostscriptUnavailable) {
		BadRequest(c, "课件安全水印服务暂不可用，请稍后再试")
		return
	}
	BadRequest(c, "课件安全水印生成失败，请稍后再试")
}

func studentWatermarkedFileName(fileName string) string {
	base := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	if base == "" || base == "." {
		base = "学习资料"
	}
	return base + "-学习版.pdf"
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
