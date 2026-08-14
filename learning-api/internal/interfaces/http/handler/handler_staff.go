package handler

import (
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) Teachers(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Teachers(principal))
}
func (h *LearningHandler) AdminStaff(c *gin.Context) {
	OK(c, h.service.AdminStaff())
}
func (h *LearningHandler) CreateAdminStaff(c *gin.Context) {
	req, ok := bindAdminStaff(c)
	if !ok {
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateAdminStaff(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateAdminStaff(c *gin.Context) {
	req, ok := bindAdminStaff(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateAdminStaff(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) ResetAdminStaffPassword(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.ResetPassword(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func (h *LearningHandler) CreateTeacher(c *gin.Context) {
	req, ok := bindTeacher(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateTeacher(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateTeacher(c *gin.Context) {
	req, ok := bindTeacher(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateTeacher(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) ResetTeacherPassword(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.ResetPassword(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func bindTeacher(c *gin.Context) (learning.TeacherUpsertRequest, bool) {
	var req learning.TeacherUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return learning.TeacherUpsertRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	return req, true
}

func bindAdminStaff(c *gin.Context) (learning.AdminStaffUpsertRequest, bool) {
	var req learning.AdminStaffUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return learning.AdminStaffUpsertRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Role = learning.Role(strings.TrimSpace(string(req.Role)))
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	return req, true
}
