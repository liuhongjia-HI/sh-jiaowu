package handler

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
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

func buildPreview(originalPath, previewDir, ext string) (string, string) {
	if ext == ".pdf" {
		return originalPath, "可预览"
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
