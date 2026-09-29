package handler

import (
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) CourseFamilies(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.CourseFamilies(principal))
}

func (h *LearningHandler) CreateCourseFamily(c *gin.Context) {
	var req learning.CourseFamilyCreateRequest
	if c.ShouldBindJSON(&req) != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.CreateCourseFamily(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func (h *LearningHandler) ImportCourseFamily(c *gin.Context) {
	var req learning.CourseFamilyImportRequest
	if c.ShouldBindJSON(&req) != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.ImportCourseFamily(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func (h *LearningHandler) UpdateCourseFamily(c *gin.Context) {
	var req learning.CourseFamilyUpdateRequest
	if c.ShouldBindJSON(&req) != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.UpdateCourseFamily(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func (h *LearningHandler) AddCourseFamilyCourse(c *gin.Context) {
	var req learning.CourseFamilyAddCourseRequest
	if c.ShouldBindJSON(&req) != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.AddCourseFamilyCourse(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}
