package handler

import "github.com/gin-gonic/gin"

func (h *LearningHandler) Health(c *gin.Context)    { OK(c, gin.H{"status": "ok"}) }
func (h *LearningHandler) Dashboard(c *gin.Context) { OK(c, h.service.Dashboard()) }
func (h *LearningHandler) Packages(c *gin.Context)  { OK(c, h.service.Packages()) }
func (h *LearningHandler) LearningSpaces(c *gin.Context) {
	OK(c, h.service.LearningSpaces())
}
