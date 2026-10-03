package handler

import (
	"github.com/gin-gonic/gin"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"
)

func (h *LearningHandler) MaterialDownloadSelection(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	var req learning.MaterialDownloadScope
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "下载范围无效")
		return
	}
	quote, err := h.service.MaterialDownloadSelection(p, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, quote)
}
func (h *LearningHandler) CreateMaterialDownload(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	var req learning.MaterialDownloadScope
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "下载范围无效")
		return
	}
	job, err := h.service.CreateMaterialDownload(p, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, job)
}
func (h *LearningHandler) MaterialDownloads(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.MaterialDownloads(p))
}
func (h *LearningHandler) RetryMaterialDownload(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	job, err := h.service.RetryMaterialDownload(p, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, job)
}
func (h *LearningHandler) MaterialDownloadArchive(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	archive, err := h.service.MaterialDownloadArchive(p, c.Param("id"))
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	stored, err := secureMaterialArchivePath(h.fileStorageRoot, archive)
	if err != nil {
		if persistErr := h.service.InvalidateMaterialDownload(p, c.Param("id")); persistErr != nil {
			BadRequest(c, "下载包文件不可用，请刷新后重试")
			return
		}
		BadRequest(c, "下载包文件不可用，请重新生成")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.FileAttachment(stored, "课程讲义.zip")
}
