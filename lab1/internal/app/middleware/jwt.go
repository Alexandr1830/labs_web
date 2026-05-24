package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"lab1/internal/app/auth"
)

const (
	// Роли в claims.
	RoleUser      = 0
	RoleModerator = 1
)

// extractToken — достаёт JWT либо из cookie "token", либо из Authorization: Bearer.
func extractToken(c *gin.Context) string {
	if tok, err := c.Cookie("token"); err == nil && tok != "" {
		return tok
	}
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// RequireAuth — обязывает наличие валидного JWT, не в blacklist.
// Кладёт в контекст user_id, role, jti.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := extractToken(c)
		if tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := auth.ParseToken(tok)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		// blacklist: если jti отозван logout'ом — 401
		if auth.IsBlacklisted(c.Request.Context(), claims.ID) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token revoked"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// RequireRole — пропускает только если в контексте роль не ниже минимальной.
// Использовать после RequireAuth.
func RequireRole(minRole int) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get("role")
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no role in context"})
			return
		}
		role, _ := v.(int)
		if role < minRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// JWTMiddleware — алиас для совместимости со старым handler.go.
func JWTMiddleware() gin.HandlerFunc {
	return RequireAuth()
}
