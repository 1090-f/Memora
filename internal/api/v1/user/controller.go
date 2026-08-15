package user

import (
	"errors"
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

// UploadAvatar 上传当前已认证用户的头像图片。
func (ctrl *Controller) UploadAvatar(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxAvatarFileSize+1024*1024)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			response.Failure(c, apperrors.ErrPayloadTooLarge)
			return
		}
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	if fileHeader.Size > service.MaxAvatarFileSize {
		response.Failure(c, apperrors.ErrPayloadTooLarge)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	defer file.Close()

	result, err := ctrl.users.UploadAvatar(c.Request.Context(), user.ID, service.AvatarUploadInput{
		Reader: file, Size: fileHeader.Size, FileName: fileHeader.Filename,
	})
	if err != nil {
		response.Failure(c, err)
		return
	}
	audit.Record("user.avatar.update", user.ID, "user:"+user.ID, middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, result)
}

// Avatar 流式返回指定用户的头像，供浏览器图片标签直接加载。
func (ctrl *Controller) Avatar(c *gin.Context) {
	file, err := ctrl.users.OpenAvatar(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Failure(c, err)
		return
	}
	defer file.Reader.Close()
	c.DataFromReader(http.StatusOK, file.Size, file.ContentType, file.Reader, map[string]string{
		"Cache-Control":          "public, max-age=86400",
		"Content-Disposition":    "inline",
		"X-Content-Type-Options": "nosniff",
	})
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
