package middleware

import (
	"strings"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/service"
	jwtmanager "github.com/1090-f/Memora/pkg/jwt"
	"github.com/gin-gonic/gin"
)

const (
	contextUserKey   = "auth_user"
	contextClaimsKey = "auth_claims"
)

// Auth 返回一个 JWT 认证中间件，从 Authorization 头提取 Token 并验证用户身份。
func Auth(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Failure(c, apperrors.ErrUnauthorized)
			c.Abort()
			return
		}
		user, claims, err := authService.Authenticate(c.Request.Context(), parts[1])
		if err != nil {
			response.Failure(c, err)
			c.Abort()
			return
		}
		c.Set(contextUserKey, user)
		c.Set(contextClaimsKey, claims)
		c.Next()
	}
}

// GetUser 从 Gin 上下文中获取已认证的用户信息。
func GetUser(c *gin.Context) (*entity.User, bool) {
	value, ok := c.Get(contextUserKey)
	user, valid := value.(*entity.User)
	return user, ok && valid
}

// GetClaims 从 Gin 上下文中获取 JWT 的 Claims 信息。
func GetClaims(c *gin.Context) (*jwtmanager.Claims, bool) {
	value, ok := c.Get(contextClaimsKey)
	claims, valid := value.(*jwtmanager.Claims)
	return claims, ok && valid
}
