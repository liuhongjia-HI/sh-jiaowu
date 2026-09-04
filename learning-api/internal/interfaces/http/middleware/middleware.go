package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/infrastructure/auth"
	"starline/learning-api/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
)

const OperatorNameKey = "operator_name"
const OperatorIDKey = "operator_id"
const PrincipalKey = "principal"
const auditOperatorPrefix = "audit:"

type auditOperator struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
}

type PrincipalResolver interface {
	PrincipalByUserID(userID string) (learning.Principal, error)
	// GuardianStudentActive 校验家长-学生关系是否仍然"在读"。PrincipalByUserID
	// 是从 users/roles 表重建 principal 的权威来源，但它不知道 GuardianID——
	// 这个字段是家长身份特有的，必须单独从 token 里取出来后再逐请求校验一次，
	// 否则关系被后台解除之后，旧 token 还能继续读那个孩子的数据。
	GuardianStudentActive(guardianID, studentID string) bool
}

func RequestLogger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Infof("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
	}
}

func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = true
		}
	}
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if allowed[origin] {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Vary", "Origin")
			header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Operator-ID, X-Operator-Name")
			header.Set("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func AuthRequired(tokens *auth.TokenManager, resolver PrincipalResolver, roles ...learning.Role) gin.HandlerFunc {
	allowed := make(map[learning.Role]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token := ""
		if len(header) > 7 && header[:7] == "Bearer " {
			token = header[7:]
		}
		tokenPrincipal, err := tokens.Parse(token)
		if err != nil {
			log.Printf("event=auth_denied path=%s status=401 reason=token_parse token_sha256=%s", c.Request.URL.Path, tokenFingerprint(token))
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "请先登录", "data": nil})
			return
		}
		principal, err := resolver.PrincipalByUserID(tokenPrincipal.UserID)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": err.Error(), "data": nil})
			return
		}
		if tokenPrincipal.TokenVersion != principal.TokenVersion {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "登录状态已更新，请重新登录", "data": nil})
			return
		}
		if tokenPrincipal.GuardianID != "" {
			// GuardianID/当前查看的学生 是家长登录特有的状态，PrincipalByUserID
			// 从 users 表重建 principal 时并不知道它，必须从 token 里取出来并且
			// 每次请求都重新校验关系还在——这是切换账号能不能被后台实时收回权限
			// 的关键一步，不能只在切换的那一刻校验一次就完事。
			if !resolver.GuardianStudentActive(tokenPrincipal.GuardianID, principal.StudentID) {
				c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "登录状态已更新，请重新登录", "data": nil})
				return
			}
			principal.GuardianID = tokenPrincipal.GuardianID
		}
		principal.AuthMethod = tokenPrincipal.AuthMethod
		if len(allowed) > 0 && !hasAnyRole(principal.Roles, allowed) {
			log.Printf("event=auth_denied path=%s status=403 reason=role_mismatch user_id=%s student_id=%s roles=%v required_roles=%v token_sha256=%s", c.Request.URL.Path, principal.UserID, principal.StudentID, principal.Roles, roles, tokenFingerprint(token))
			c.AbortWithStatusJSON(403, gin.H{"code": 403, "message": "没有权限访问该功能", "data": nil})
			return
		}
		if principal.MustChangePassword && principal.AuthMethod == "password" && !isPasswordBootstrapPath(c.Request.URL.Path) {
			c.AbortWithStatusJSON(403, gin.H{"code": 403, "message": "请先修改初始密码", "data": nil})
			return
		}
		c.Set(PrincipalKey, principal)
		operatorName, _ := c.Get(OperatorNameKey)
		name, _ := operatorName.(string)
		if name == "" || name == "本地开发" || strings.HasPrefix(name, auditOperatorPrefix) {
			name = principal.Name
		}
		operatorID, _ := c.Get(OperatorIDKey)
		id, _ := operatorID.(string)
		if id == "" {
			id = principal.UserID
		}
		c.Set(OperatorNameKey, AuditOperatorLabel(name, id, c.ClientIP(), c.Request.UserAgent()))
		c.Set(OperatorIDKey, id)
		c.Next()
	}
}

// tokenFingerprint lets operators correlate repeated failures without logging
// the bearer token itself.
func tokenFingerprint(token string) string {
	if token == "" {
		return "empty"
	}
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:8])
}

func isPasswordBootstrapPath(path string) bool {
	return path == "/api/auth/me" || path == "/api/auth/change-password" || path == "/api/auth/logout"
}

func CurrentPrincipal(c *gin.Context) (learning.Principal, bool) {
	value, ok := c.Get(PrincipalKey)
	if !ok {
		return learning.Principal{}, false
	}
	principal, ok := value.(learning.Principal)
	return principal, ok
}

func hasAnyRole(roles []learning.Role, allowed map[learning.Role]bool) bool {
	for _, role := range roles {
		if allowed[role] {
			return true
		}
	}
	return false
}

func OperatorContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Operator-ID"))
		name := decodeHeaderValue(c.GetHeader("X-Operator-Name"))
		if name == "" {
			name = "本地开发"
		}
		c.Set(OperatorIDKey, id)
		c.Set(OperatorNameKey, AuditOperatorLabel(name, id, c.ClientIP(), c.Request.UserAgent()))
		c.Next()
	}
}

func AuditOperatorLabel(name, id, ip, userAgent string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "本地开发"
	}
	payload, err := json.Marshal(auditOperator{
		Name:      name,
		ID:        strings.TrimSpace(id),
		IP:        strings.TrimSpace(ip),
		UserAgent: strings.TrimSpace(userAgent),
	})
	if err != nil {
		return name
	}
	return auditOperatorPrefix + base64.RawURLEncoding.EncodeToString(payload)
}

func decodeHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if decoded, err := url.QueryUnescape(value); err == nil && decoded != "" {
		return decoded
	}
	return value
}
