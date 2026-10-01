package handler

import (
	"github.com/gin-gonic/gin"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"
)

func (h *LearningHandler) BusinessNoticeBindings(c *gin.Context) {
	OK(c, h.service.BusinessNoticeBindings())
}
func (h *LearningHandler) UpdateBusinessNoticeBinding(c *gin.Context) {
	var req learning.BusinessNoticeBinding
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	req.Kind = c.Param("kind")
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.UpdateBusinessNoticeBinding(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}
func (h *LearningHandler) BusinessNoticeTasks(c *gin.Context) { OK(c, h.service.BusinessNoticeTasks()) }
func (h *LearningHandler) RetryBusinessNotice(c *gin.Context) {
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.RetryBusinessNotice(operator.(string), c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}
func (h *LearningHandler) BusinessNoticeDetail(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	result, err := h.service.BusinessNoticeDetail(principal, c.Param("id"))
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	OK(c, result)
}
