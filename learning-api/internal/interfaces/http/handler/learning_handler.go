package handler

import (
	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/infrastructure/auth"
)

// LearningHandler owns HTTP-facing dependencies shared by domain handlers.
type LearningHandler struct {
	service                   *learningapp.Service
	tokens                    *auth.TokenManager
	loginProtector            *auth.LoginProtector
	adminPasswordLoginEnabled bool
	demoStudentLoginEnabled   bool
}

func NewLearningHandler(service *learningapp.Service, tokens *auth.TokenManager, loginProtector *auth.LoginProtector, adminPasswordLoginEnabled bool, demoStudentLoginEnabled bool) *LearningHandler {
	return &LearningHandler{
		service:                   service,
		tokens:                    tokens,
		loginProtector:            loginProtector,
		adminPasswordLoginEnabled: adminPasswordLoginEnabled,
		demoStudentLoginEnabled:   demoStudentLoginEnabled,
	}
}
