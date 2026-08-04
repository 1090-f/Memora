package service

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
)

// UserService 定义用户管理业务逻辑的接口。
type UserService interface {
	// GetCurrent 根据用户 ID 获取当前用户信息。
	GetCurrent(ctx context.Context, id string) (*dto.UserResponse, error)
	// UpdateCurrent 更新当前用户的个人资料。
	UpdateCurrent(ctx context.Context, id string, req *request.UpdateUserRequest) (*dto.UserResponse, error)
	// ChangePassword 修改当前用户的密码。
	ChangePassword(ctx context.Context, id string, req *request.ChangePasswordRequest) error
}
