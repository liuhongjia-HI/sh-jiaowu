package router

import (
	"time"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
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
	api := r.Group("/api")
	{
		api.GET("/health", h.Health)
		api.POST("/auth/wechat-login", h.WechatLogin)
		api.POST("/auth/admin-password-login", h.AdminPasswordLogin)
		api.POST("/auth/demo-student-login", h.DemoStudentLogin)
		api.GET("/auth/captcha", h.Captcha)

		authenticated := api.Group("", middleware.AuthRequired(tokens, dep.Service, learning.RoleStudent, learning.RoleTeacher, learning.RoleOpsStaff, learning.RoleCampusAdmin, learning.RoleSuperAdmin))
		{
			authenticated.GET("/auth/me", h.Me)
			authenticated.POST("/auth/change-password", h.ChangePassword)
			authenticated.POST("/auth/refresh", h.RefreshToken)
			authenticated.POST("/auth/logout", h.Logout)
		}

		admin := api.Group("", middleware.AuthRequired(tokens, dep.Service, learning.RoleTeacher, learning.RoleOpsStaff, learning.RoleCampusAdmin, learning.RoleSuperAdmin))
		{
			admin.GET("/dashboard/overview", h.Dashboard)
			admin.GET("/packages", h.Packages)
			admin.GET("/learning-spaces", h.LearningSpaces)
			admin.GET("/students", h.Students)
			admin.GET("/teachers", h.Teachers)
			admin.GET("/students/:id", h.StudentDetail)
			admin.GET("/students/:id/grants", h.StudentGrants)
			admin.GET("/students/:id/learning-records", h.StudentLearningRecords)
			admin.GET("/courses", h.Courses)
			admin.POST("/courses", h.CreateCourse)
			admin.PUT("/courses/:id", h.UpdateCourse)
			admin.GET("/materials", h.Materials)
			admin.POST("/materials", h.CreateMaterial)
			admin.PUT("/materials/:id", h.UpdateMaterial)
			admin.GET("/homework", h.Homework)
			admin.POST("/homework", h.CreateHomework)
			admin.PUT("/homework/:id", h.UpdateHomework)
			admin.GET("/files/:id/preview", h.PreviewFile)
			admin.GET("/files/:id/download", h.DownloadFile)
			admin.GET("/reviews/pending", h.Reviews)
			admin.POST("/reviews/:id/complete", h.CompleteReview)
			admin.GET("/notices", h.Notices)
			admin.POST("/notices", h.CreateNotice)
			admin.GET("/availability/overview", h.AvailabilityOverview)
			admin.GET("/availability", h.Availability)
			admin.PUT("/availability", h.SaveAvailability)
			admin.GET("/schedule-classes", h.ScheduleClasses)
		}

		ops := api.Group("", middleware.AuthRequired(tokens, dep.Service, learning.RoleOpsStaff, learning.RoleCampusAdmin, learning.RoleSuperAdmin))
		{
			ops.GET("/permissions/students", h.StudentPermissions)
			ops.GET("/permissions/packages", h.PackagePermissions)
			ops.GET("/permissions/content", h.ContentPermissions)
			ops.GET("/grants/preview", h.GrantPreview)
			ops.POST("/grants", h.CreateGrant)
			ops.POST("/packages", h.CreatePackage)
			ops.PUT("/packages/:id", h.UpdatePackage)
			ops.POST("/students", h.CreateStudent)
			ops.PUT("/students/:id", h.UpdateStudent)
			ops.POST("/students/:id/remind", h.RemindStudent)
			ops.POST("/students/import", h.ImportStudents)
			ops.GET("/commercial/summary", h.CommercialSummary)
			ops.GET("/commercial/orders", h.CommercialOrders)
			ops.POST("/commercial/orders", h.CreateCommercialOrder)
			ops.POST("/commercial/orders/:id/payments", h.CreatePayment)
			ops.POST("/commercial/orders/:id/refunds", h.CreateRefund)
			ops.POST("/commercial/orders/:id/contracts", h.CreateContract)
			ops.POST("/commercial/orders/:id/invoices", h.CreateInvoice)
			ops.POST("/commercial/lesson-consumptions", h.CreateLessonConsumption)
			ops.POST("/commercial/renewal-reminders", h.CreateRenewalReminder)
			ops.POST("/commercial/parent-notices", h.CreateParentNotice)
			ops.POST("/scheduling/candidates", h.ScheduleCandidates)
			ops.POST("/schedule-classes", h.CreateScheduleClass)
			ops.PUT("/schedule-classes/:id", h.UpdateScheduleClass)
			ops.POST("/schedule-classes/:id/cancel", h.CancelScheduleClass)
		}

		systemAdmin := api.Group("", middleware.AuthRequired(tokens, dep.Service, learning.RoleCampusAdmin, learning.RoleSuperAdmin))
		{
			systemAdmin.POST("/teachers", h.CreateTeacher)
			systemAdmin.PUT("/teachers/:id", h.UpdateTeacher)
			systemAdmin.POST("/teachers/:id/reset-password", h.ResetTeacherPassword)
			systemAdmin.GET("/logs", h.Logs)
			systemAdmin.GET("/settings", h.Settings)
			systemAdmin.PUT("/settings", h.UpdateSetting)
		}

		superAdmin := api.Group("", middleware.AuthRequired(tokens, dep.Service, learning.RoleSuperAdmin))
		{
			superAdmin.GET("/admin-staff", h.AdminStaff)
			superAdmin.POST("/admin-staff", h.CreateAdminStaff)
			superAdmin.PUT("/admin-staff/:id", h.UpdateAdminStaff)
			superAdmin.POST("/admin-staff/:id/reset-password", h.ResetAdminStaffPassword)
		}

		student := api.Group("/student", middleware.AuthRequired(tokens, dep.Service, learning.RoleStudent))
		{
			student.GET("/home", h.StudentHome)
			student.GET("/study", h.StudentStudy)
			student.GET("/study/:id", h.StudentCourseDetail)
			student.GET("/materials/:id", h.StudentMaterialDetail)
			student.GET("/homework/:id", h.StudentHomeworkDetail)
			student.GET("/tasks", h.StudentTasks)
			student.GET("/notices", h.StudentNotices)
			student.GET("/me", h.StudentMe)
			student.GET("/availability", h.StudentAvailability)
			student.PUT("/availability", h.SaveStudentAvailability)
			student.GET("/schedule", h.StudentSchedule)
			student.POST("/submissions", h.StudentSubmission)
			student.GET("/submissions/:id", h.StudentSubmissionResult)
			student.GET("/growth", h.StudentGrowth)
			student.GET("/badges", h.StudentBadges)
			student.GET("/favorites", h.StudentFavorites)
			student.POST("/favorites", h.AddFavorite)
			student.DELETE("/favorites/:id", h.RemoveFavorite)
		}
	}
	return r
}
