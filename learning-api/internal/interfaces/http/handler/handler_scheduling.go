package handler

import (
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) Availability(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	slots, err := h.service.Availability(principal, c.Query("ownerType"), c.Query("ownerId"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, slots)
}

func (h *LearningHandler) AvailabilityOverview(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.AvailabilityOverview(principal))
}

func (h *LearningHandler) SaveAvailability(c *gin.Context) {
	var req learning.AvailabilityUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	slots, err := h.service.SaveAvailability(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, slots)
}

func (h *LearningHandler) ScheduleCandidates(c *gin.Context) {
	var req learning.ScheduleCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	candidates, err := h.service.ScheduleCandidates(principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, candidates)
}

func (h *LearningHandler) ScheduleClasses(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.ScheduleClasses(principal))
}

func (h *LearningHandler) LessonFeedbacks(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	items, err := h.service.LessonFeedbacks(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, items)
}

func (h *LearningHandler) UpsertLessonFeedback(c *gin.Context) {
	var req learning.LessonFeedbackUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	item, err := h.service.UpsertLessonFeedback(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, item)
}

func (h *LearningHandler) CreateScheduleClass(c *gin.Context) {
	var req learning.ScheduleClassCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	item, err := h.service.CreateScheduleClass(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, item)
}

func (h *LearningHandler) UpdateScheduleClass(c *gin.Context) {
	var req learning.ScheduleClassCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	item, err := h.service.UpdateScheduleClass(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, item)
}

func (h *LearningHandler) CancelScheduleClass(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	item, err := h.service.CancelScheduleClass(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, item)
}

func (h *LearningHandler) StudentAvailability(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	slots, err := h.service.Availability(principal, "student", principal.StudentID)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, slots)
}

func (h *LearningHandler) SaveStudentAvailability(c *gin.Context) {
	var req struct {
		Slots []learning.AvailabilitySlot `json:"slots"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	slots, err := h.service.SaveAvailability(operator.(string), principal, learning.AvailabilityUpsertRequest{
		OwnerType: "student",
		OwnerID:   principal.StudentID,
		Slots:     req.Slots,
	})
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, slots)
}

func (h *LearningHandler) StudentSchedule(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	classes, err := h.service.StudentSchedule(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, classes)
}

func (h *LearningHandler) PendingScheduleClasses(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.PendingScheduleClasses(principal))
}

func (h *LearningHandler) ApproveScheduleClass(c *gin.Context) {
	h.reviewScheduleClass(c, true)
}

func (h *LearningHandler) RejectScheduleClass(c *gin.Context) {
	h.reviewScheduleClass(c, false)
}

func (h *LearningHandler) reviewScheduleClass(c *gin.Context, approve bool) {
	var req learning.ScheduleAuditRequest
	// 通过时可以不带 body，驳回理由由 store 层校验必填。
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			BadRequest(c, "invalid request")
			return
		}
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	item, err := h.service.ReviewScheduleClass(operator.(string), principal, c.Param("id"), approve, req.Reason)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, item)
}
