package handler

import (
	"os"
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) TeachingPlans(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.TeachingPlans(p))
}

func (h *LearningHandler) TeachingPlanNotificationAudience(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	var req learning.TeachingPlanAudienceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "教案参数无效")
		return
	}
	rows, err := h.service.TeachingPlanNotificationAudience(p, req)
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	OK(c, rows)
}
func (h *LearningHandler) RecordTeachingPlanView(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	var req learning.TeachingPlanReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "阅读版本参数无效")
		return
	}
	if err := h.service.RecordTeachingPlanView(p, c.Param("id"), req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"read": true})
}

func (h *LearningHandler) UpdateTeachingPlanChapter(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	var req learning.TeachingPlanChapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "章节参数无效")
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	plan, err := h.service.UpdateTeachingPlanChapter(operator.(string), p, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, plan)
}

func (h *LearningHandler) TeachingPlan(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	plan, err := h.service.TeachingPlan(p, c.Param("id"))
	if err != nil {
		Forbidden(c, "没有权限查看该教案")
		return
	}
	OK(c, plan)
}

func (h *LearningHandler) CreateTeachingPlan(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	batchID := c.PostForm("batchId")
	if len(batchID) > 64 || batchID != strings.TrimSpace(batchID) {
		BadRequest(c, "上传批次编号无效")
		return
	}
	grade, subject := strings.TrimSpace(c.PostForm("grade")), strings.TrimSpace(c.PostForm("subject"))
	allowed := false
	for _, scope := range h.service.TeachingPlans(p).UploadScopes {
		if scope.Grade == grade && scope.Subject == subject {
			allowed = true
			break
		}
	}
	if !allowed {
		Forbidden(c, "没有权限上传该年级学科的教案")
		return
	}
	asset, ok := h.saveUploadedLearningFile(c)
	if !ok {
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateTeachingPlan(operator.(string), p, learning.TeachingPlanUploadRequest{BatchID: batchID, Title: strings.TrimSpace(c.PostForm("title")), Grade: grade, Subject: subject, CourseID: c.PostForm("courseId"), LessonID: c.PostForm("lessonId"), File: asset})
	if err != nil {
		_ = os.Remove(asset.OriginalPath)
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}

func (h *LearningHandler) TeachingPlanPreview(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	asset, err := h.service.TeachingPlanFile(p, c.Param("id"))
	if err != nil {
		Forbidden(c, "没有权限查看该教案")
		return
	}
	if asset.PreviewStatus != "可预览" || asset.PreviewPath == "" {
		BadRequest(c, "教案预览尚未生成，请稍后刷新")
		return
	}
	if _, err := os.Stat(asset.PreviewPath); err != nil {
		_ = h.service.MarkPreviewFileMissing(asset.ID, "预览文件已丢失，请重新生成预览")
		BadRequest(c, "教案预览文件已丢失，请重新生成")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", "inline; filename=preview.pdf")
	c.File(asset.PreviewPath)
}

func (h *LearningHandler) TeachingPlanDownload(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	if !p.CanDownloadTeacherMaterial() {
		Forbidden(c, "当前账号仅支持在线查看教案")
		return
	}
	asset, err := h.service.TeachingPlanFile(p, c.Param("id"))
	if err != nil {
		Forbidden(c, "没有权限查看该教案")
		return
	}
	if _, err := os.Stat(asset.OriginalPath); err != nil {
		BadRequest(c, "教案原文件不存在")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.FileAttachment(asset.OriginalPath, asset.FileName)
}

func (h *LearningHandler) RetryTeachingPlanPreview(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.RetryTeachingPlanPreview(operator.(string), p, c.Param("id")); err != nil {
		Forbidden(c, err.Error())
		return
	}
	OK(c, gin.H{"queued": true})
}

func (h *LearningHandler) PendingTeachingPlanNoticeBatches(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.PendingTeachingPlanNoticeBatches(p))
}
func (h *LearningHandler) CompleteTeachingPlanNoticeBatch(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.CompleteTeachingPlanNoticeBatch(operator.(string), p, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func (h *LearningHandler) TeachingPlanNotice(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	detail, err := h.service.TeachingPlanNotice(p, c.Param("id"))
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	OK(c, detail)
}
