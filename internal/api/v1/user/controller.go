package user

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

type Controller struct{ users service.UserService }

func NewController(users service.UserService) *Controller { return &Controller{users: users} }

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
