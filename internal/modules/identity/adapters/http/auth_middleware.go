package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/modules/identity/application"
	"github.com/1090-f/Memora/internal/modules/identity/domain"
	"github.com/1090-f/Memora/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const (
	userIDContextKey         = "user_id"
	userContextKey           = "identity_user"
	tokenIDContextKey        = "identity_token_id"
	tokenExpiresAtContextKey = "identity_token_expires_at"
)

type AuthMiddleware struct {
	auth *application.AuthService
}

func NewAuthMiddleware(auth *application.AuthService) *AuthMiddleware {
	return &AuthMiddleware{auth: auth}
}

// AuthRequired validates the bearer token and injects UserID into the Gin context.
func (m *AuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			writeUnauthorized(c)
			return
		}
		user, claims, err := m.auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, domain.ErrUnauthorized) {
				writeUnauthorized(c)
				return
			}
			httpx.Failure(c, contracts.AppError{Code: contracts.InternalError})
			c.Abort()
			return
		}
		c.Set(userIDContextKey, user.ID)
		c.Set(userContextKey, user)
		c.Set(tokenIDContextKey, claims.TokenID)
		c.Set(tokenExpiresAtContextKey, time.Unix(claims.ExpiresAt, 0))
		c.Next()
	}
}

func UserIDFrom(c *gin.Context) string {
	value, _ := c.Get(userIDContextKey)
	userID, _ := value.(string)
	return userID
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func writeUnauthorized(c *gin.Context) {
	httpx.FailureWithStatus(c, http.StatusUnauthorized, contracts.AppError{Code: contracts.Unauthorized})
	c.Abort()
}
