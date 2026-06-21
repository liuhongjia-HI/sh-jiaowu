package middleware

import (
	"encoding/base64"
	"encoding/json"
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
}

func RequestLogger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Infof("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
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
		if len(allowed) > 0 && !hasAnyRole(principal.Roles, allowed) {
			c.AbortWithStatusJSON(403, gin.H{"code": 403, "message": "没有权限访问该功能", "data": nil})
			return
		}
		if principal.MustChangePassword && !isPasswordBootstrapPath(c.Request.URL.Path) {
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
