package handler

import (
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) Notices(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Notices(principal))
}
func (h *LearningHandler) CreateNotice(c *gin.Context) {
	var req learning.NoticeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Title = strings.TrimSpace(req.Title)
	req.Target = strings.TrimSpace(req.Target)
	req.Summary = strings.TrimSpace(req.Summary)
	req.Channel = strings.TrimSpace(req.Channel)
	req.RecipientOpenID = strings.TrimSpace(req.RecipientOpenID)
	req.RelatedType = strings.TrimSpace(req.RelatedType)
	req.RelatedID = strings.TrimSpace(req.RelatedID)
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	notice, err := h.service.CreateNotice(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, notice)
}
func (h *LearningHandler) RetryNotice(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	notice, err := h.service.RetryNotice(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, notice)
}
func (h *LearningHandler) Logs(c *gin.Context)     { OK(c, h.service.Logs()) }
func (h *LearningHandler) Settings(c *gin.Context) { OK(c, h.service.Settings()) }
func (h *LearningHandler) SystemReadiness(c *gin.Context) {
	OK(c, h.service.SystemReadiness())
}
func (h *LearningHandler) UpdateSetting(c *gin.Context) {
	var req learning.SettingUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.Key = strings.TrimSpace(req.Key)
	req.Value = strings.TrimSpace(req.Value)
	operator, _ := c.Get(middleware.OperatorNameKey)
	settings, err := h.service.UpdateSetting(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, settings)
}

func (h *LearningHandler) StudentNotices(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	home, err := h.service.StudentHome(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, home.Notices)
}
