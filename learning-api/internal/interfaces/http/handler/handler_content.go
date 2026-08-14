package handler

import (
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) Courses(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Courses(principal))
}
func (h *LearningHandler) CreateCourse(c *gin.Context) {
	req, ok := bindCourse(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateCourse(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateCourse(c *gin.Context) {
	req, ok := bindCourse(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateCourse(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}
func (h *LearningHandler) Questions(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Questions(principal))
}
func (h *LearningHandler) CreateQuestion(c *gin.Context) {
	var req learning.QuestionBankUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateQuestion(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateQuestion(c *gin.Context) {
	var req learning.QuestionBankUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateQuestion(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}
func (h *LearningHandler) Materials(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Materials(principal))
}
func (h *LearningHandler) Homework(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Homework(principal))
}
func (h *LearningHandler) HomeworkSubmissions(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	summary, err := h.service.HomeworkSubmissions(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, summary)
}
func (h *LearningHandler) Reviews(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Reviews(principal))
}
func (h *LearningHandler) CompleteReview(c *gin.Context) {
	var req learning.ReviewCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.TeacherComment = strings.TrimSpace(req.TeacherComment)
	req.Reward = strings.TrimSpace(req.Reward)
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	submission, err := h.service.CompleteReview(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, submission)
}

func (h *LearningHandler) StudentCourseDetail(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	detail, err := h.service.StudentCourseDetail(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, detail)
}

func (h *LearningHandler) StudentMaterialDetail(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	material, err := h.service.StudentMaterial(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, material)
}

func bindCourse(c *gin.Context) (learning.CourseUpsertRequest, bool) {
	var req learning.CourseUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return learning.CourseUpsertRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.Status = learning.Status(strings.TrimSpace(string(req.Status)))
	return req, true
}
