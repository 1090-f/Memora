package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/modules/identity/application"
	"github.com/1090-f/Memora/internal/modules/identity/domain"
	"github.com/1090-f/Memora/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth *application.AuthService
}

func NewAuthHandler(auth *application.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func RegisterRoutes(router gin.IRouter, handler *AuthHandler, authRequired gin.HandlerFunc) {
	api := router.Group("/api/v1")
	api.POST("/auth/login", handler.Login)
	protected := api.Group("")
	protected.Use(authRequired)
	protected.POST("/auth/logout", handler.Logout)
	protected.GET("/users/me", handler.Me)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request struct {
		Account  string `json:"account" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Failure(c, contracts.AppError{Code: contracts.InvalidArgument})
		return
	}
	result, err := h.auth.Login(c.Request.Context(), request.Account, request.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			httpx.Failure(c, contracts.AppError{Code: contracts.Unauthorized})
			return
		}
		httpx.Failure(c, contracts.AppError{Code: contracts.InternalError})
		return
	}
	httpx.Success(c, http.StatusOK, loginResponse{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		ExpiresIn:   result.ExpiresIn,
		User:        loginUserResponseFrom(result.User),
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	tokenID, _ := c.Get(tokenIDContextKey)
	expiresAt, _ := c.Get(tokenExpiresAtContextKey)
	if err := h.auth.Logout(c.Request.Context(), tokenID.(string), expiresAt.(time.Time)); err != nil {
		httpx.Failure(c, contracts.AppError{Code: contracts.InternalError})
		return
	}
	httpx.Success(c, http.StatusOK, nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	value, ok := c.Get(userContextKey)
	user, valid := value.(*domain.User)
	if !ok || !valid {
		writeUnauthorized(c)
		return
	}
	httpx.Success(c, http.StatusOK, currentUserResponseFrom(*user))
}

type loginResponse struct {
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresIn   int64             `json:"expires_in"`
	User        loginUserResponse `json:"user"`
}

type loginUserResponse struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	Nickname  string  `json:"nickname"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
}

type currentUserResponse struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	Nickname  string  `json:"nickname"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	Bio       string  `json:"bio"`
}

func loginUserResponseFrom(user domain.User) loginUserResponse {
	return loginUserResponse{
		ID: user.ID, Username: user.Username, Nickname: user.DisplayNickname(),
		Email: user.Email, AvatarURL: user.AvatarURL,
	}
}

func currentUserResponseFrom(user domain.User) currentUserResponse {
	return currentUserResponse{
		ID: user.ID, Username: user.Username, Nickname: user.DisplayNickname(),
		Email: user.Email, AvatarURL: user.AvatarURL, Bio: user.Bio,
	}
}
