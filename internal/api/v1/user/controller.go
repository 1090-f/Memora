package user

import (
	"net/http"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/audit"
	"github.com/gin-gonic/gin"
)

// Controller 处理用户管理相关的 HTTP 请求。
type Controller struct{ users service.UserService }

// NewController 创建一个新的用户控制器实例。
func NewController(users service.UserService) *Controller { return &Controller{users: users} }

// Me 返回当前已认证用户的个人信息。
func (ctrl *Controller) Me(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.users.GetCurrent(c.Request.Context(), user.ID)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// UpdateMe 更新当前已认证用户的个人信息。
func (ctrl *Controller) UpdateMe(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.users.UpdateCurrent(c.Request.Context(), user.ID, &req)
	if err != nil {
		response.Failure(c, err)
		return
	}
	audit.Record("user.profile.update", user.ID, "user:"+user.ID, middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, result)
}

// ChangePassword 修改当前已认证用户的密码。
func (ctrl *Controller) ChangePassword(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	if err := ctrl.users.ChangePassword(c.Request.Context(), user.ID, &req); err != nil {
		response.Failure(c, err)
		return
	}
	audit.Record("user.password.change", user.ID, "user:"+user.ID, middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, gin.H{"password_changed": true})
}
