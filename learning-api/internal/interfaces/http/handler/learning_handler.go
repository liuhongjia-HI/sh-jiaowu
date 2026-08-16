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
	fileStorageRoot           string
}

func NewLearningHandler(service *learningapp.Service, tokens *auth.TokenManager, loginProtector *auth.LoginProtector, adminPasswordLoginEnabled bool, demoStudentLoginEnabled bool, storageRoots ...string) *LearningHandler {
	storageRoot := "uploads"
	if len(storageRoots) > 0 && storageRoots[0] != "" {
		storageRoot = storageRoots[0]
	}
	return &LearningHandler{
		service:                   service,
		tokens:                    tokens,
		loginProtector:            loginProtector,
		adminPasswordLoginEnabled: adminPasswordLoginEnabled,
		demoStudentLoginEnabled:   demoStudentLoginEnabled,
		fileStorageRoot:           storageRoot,
	}
}
