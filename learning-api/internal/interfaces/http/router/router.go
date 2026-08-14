package router

import (
	"time"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/infrastructure/auth"
	"starline/learning-api/internal/infrastructure/config"
	"starline/learning-api/internal/infrastructure/logger"
	"starline/learning-api/internal/interfaces/http/handler"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Config  *config.Config
	Logger  *logger.Logger
	Service *learningapp.Service
}

func New(dep Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(dep.Config.CORS.AllowedOrigins))
	r.Use(middleware.RequestLogger(dep.Logger))
	r.Use(middleware.OperatorContext())

	tokens := auth.NewTokenManager(dep.Config.Auth.TokenSecret, 24*time.Hour)
	h := handler.NewLearningHandler(dep.Service, tokens, auth.NewLoginProtector(), dep.Config.Demo.AdminPasswordLogin, dep.Config.Demo.StudentPasswordLogin)
	registerRoutes(r.Group("/api"), dep.Service, tokens, h)
	return r
}
