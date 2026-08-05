package auth

import (
	"net/http"

	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/audit"
	apperrors "github.com/1090-f/Memora/pkg/errors"
	"github.com/1090-f/Memora/pkg/response"
	"github.com/gin-gonic/gin"
)

// Controller 处理认证相关的 HTTP 请求。
type Controller struct{ auth service.AuthService }

// NewController 创建一个新的认证控制器实例。
func NewController(auth service.AuthService) *Controller { return &Controller{auth: auth} }

// Login 处理用户登录请求，验证凭据并返回 JWT Token。
func (ctrl *Controller) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.auth.Login(c.Request.Context(), &req)
	if err != nil {
		audit.Record("auth.login", "", "session", middleware.GetRequestID(c), middleware.GetTraceID(c), "denied")
		response.Failure(c, err)
		return
	}
	audit.Record("auth.login", result.User.ID, "session", middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, result)
}

// Logout 处理用户登出请求，吊销 JWT Token。
func (ctrl *Controller) Logout(c *gin.Context) {
	claims, ok := middleware.GetClaims(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	if err := ctrl.auth.Logout(c.Request.Context(), claims); err != nil {
		response.Failure(c, err)
		return
	}
	audit.Record("auth.logout", claims.Subject, "session", middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, nil)
}
