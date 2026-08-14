package router

import (
	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/infrastructure/auth"
	"starline/learning-api/internal/interfaces/http/handler"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

// registerRoutes keeps route policy close to the route groups while New owns engine setup.
func registerRoutes(api *gin.RouterGroup, service *learningapp.Service, tokens *auth.TokenManager, h *handler.LearningHandler) {
	registerPublicRoutes(api, h)
	registerAuthenticatedRoutes(api, service, tokens, h)
	registerAdminRoutes(api, service, tokens, h)
	registerOpsRoutes(api, service, tokens, h)
	registerSystemRoutes(api, service, tokens, h)
	registerSuperRoutes(api, service, tokens, h)
	registerStudentRoutes(api, service, tokens, h)
}

func protected(api *gin.RouterGroup, service *learningapp.Service, tokens *auth.TokenManager, roles ...learning.Role) *gin.RouterGroup {
	return api.Group("", middleware.AuthRequired(tokens, service, roles...))
}

func registerPublicRoutes(api *gin.RouterGroup, h *handler.LearningHandler) {
	api.GET("/health", h.Health)
	// 学生头像由 image 组件直接读取，不能依赖 Authorization header；文件名使用不可预测随机值。
	api.GET("/student/avatars/:asset", h.StudentAvatar)
	api.POST("/auth/wechat-login", h.WechatLogin)
	api.POST("/auth/admin-password-login", h.AdminPasswordLogin)
	api.POST("/auth/demo-student-login", h.DemoStudentLogin)
	api.GET("/auth/captcha", h.Captcha)
}

func registerAuthenticatedRoutes(api *gin.RouterGroup, service *learningapp.Service, tokens *auth.TokenManager, h *handler.LearningHandler) {
	g := protected(api, service, tokens, learning.RoleStudent, learning.RoleTeacher, learning.RoleOpsStaff, learning.RoleCampusAdmin, learning.RoleSuperAdmin)
	g.GET("/auth/me", h.Me)
	g.POST("/auth/change-password", h.ChangePassword)
	g.POST("/auth/refresh", h.RefreshToken)
	g.POST("/auth/logout", h.Logout)
}

func registerAdminRoutes(api *gin.RouterGroup, service *learningapp.Service, tokens *auth.TokenManager, h *handler.LearningHandler) {
	g := protected(api, service, tokens, learning.RoleTeacher, learning.RoleOpsStaff, learning.RoleCampusAdmin, learning.RoleSuperAdmin)
	g.GET("/dashboard/overview", h.Dashboard)
	g.GET("/settings", h.Settings)
	g.GET("/packages", h.Packages)
	g.GET("/learning-spaces", h.LearningSpaces)
	g.GET("/students", h.Students)
	g.GET("/teachers", h.Teachers)
	g.GET("/students/:id", h.StudentDetail)
	g.GET("/students/:id/grants", h.StudentGrants)
	g.GET("/students/:id/learning-records", h.StudentLearningRecords)
	g.GET("/students/:id/scores", h.StudentScores)
	g.GET("/courses", h.Courses)
	g.POST("/courses", h.CreateCourse)
	g.PUT("/courses/:id", h.UpdateCourse)
	g.GET("/questions", h.Questions)
	g.POST("/questions", h.CreateQuestion)
	g.PUT("/questions/:id", h.UpdateQuestion)
	g.GET("/materials", h.Materials)
	g.POST("/materials", h.CreateMaterial)
	g.PUT("/materials/:id", h.UpdateMaterial)
	g.GET("/homework", h.Homework)
	g.GET("/homework/:id/submissions", h.HomeworkSubmissions)
	g.POST("/homework", h.CreateHomework)
	g.PUT("/homework/:id", h.UpdateHomework)
	g.GET("/files/:id/preview", h.PreviewFile)
	g.GET("/files/:id/download", h.DownloadFile)
	g.GET("/reviews/pending", h.Reviews)
	g.POST("/reviews/:id/complete", h.CompleteReview)
	g.POST("/students/:id/scores", h.CreateStudentScore)
	g.PUT("/students/:id/scores/:scoreId", h.UpdateStudentScore)
	g.GET("/notices", h.Notices)
	g.POST("/notices", h.CreateNotice)
	g.POST("/notices/:id/retry", h.RetryNotice)
	g.GET("/availability/overview", h.AvailabilityOverview)
	g.GET("/availability", h.Availability)
	g.PUT("/availability", h.SaveAvailability)
	g.GET("/schedule-classes", h.ScheduleClasses)
}

func registerOpsRoutes(api *gin.RouterGroup, service *learningapp.Service, tokens *auth.TokenManager, h *handler.LearningHandler) {
	g := protected(api, service, tokens, learning.RoleOpsStaff, learning.RoleCampusAdmin, learning.RoleSuperAdmin)
	g.GET("/permissions/students", h.StudentPermissions)
	g.GET("/permissions/packages", h.PackagePermissions)
	g.GET("/permissions/content", h.ContentPermissions)
	g.GET("/grants/preview", h.GrantPreview)
	g.POST("/grants", h.CreateGrant)
	g.POST("/packages", h.CreatePackage)
	g.PUT("/packages/:id", h.UpdatePackage)
	g.POST("/students", h.CreateStudent)
	g.PUT("/students/:id", h.UpdateStudent)
	g.POST("/students/:id/remind", h.RemindStudent)
	g.POST("/students/import", h.ImportStudents)
	g.GET("/commercial/summary", h.CommercialSummary)
	g.GET("/commercial/orders", h.CommercialOrders)
	g.POST("/commercial/orders", h.CreateCommercialOrder)
	g.POST("/commercial/orders/:id/payments", h.CreatePayment)
	g.POST("/commercial/orders/:id/refunds", h.CreateRefund)
	g.POST("/commercial/orders/:id/contracts", h.CreateContract)
	g.POST("/commercial/orders/:id/invoices", h.CreateInvoice)
	g.POST("/commercial/lesson-consumptions", h.CreateLessonConsumption)
	g.POST("/commercial/renewal-reminders", h.CreateRenewalReminder)
	g.POST("/commercial/parent-notices", h.CreateParentNotice)
	g.POST("/scheduling/candidates", h.ScheduleCandidates)
	g.POST("/schedule-classes", h.CreateScheduleClass)
	g.PUT("/schedule-classes/:id", h.UpdateScheduleClass)
	g.POST("/schedule-classes/:id/cancel", h.CancelScheduleClass)
}

func registerSystemRoutes(api *gin.RouterGroup, service *learningapp.Service, tokens *auth.TokenManager, h *handler.LearningHandler) {
	g := protected(api, service, tokens, learning.RoleCampusAdmin, learning.RoleSuperAdmin)
	g.POST("/teachers", h.CreateTeacher)
	g.PUT("/teachers/:id", h.UpdateTeacher)
	g.POST("/teachers/:id/reset-password", h.ResetTeacherPassword)
	g.GET("/logs", h.Logs)
	g.GET("/system/readiness", h.SystemReadiness)
	g.PUT("/settings", h.UpdateSetting)
}

func registerSuperRoutes(api *gin.RouterGroup, service *learningapp.Service, tokens *auth.TokenManager, h *handler.LearningHandler) {
	g := protected(api, service, tokens, learning.RoleSuperAdmin)
	g.GET("/admin-staff", h.AdminStaff)
	g.POST("/admin-staff", h.CreateAdminStaff)
	g.PUT("/admin-staff/:id", h.UpdateAdminStaff)
	g.POST("/admin-staff/:id/reset-password", h.ResetAdminStaffPassword)
}

func registerStudentRoutes(api *gin.RouterGroup, service *learningapp.Service, tokens *auth.TokenManager, h *handler.LearningHandler) {
	g := api.Group("/student", middleware.AuthRequired(tokens, service, learning.RoleStudent))
	g.GET("/home", h.StudentHome)
	g.GET("/recommendations", h.StudentRecommendations)
	g.POST("/subscription", h.ConfirmStudentSubscription)
	g.GET("/study", h.StudentStudy)
	g.GET("/study/:id", h.StudentCourseDetail)
	g.GET("/materials/:id", h.StudentMaterialDetail)
	g.GET("/materials/:id/preview", h.StudentMaterialPreview)
	g.GET("/materials/:id/preview/pages", h.StudentMaterialPreviewPages)
	g.GET("/materials/:id/preview/pages/:page", h.StudentMaterialPreviewPage)
	g.GET("/homework/:id", h.StudentHomeworkDetail)
	g.POST("/security/events", h.StudentSecurityEvent)
	g.GET("/tasks", h.StudentTasks)
	g.GET("/notices", h.StudentNotices)
	g.GET("/me", h.StudentMe)
	g.PUT("/profile", h.UpdateStudentProfile)
	g.POST("/profile/avatar", h.UploadStudentAvatar)
	g.GET("/availability", h.StudentAvailability)
	g.PUT("/availability", h.SaveStudentAvailability)
	g.GET("/schedule", h.StudentSchedule)
	g.POST("/submissions", h.StudentSubmission)
	g.GET("/submissions/:id", h.StudentSubmissionResult)
	g.GET("/growth", h.StudentGrowth)
	g.GET("/scores", h.StudentOwnScores)
	g.GET("/badges", h.StudentBadges)
	g.GET("/favorites", h.StudentFavorites)
	g.POST("/favorites", h.AddFavorite)
	g.DELETE("/favorites/:id", h.RemoveFavorite)
}
