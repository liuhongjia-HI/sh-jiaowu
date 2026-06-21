package handler

import (
	"encoding/csv"
	"io"
	"strings"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/infrastructure/auth"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

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

func (h *LearningHandler) Health(c *gin.Context)    { OK(c, gin.H{"status": "ok"}) }
func (h *LearningHandler) Dashboard(c *gin.Context) { OK(c, h.service.Dashboard()) }
func (h *LearningHandler) Packages(c *gin.Context)  { OK(c, h.service.Packages()) }
func (h *LearningHandler) LearningSpaces(c *gin.Context) {
	OK(c, h.service.LearningSpaces())
}
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
func (h *LearningHandler) Students(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Students(principal, learning.StudentQuery{
		Keyword:        strings.TrimSpace(c.Query("keyword")),
		Grade:          strings.TrimSpace(c.Query("grade")),
		AccountStatus:  strings.TrimSpace(c.Query("accountStatus")),
		LearningStatus: strings.TrimSpace(c.Query("learningStatus")),
		PackageState:   strings.TrimSpace(c.Query("packageState")),
	}))
}
func (h *LearningHandler) StudentDetail(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	detail, err := h.service.StudentDetail(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, detail)
}
func (h *LearningHandler) CreateStudent(c *gin.Context) {
	req, ok := bindStudent(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateStudent(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateStudent(c *gin.Context) {
	req, ok := bindStudent(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateStudent(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}
func (h *LearningHandler) RemindStudent(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.RemindStudent(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}
func (h *LearningHandler) ImportStudents(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择导入文件")
		return
	}
	opened, err := file.Open()
	if err != nil {
		BadRequest(c, "导入文件读取失败")
		return
	}
	defer opened.Close()
	rows, err := parseStudentCSV(opened)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	OK(c, h.service.ImportStudents(operator.(string), principal, rows))
}
func (h *LearningHandler) StudentGrants(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	grants, err := h.service.StudentGrants(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, grants)
}
func (h *LearningHandler) StudentLearningRecords(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	records, err := h.service.StudentLearningRecords(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, records)
}
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
func (h *LearningHandler) Materials(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Materials(principal))
}
func (h *LearningHandler) Homework(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Homework(principal))
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
func (h *LearningHandler) Notices(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Notices(principal))
}
func (h *LearningHandler) CreateNotice(c *gin.Context) {
	var req learning.NoticeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Title = strings.TrimSpace(req.Title)
	req.Target = strings.TrimSpace(req.Target)
	req.Summary = strings.TrimSpace(req.Summary)
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	notice, err := h.service.CreateNotice(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, notice)
}
func (h *LearningHandler) Logs(c *gin.Context)     { OK(c, h.service.Logs()) }
func (h *LearningHandler) Settings(c *gin.Context) { OK(c, h.service.Settings()) }
func (h *LearningHandler) UpdateSetting(c *gin.Context) {
	var req learning.SettingUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.Key = strings.TrimSpace(req.Key)
	req.Value = strings.TrimSpace(req.Value)
	operator, _ := c.Get(middleware.OperatorNameKey)
	settings, err := h.service.UpdateSetting(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, settings)
}
func (h *LearningHandler) Teachers(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Teachers(principal))
}
func (h *LearningHandler) AdminStaff(c *gin.Context) {
	OK(c, h.service.AdminStaff())
}
func (h *LearningHandler) CreateAdminStaff(c *gin.Context) {
	req, ok := bindAdminStaff(c)
	if !ok {
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateAdminStaff(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateAdminStaff(c *gin.Context) {
	req, ok := bindAdminStaff(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateAdminStaff(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) ResetAdminStaffPassword(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.ResetPassword(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}

func (h *LearningHandler) CreateTeacher(c *gin.Context) {
	req, ok := bindTeacher(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateTeacher(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateTeacher(c *gin.Context) {
	req, ok := bindTeacher(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateTeacher(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) ResetTeacherPassword(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.ResetPassword(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
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
func (h *LearningHandler) WechatLogin(c *gin.Context) {
	var req struct {
		Code      string `json:"code"`
		Phone     string `json:"phone"`
		PhoneCode string `json:"phoneCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	key := loginKey(c, "wechat", req.Code)
	if err := h.loginProtector.Allow(key); err != nil {
		h.recordSecurityEvent(c, "登录拦截", req.Code, err.Error())
		Unauthorized(c, err.Error())
		return
	}
	principal, err := h.service.LoginWithWechatCode(req.Code, req.Phone, req.PhoneCode)
	if err != nil {
		h.loginProtector.RegisterFailure(key)
		h.recordSecurityEvent(c, "微信登录失败", req.Code, err.Error())
		Unauthorized(c, err.Error())
		return
	}
	h.loginProtector.RegisterSuccess(key)
	token, err := h.tokens.Issue(principal)
	if err != nil {
		BadRequest(c, "login failed")
		return
	}
	OK(c, learning.AuthResult{Token: token, User: principal})
}

func (h *LearningHandler) AdminPasswordLogin(c *gin.Context) {
	if !h.adminPasswordLoginEnabled {
		Unauthorized(c, "后台密码登录未启用")
		return
	}
	var req struct {
		Phone         string `json:"phone"`
		Password      string `json:"password"`
		CaptchaID     string `json:"captchaId"`
		CaptchaAnswer string `json:"captchaAnswer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	key := loginKey(c, "admin", req.Phone)
	if err := h.loginProtector.Allow(key); err != nil {
		h.recordSecurityEvent(c, "登录拦截", req.Phone, err.Error())
		Unauthorized(c, err.Error())
		return
	}
	if h.loginProtector.RequiresCaptcha(key) && !h.loginProtector.VerifyCaptcha(strings.TrimSpace(req.CaptchaID), strings.TrimSpace(req.CaptchaAnswer)) {
		h.loginProtector.RegisterFailure(key)
		h.recordSecurityEvent(c, "验证码校验失败", req.Phone, "后台登录需要验证码")
		Unauthorized(c, "请输入正确验证码")
		return
	}
	principal, err := h.service.LoginWithAdminPassword(req.Phone, req.Password)
	if err != nil {
		h.loginProtector.RegisterFailure(key)
		h.recordSecurityEvent(c, "后台登录失败", req.Phone, err.Error())
		Unauthorized(c, err.Error())
		return
	}
	h.loginProtector.RegisterSuccess(key)
	token, err := h.tokens.Issue(principal)
	if err != nil {
		BadRequest(c, "login failed")
		return
	}
	OK(c, learning.AuthResult{Token: token, User: principal})
}

func (h *LearningHandler) Captcha(c *gin.Context) {
	captcha, err := h.loginProtector.NewCaptcha()
	if err != nil {
		BadRequest(c, "验证码生成失败")
		return
	}
	OK(c, captcha)
}

func (h *LearningHandler) DemoStudentLogin(c *gin.Context) {
	if !h.demoStudentLoginEnabled {
		Unauthorized(c, "学生演示登录未启用")
		return
	}
	var req struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	key := loginKey(c, "student-demo", req.Phone)
	if err := h.loginProtector.Allow(key); err != nil {
		h.recordSecurityEvent(c, "登录拦截", req.Phone, err.Error())
		Unauthorized(c, err.Error())
		return
	}
	principal, err := h.service.LoginWithDemoStudentPassword(req.Phone, req.Password)
	if err != nil {
		h.loginProtector.RegisterFailure(key)
		h.recordSecurityEvent(c, "学生演示登录失败", req.Phone, err.Error())
		Unauthorized(c, err.Error())
		return
	}
	h.loginProtector.RegisterSuccess(key)
	token, err := h.tokens.Issue(principal)
	if err != nil {
		BadRequest(c, "login failed")
		return
	}
	OK(c, learning.AuthResult{Token: token, User: principal})
}

func loginKey(c *gin.Context, scope string, account string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		account = "anonymous"
	}
	return scope + ":" + c.ClientIP() + ":" + account
}

func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if len(header) > 7 && header[:7] == "Bearer " {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func (h *LearningHandler) recordSecurityEvent(c *gin.Context, action, target, detail string) {
	operator := middleware.AuditOperatorLabel("未登录用户", "", c.ClientIP(), c.Request.UserAgent())
	h.service.RecordSecurityEvent(operator, action, target, detail)
}

func (h *LearningHandler) Me(c *gin.Context) {
	principal, ok := middleware.CurrentPrincipal(c)
	if !ok {
		Unauthorized(c, "请先登录")
		return
	}
	OK(c, principal)
}

func (h *LearningHandler) ChangePassword(c *gin.Context) {
	var req learning.PasswordChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.ChangePassword(operator.(string), principal, req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"changed": true})
}

func (h *LearningHandler) Logout(c *gin.Context) {
	token := bearerToken(c)
	if token == "" {
		Unauthorized(c, "请先登录")
		return
	}
	if err := h.tokens.Revoke(token); err != nil {
		Unauthorized(c, "登录状态无效")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	h.service.RecordSecurityEvent(operator.(string), "退出登录", principal.Name, "当前 token 已作废")
	OK(c, gin.H{"loggedOut": true})
}

func (h *LearningHandler) RefreshToken(c *gin.Context) {
	token := bearerToken(c)
	if token == "" {
		Unauthorized(c, "请先登录")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	nextToken, err := h.tokens.Issue(principal)
	if err != nil {
		Unauthorized(c, "登录状态刷新失败")
		return
	}
	if err := h.tokens.Revoke(token); err != nil {
		Unauthorized(c, "登录状态无效")
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	h.service.RecordSecurityEvent(operator.(string), "刷新登录状态", principal.Name, "旧 token 已作废并签发新 token")
	OK(c, learning.AuthResult{Token: nextToken, User: principal})
}

func (h *LearningHandler) StudentHome(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	home, err := h.service.StudentHome(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, home)
}

func (h *LearningHandler) Availability(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	slots, err := h.service.Availability(principal, c.Query("ownerType"), c.Query("ownerId"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, slots)
}

func (h *LearningHandler) AvailabilityOverview(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.AvailabilityOverview(principal))
}

func (h *LearningHandler) SaveAvailability(c *gin.Context) {
	var req learning.AvailabilityUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	slots, err := h.service.SaveAvailability(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, slots)
}

func (h *LearningHandler) ScheduleCandidates(c *gin.Context) {
	var req learning.ScheduleCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	candidates, err := h.service.ScheduleCandidates(principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, candidates)
}

func (h *LearningHandler) ScheduleClasses(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.ScheduleClasses(principal))
}

func (h *LearningHandler) CreateScheduleClass(c *gin.Context) {
	var req learning.ScheduleClassCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	item, err := h.service.CreateScheduleClass(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, item)
}

func (h *LearningHandler) UpdateScheduleClass(c *gin.Context) {
	var req learning.ScheduleClassCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	item, err := h.service.UpdateScheduleClass(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, item)
}

func (h *LearningHandler) CancelScheduleClass(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	item, err := h.service.CancelScheduleClass(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, item)
}

func (h *LearningHandler) StudentAvailability(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	slots, err := h.service.Availability(principal, "student", principal.StudentID)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, slots)
}

func (h *LearningHandler) SaveStudentAvailability(c *gin.Context) {
	var req struct {
		Slots []learning.AvailabilitySlot `json:"slots"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	slots, err := h.service.SaveAvailability(operator.(string), principal, learning.AvailabilityUpsertRequest{
		OwnerType: "student",
		OwnerID:   principal.StudentID,
		Slots:     req.Slots,
	})
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, slots)
}

func (h *LearningHandler) StudentSchedule(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	classes, err := h.service.StudentSchedule(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, classes)
}
func (h *LearningHandler) StudentStudy(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	board, err := h.service.StudentStudy(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, board)
}
func (h *LearningHandler) StudentTasks(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	tasks, err := h.service.StudentTasks(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, tasks)
}
func (h *LearningHandler) StudentNotices(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	home, err := h.service.StudentHome(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, home.Notices)
}
func (h *LearningHandler) StudentMe(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	home, err := h.service.StudentHome(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, home.Student)
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
	var req struct {
		StudentID string `json:"studentId"`
		PackageID string `json:"packageId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	preview, err := h.service.CreateGrant(operator.(string), req.StudentID, req.PackageID)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, preview)
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

func (h *LearningHandler) StudentHomeworkDetail(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	homework, err := h.service.StudentHomework(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, homework)
}

func (h *LearningHandler) StudentSubmissionResult(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	submission, err := h.service.StudentSubmission(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, submission)
}

func (h *LearningHandler) StudentSubmission(c *gin.Context) {
	var req learning.SubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	submission, err := h.service.CreateSubmission(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{
		"submissionId": submission.ID,
		"status":       submission.Status,
		"score":        submission.Score,
	})
}

func (h *LearningHandler) StudentGrowth(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	records, err := h.service.StudentGrowth(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, records)
}

func (h *LearningHandler) StudentBadges(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	badges, err := h.service.StudentBadges(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, badges)
}

func (h *LearningHandler) StudentFavorites(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	favorites, err := h.service.StudentFavorites(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, favorites)
}

func (h *LearningHandler) AddFavorite(c *gin.Context) {
	var req learning.FavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	favorite, err := h.service.AddFavorite(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, favorite)
}

func (h *LearningHandler) RemoveFavorite(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.RemoveFavorite(operator.(string), principal, c.Param("id")); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"removed": true})
}

func bindTeacher(c *gin.Context) (learning.TeacherUpsertRequest, bool) {
	var req learning.TeacherUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return learning.TeacherUpsertRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	return req, true
}

func bindAdminStaff(c *gin.Context) (learning.AdminStaffUpsertRequest, bool) {
	var req learning.AdminStaffUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return learning.AdminStaffUpsertRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Role = learning.Role(strings.TrimSpace(string(req.Role)))
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	return req, true
}

func bindStudent(c *gin.Context) (learning.StudentUpsertRequest, bool) {
	var req learning.StudentUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return learning.StudentUpsertRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Grade = strings.TrimSpace(req.Grade)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	return req, true
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

func parseStudentCSV(reader io.Reader) ([]learning.StudentUpsertRequest, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	rows := make([]learning.StudentUpsertRequest, 0, len(records))
	for index, record := range records {
		if index == 0 && len(record) > 0 && strings.EqualFold(strings.TrimSpace(record[0]), "name") {
			continue
		}
		if len(record) < 3 {
			rows = append(rows, learning.StudentUpsertRequest{})
			continue
		}
		remark := ""
		if len(record) > 3 {
			remark = strings.TrimSpace(record[3])
		}
		rows = append(rows, learning.StudentUpsertRequest{
			Name:   strings.TrimSpace(record[0]),
			Phone:  strings.TrimSpace(record[1]),
			Grade:  strings.TrimSpace(record[2]),
			Remark: remark,
		})
	}
	return rows, nil
}
