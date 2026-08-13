package handler

import (
	"context"
	"errors"
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
		BadRequest(c, "预览文件不存在，请下载原文件查看")
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
	if _, err := os.Stat(asset.PreviewPath); err != nil {
		BadRequest(c, "资料正在生成安全预览，请稍后再试")
		return
	}
	servePath := asset.PreviewPath
	// 优先下发烧录了本次访问学生水印的临时文件；Ghostscript 不可用时，
	// 降级为下发干净的预览副本——依然与原始文件物理隔离，只是没有动态水印。
	if strings.TrimSpace(asset.WatermarkText) != "" {
		if stampedPath, err := stampStudentPreviewCopy(c.Request.Context(), asset.PreviewPath, asset.WatermarkText); err == nil {
			defer os.Remove(stampedPath)
			servePath = stampedPath
		} else if err != errGhostscriptUnavailable {
			h.recordSecurityEvent(c, "水印烧录失败", asset.ID, err.Error())
		}
	}
	c.Header("Content-Disposition", "inline; filename=\"secure-preview.pdf\"")
	c.File(servePath)
}

// StudentMaterialPreviewPages 返回分页预览的元信息。图片模式依赖服务器安装 Ghostscript；
// 不可用时前端应当回退到 StudentMaterialPreview 的整份 PDF 预览。
func (h *LearningHandler) StudentMaterialPreviewPages(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	asset, err := h.service.StudentMaterialPreviewFile(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	if _, err := os.Stat(asset.PreviewPath); err != nil {
		BadRequest(c, "资料正在生成安全预览，请稍后再试")
		return
	}
	pageCount, err := countPDFPages(c.Request.Context(), asset.PreviewPath)
	if err != nil {
		OK(c, gin.H{"imageMode": false, "pageCount": 0})
		return
	}
	OK(c, gin.H{"imageMode": true, "pageCount": pageCount})
}

// StudentMaterialPreviewPage 返回单页栅格化图片，水印已经烧进像素点，
// 不是覆盖层，无法通过复制文本或去掉某一层的方式剥离。
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
	if _, err := os.Stat(asset.PreviewPath); err != nil {
		BadRequest(c, "资料正在生成安全预览，请稍后再试")
		return
	}
	sourcePath := asset.PreviewPath
	if strings.TrimSpace(asset.WatermarkText) != "" {
		if stampedPath, stampErr := stampStudentPreviewCopy(c.Request.Context(), asset.PreviewPath, asset.WatermarkText); stampErr == nil {
			defer os.Remove(stampedPath)
			sourcePath = stampedPath
		}
	}
	imagePath, err := studentPreviewPageImage(c.Request.Context(), sourcePath, asset.ID, page)
	if err != nil {
		BadRequest(c, "该页图片生成失败，请稍后再试")
		return
	}
	defer os.Remove(imagePath)
	c.Header("Cache-Control", "no-store")
	c.File(imagePath)
}

func stampStudentPreviewCopy(ctx context.Context, previewPath, watermarkText string) (string, error) {
	stampedDir := filepath.Join(filepath.Dir(filepath.Dir(previewPath)), "preview-stamped")
	if err := os.MkdirAll(stampedDir, 0755); err != nil {
		return "", err
	}
	target := filepath.Join(stampedDir, "stamp-"+time.Now().Format("20060102150405.000000000")+".pdf")
	if err := stampWatermarkPDF(ctx, previewPath, target, watermarkText); err != nil {
		return "", err
	}
	return target, nil
}

func studentPreviewPageImage(ctx context.Context, sourcePath, assetID string, page int) (string, error) {
	stampedDir := filepath.Join(filepath.Dir(filepath.Dir(sourcePath)), "preview-stamped")
	if err := os.MkdirAll(stampedDir, 0755); err != nil {
		return "", err
	}
	target := filepath.Join(stampedDir, "page-"+assetID+"-"+strconv.Itoa(page)+"-"+time.Now().Format("150405.000000000")+".jpg")
	if err := rasterizePDFPage(ctx, sourcePath, target, page); err != nil {
		return "", err
	}
	return target, nil
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

func (h *LearningHandler) saveUploadedLearningFile(c *gin.Context) (learning.FileAsset, bool) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择要上传的文件")
		return learning.FileAsset{}, false
	}
	asset, err := saveLearningFile(file)
	if err != nil {
		BadRequest(c, err.Error())
		return learning.FileAsset{}, false
	}
	return asset, true
}

func saveLearningFile(file *multipart.FileHeader) (learning.FileAsset, error) {
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
	uploadRoot, err := filepath.Abs("uploads")
	if err != nil {
		return learning.FileAsset{}, errors.New("上传目录初始化失败")
	}
	originalDir := filepath.Join(uploadRoot, "original")
	previewDir := filepath.Join(uploadRoot, "preview")
	if err := os.MkdirAll(originalDir, 0755); err != nil {
		return learning.FileAsset{}, errors.New("上传目录创建失败")
	}
	if err := os.MkdirAll(previewDir, 0755); err != nil {
		return learning.FileAsset{}, errors.New("预览目录创建失败")
	}
	id := "file-" + time.Now().Format("20060102150405.000000000")
	originalPath := filepath.Join(originalDir, id+ext)
	if err := copyUpload(file, originalPath); err != nil {
		return learning.FileAsset{}, errors.New("文件保存失败")
	}
	previewPath, previewStatus := buildPreview(originalPath, previewDir, ext)
	return learning.FileAsset{
		ID:            id,
		FileName:      filepath.Base(file.Filename),
		FileSize:      file.Size,
		FileType:      spec.label,
		ContentType:   spec.contentType,
		OriginalPath:  originalPath,
		PreviewPath:   previewPath,
		PreviewStatus: previewStatus,
	}, nil
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

func buildPreview(originalPath, previewDir, ext string) (string, string) {
	if ext == ".pdf" {
		// 预览文件必须与原始文件在磁盘上物理分离：即使内容一开始相同，
		// 学生端也永远只可能拿到 preview/ 目录下的文件路径，任何代码路径的
		// 疏漏都不会意外把 original/ 目录下的原文件暴露出去。
		previewPath := filepath.Join(previewDir, filepath.Base(originalPath))
		if err := copyFile(originalPath, previewPath); err != nil {
			return "", "预览生成失败"
		}
		return previewPath, "可预览"
	}
	if _, err := exec.LookPath("soffice"); err != nil {
		return "", "预览生成失败"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "soffice", "--headless", "--convert-to", "pdf", "--outdir", previewDir, originalPath)
	if err := cmd.Run(); err != nil {
		return "", "预览生成失败"
	}
	previewPath := filepath.Join(previewDir, strings.TrimSuffix(filepath.Base(originalPath), filepath.Ext(originalPath))+".pdf")
	if _, err := os.Stat(previewPath); err != nil {
		return "", "预览生成失败"
	}
	return previewPath, "可预览"
}
