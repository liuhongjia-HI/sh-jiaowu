package handler

import (
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) CreatePackage(c *gin.Context) {
	req, ok := bindPackage(c)
	if !ok {
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreatePackage(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdatePackage(c *gin.Context) {
	req, ok := bindPackage(c)
	if !ok {
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdatePackage(operator.(string), c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) StudentPermissions(c *gin.Context) {
	OK(c, h.service.StudentPermissions())
}
func (h *LearningHandler) PackagePermissions(c *gin.Context) {
	OK(c, h.service.PackagePermissions())
}
func (h *LearningHandler) ContentPermissions(c *gin.Context) {
	OK(c, h.service.ContentPermissions())
}

func (h *LearningHandler) GrantPreview(c *gin.Context) {
	preview, err := h.service.GrantPreview(c.Query("studentId"), c.Query("packageId"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, preview)
}

func (h *LearningHandler) CreateGrant(c *gin.Context) {
	var req learning.GrantCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.StudentID = strings.TrimSpace(req.StudentID)
	req.PackageID = strings.TrimSpace(req.PackageID)
	req.StartsAt = strings.TrimSpace(req.StartsAt)
	req.EndsAt = strings.TrimSpace(req.EndsAt)
	operator, _ := c.Get(middleware.OperatorNameKey)
	preview, err := h.service.CreateGrant(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, preview)
}

func (h *LearningHandler) CreateDirectGrant(c *gin.Context) {
	var req learning.DirectGrantCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.StudentID = strings.TrimSpace(req.StudentID)
	req.LearningSpaceIDs = trimStringSlice(req.LearningSpaceIDs)
	req.ContentTypeCodes = trimStringSlice(req.ContentTypeCodes)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.CreateDirectGrant(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func bindPackage(c *gin.Context) (learning.PackageUpsertRequest, bool) {
	var req learning.PackageUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return learning.PackageUpsertRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.AcademicYear = strings.TrimSpace(req.AcademicYear)
	req.Grade = strings.TrimSpace(req.Grade)
	req.Semester = strings.TrimSpace(req.Semester)
	req.Subject = strings.TrimSpace(req.Subject)
	req.PhaseScope = strings.TrimSpace(req.PhaseScope)
	req.PackageType = strings.TrimSpace(req.PackageType)
	req.Summary = strings.TrimSpace(req.Summary)
	req.Status = learning.Status(strings.TrimSpace(string(req.Status)))
	req.LearningSpaceIDs = trimStringSlice(req.LearningSpaceIDs)
	req.ContentTypeCodes = trimStringSlice(req.ContentTypeCodes)
	return req, true
}

func trimStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
