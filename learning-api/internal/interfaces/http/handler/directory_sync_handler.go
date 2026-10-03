package handler

import (
	"github.com/gin-gonic/gin"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"
)

func (h *LearningHandler) PreviewCourseDirectorySync(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	var req learning.CourseDirectorySyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "同步参数无效")
		return
	}
	result, err := h.service.PreviewCourseDirectorySync(p, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func (h *LearningHandler) SyncCourseDirectory(c *gin.Context) {
	p, _ := middleware.CurrentPrincipal(c)
	var req learning.CourseDirectorySyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "同步参数无效")
		return
	}
	o, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.SyncCourseDirectory(o.(string), p, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}
