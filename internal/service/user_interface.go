package service

import (
	"context"
	"io"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
)

// MaxAvatarFileSize 是头像文件允许的最大字节数（5 MiB）。
const MaxAvatarFileSize int64 = 5 * 1024 * 1024

// AvatarUploadInput 描述一次头像文件上传。
type AvatarUploadInput struct {
	Reader   io.Reader
	Size     int64
	FileName string
}

// AvatarFile 描述可直接输出到 HTTP 响应的头像文件。
type AvatarFile struct {
	Reader      io.ReadCloser
	Size        int64
	ContentType string
}

// UserService 定义用户管理业务逻辑的接口。
type UserService interface {
	// GetCurrent 根据用户 ID 获取当前用户信息。
	GetCurrent(ctx context.Context, id string) (*dto.UserResponse, error)
	// UpdateCurrent 更新当前用户的个人资料。
	UpdateCurrent(ctx context.Context, id string, req *request.UpdateUserRequest) (*dto.UserResponse, error)
	// ChangePassword 修改当前用户的密码。
	ChangePassword(ctx context.Context, id string, req *request.ChangePasswordRequest) error
	// UploadAvatar 上传并更新当前用户的头像。
	UploadAvatar(ctx context.Context, id string, input AvatarUploadInput) (*dto.UserResponse, error)
	// OpenAvatar 打开指定用户的头像读取流。
	OpenAvatar(ctx context.Context, id string) (*AvatarFile, error)
}
