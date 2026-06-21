package handler

import "github.com/gin-gonic/gin"

func OK(c *gin.Context, data any) {
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": data})
}

func BadRequest(c *gin.Context, message string) {
	c.JSON(400, gin.H{"code": 400, "message": message, "data": nil})
}

func Unauthorized(c *gin.Context, message string) {
	c.JSON(401, gin.H{"code": 401, "message": message, "data": nil})
}

func Forbidden(c *gin.Context, message string) {
	c.JSON(403, gin.H{"code": 403, "message": message, "data": nil})
}
