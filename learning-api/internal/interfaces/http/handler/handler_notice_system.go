package handler

import (
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type officialAccountEventXML struct {
	FromUserName string `xml:"FromUserName"`
	Event        string `xml:"Event"`
	CreateTime   int64  `xml:"CreateTime"`
}

type officialAccountEncryptedXML struct {
	Encrypt string `xml:"Encrypt"`
}

func (h *LearningHandler) VerifyOfficialAccountCallback(c *gin.Context) {
	if !h.service.VerifyOfficialCallback(c.Query("signature"), c.Query("timestamp"), c.Query("nonce")) {
		c.String(http.StatusForbidden, "invalid signature")
		return
	}
	c.String(http.StatusOK, c.Query("echostr"))
}

func (h *LearningHandler) OfficialAccountCallback(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid xml")
		return
	}
	if c.Query("encrypt_type") == "aes" {
		var wrapper officialAccountEncryptedXML
		if err := xml.Unmarshal(raw, &wrapper); err != nil {
			c.String(http.StatusBadRequest, "invalid xml")
			return
		}
		raw, err = h.service.DecryptOfficialCallback(c.Query("msg_signature"), c.Query("timestamp"), c.Query("nonce"), wrapper.Encrypt)
		if err != nil {
			c.String(http.StatusForbidden, "invalid signature")
			return
		}
	} else if !h.service.VerifyOfficialCallback(c.Query("signature"), c.Query("timestamp"), c.Query("nonce")) {
		c.String(http.StatusForbidden, "invalid signature")
		return
	}
	var event officialAccountEventXML
	if err := xml.Unmarshal(raw, &event); err != nil {
		c.String(http.StatusBadRequest, "invalid xml")
		return
	}
	if err := h.service.HandleOfficialCallback(event.FromUserName, event.Event, event.CreateTime); err != nil {
		c.String(http.StatusInternalServerError, "failed")
		return
	}
	c.String(http.StatusOK, "success")
}

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

func (h *LearningHandler) WechatSettings(c *gin.Context) { OK(c, h.service.WechatSettings()) }

func (h *LearningHandler) UpdateWechatSettings(c *gin.Context) {
	var req learning.WechatSettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	settings, err := h.service.UpdateWechatSettings(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, settings)
}

func (h *LearningHandler) OfficialTemplates(c *gin.Context) { OK(c, h.service.OfficialTemplates()) }

func (h *LearningHandler) SyncOfficialTemplates(c *gin.Context) {
	operator, _ := c.Get(middleware.OperatorNameKey)
	templates, err := h.service.SyncOfficialTemplates(operator.(string))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, templates)
}

func (h *LearningHandler) SyncOfficialFollowers(c *gin.Context) {
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.SyncOfficialFollowers(operator.(string))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func (h *LearningHandler) PreviewOfficialAudience(c *gin.Context) {
	var req learning.OfficialAudiencePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	preview, err := h.service.PreviewOfficialAudience(req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, preview)
}

func (h *LearningHandler) OfficialCampaigns(c *gin.Context) { OK(c, h.service.OfficialCampaigns()) }

func (h *LearningHandler) OfficialCampaign(c *gin.Context) {
	detail, err := h.service.OfficialCampaign(c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, detail)
}

func (h *LearningHandler) CreateOfficialCampaign(c *gin.Context) {
	var req learning.OfficialCampaignCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	campaign, err := h.service.CreateOfficialCampaign(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, campaign)
}

func (h *LearningHandler) RetryOfficialCampaign(c *gin.Context) {
	operator, _ := c.Get(middleware.OperatorNameKey)
	campaign, err := h.service.RetryOfficialCampaign(operator.(string), c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, campaign)
}

func (h *LearningHandler) Subjects(c *gin.Context) { OK(c, h.service.Subjects()) }

func (h *LearningHandler) UpdateSubjectMetadata(c *gin.Context) {
	var req learning.SubjectMetadataUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	subject, err := h.service.UpdateSubjectMetadata(operator.(string), c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, subject)
}

func (h *LearningHandler) DeleteSubjectMetadata(c *gin.Context) {
	operator, _ := c.Get(middleware.OperatorNameKey)
	id := c.Param("id")
	if err := h.service.DeleteSubjectMetadata(operator.(string), id); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"id": id})
}

func (h *LearningHandler) GradeSubjects(c *gin.Context) { OK(c, h.service.GradeSubjects()) }

func (h *LearningHandler) UpdateGradeSubjects(c *gin.Context) {
	var req learning.GradeSubjectCatalogUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	items, err := h.service.UpdateGradeSubjects(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, items)
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

func (h *LearningHandler) MarkStudentNoticeRead(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	notice, err := h.service.MarkStudentNoticeRead(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, notice)
}

func (h *LearningHandler) MarkAllStudentNoticesRead(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	notices, err := h.service.MarkAllStudentNoticesRead(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, notices)
}
