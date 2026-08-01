package middleware

import (
	"strings"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/service"
	apperrors "github.com/1090-f/Memora/pkg/errors"
	jwtmanager "github.com/1090-f/Memora/pkg/jwt"
	"github.com/1090-f/Memora/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	contextUserKey   = "auth_user"
	contextClaimsKey = "auth_claims"
)

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

func GetUser(c *gin.Context) (*entity.User, bool) {
	value, ok := c.Get(contextUserKey)
	user, valid := value.(*entity.User)
	return user, ok && valid
}

func GetClaims(c *gin.Context) (*jwtmanager.Claims, bool) {
	value, ok := c.Get(contextClaimsKey)
	claims, valid := value.(*jwtmanager.Claims)
	return claims, ok && valid
}
