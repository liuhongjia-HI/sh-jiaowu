package handler

import (
	"errors"
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) WechatLogin(c *gin.Context) {
	var req learning.WechatLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Phone = strings.TrimSpace(req.Phone)
	req.PhoneCode = strings.TrimSpace(req.PhoneCode)
	req.StudentName = strings.TrimSpace(req.StudentName)
	req.SchoolName = strings.TrimSpace(req.SchoolName)
	req.Grade = strings.TrimSpace(req.Grade)
	key := loginKey(c, "wechat", req.Code)
	if err := h.loginProtector.Allow(key); err != nil {
		if auditErr := h.recordSecurityEvent(c, "登录拦截", req.Code, err.Error()); auditErr != nil {
			BadRequest(c, auditErr.Error())
			return
		}
		Unauthorized(c, err.Error())
		return
	}
	principal, err := h.service.LoginWithWechatCode(req)
	if err != nil {
		var selectionErr *learning.StudentSelectionRequiredError
		if errors.As(err, &selectionErr) {
			// 多子女：手机号命中多个学生档案，需要家长选一个再重新提交，
			// 这是正常的登录步骤而不是失败，不计入登录失败限流也不记安全审计。
			h.loginProtector.RegisterSuccess(key)
			OK(c, gin.H{"needsSelection": true, "candidates": selectionErr.Candidates})
			return
		}
		h.loginProtector.RegisterFailure(key)
		if auditErr := h.recordSecurityEvent(c, "微信登录失败", req.Code, err.Error()); auditErr != nil {
			BadRequest(c, auditErr.Error())
			return
		}
		Unauthorized(c, err.Error())
		return
	}
	h.loginProtector.RegisterSuccess(key)
	principal.AuthMethod = "wechat"
	token, err := h.tokens.Issue(principal)
	if err != nil {
		BadRequest(c, "login failed")
		return
	}
	OK(c, learning.AuthResult{Token: token, User: principal, AuthMethod: principal.AuthMethod})
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
		if auditErr := h.recordSecurityEvent(c, "登录拦截", req.Phone, err.Error()); auditErr != nil {
			BadRequest(c, auditErr.Error())
			return
		}
		Unauthorized(c, err.Error())
		return
	}
	if h.loginProtector.RequiresCaptcha(key) && !h.loginProtector.VerifyCaptcha(strings.TrimSpace(req.CaptchaID), strings.TrimSpace(req.CaptchaAnswer)) {
		h.loginProtector.RegisterFailure(key)
		if auditErr := h.recordSecurityEvent(c, "验证码校验失败", req.Phone, "后台登录需要验证码"); auditErr != nil {
			BadRequest(c, auditErr.Error())
			return
		}
		Unauthorized(c, "请输入正确验证码")
		return
	}
	principal, err := h.service.LoginWithAdminPassword(req.Phone, req.Password)
	if err != nil {
		h.loginProtector.RegisterFailure(key)
		if auditErr := h.recordSecurityEvent(c, "后台登录失败", req.Phone, err.Error()); auditErr != nil {
			BadRequest(c, auditErr.Error())
			return
		}
		Unauthorized(c, err.Error())
		return
	}
	h.loginProtector.RegisterSuccess(key)
	principal.AuthMethod = "password"
	token, err := h.tokens.Issue(principal)
	if err != nil {
		BadRequest(c, "login failed")
		return
	}
	OK(c, learning.AuthResult{Token: token, User: principal, AuthMethod: principal.AuthMethod})
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
		if auditErr := h.recordSecurityEvent(c, "登录拦截", req.Phone, err.Error()); auditErr != nil {
			BadRequest(c, auditErr.Error())
			return
		}
		Unauthorized(c, err.Error())
		return
	}
	principal, err := h.service.LoginWithDemoStudentPassword(req.Phone, req.Password)
	if err != nil {
		h.loginProtector.RegisterFailure(key)
		if auditErr := h.recordSecurityEvent(c, "学生演示登录失败", req.Phone, err.Error()); auditErr != nil {
			BadRequest(c, auditErr.Error())
			return
		}
		Unauthorized(c, err.Error())
		return
	}
	h.loginProtector.RegisterSuccess(key)
	principal.AuthMethod = "demo"
	token, err := h.tokens.Issue(principal)
	if err != nil {
		BadRequest(c, "login failed")
		return
	}
	OK(c, learning.AuthResult{Token: token, User: principal, AuthMethod: principal.AuthMethod})
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

func (h *LearningHandler) recordSecurityEvent(c *gin.Context, action, target, detail string) error {
	operator := middleware.AuditOperatorLabel("未登录用户", "", c.ClientIP(), c.Request.UserAgent())
	return h.service.RecordSecurityEvent(operator, action, target, detail)
}

func (h *LearningHandler) Me(c *gin.Context) {
	principal, ok := middleware.CurrentPrincipal(c)
	if !ok {
		Unauthorized(c, "请先登录")
		return
	}
	OK(c, principal)
}

func (h *LearningHandler) StudentAccounts(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	accounts, err := h.service.StudentAccounts(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, accounts)
}

func (h *LearningHandler) SwitchStudentAccount(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	next, err := h.service.SwitchStudentAccount(principal, c.Param("id"))
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	token, err := h.tokens.Issue(next)
	if err != nil {
		BadRequest(c, "切换账号失败")
		return
	}
	OK(c, learning.AuthResult{Token: token, User: next, AuthMethod: next.AuthMethod})
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
	if err := h.service.RecordSecurityEvent(operator.(string), "退出登录", principal.Name, "当前 token 已作废"); err != nil {
		BadRequest(c, err.Error())
		return
	}
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
	if err := h.service.RecordSecurityEvent(operator.(string), "刷新登录状态", principal.Name, "旧 token 已作废并签发新 token"); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, learning.AuthResult{Token: nextToken, User: principal, AuthMethod: principal.AuthMethod})
}
