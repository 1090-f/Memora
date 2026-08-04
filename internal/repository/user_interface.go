package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/model/entity"
)

// UserRepository 定义用户数据访问接口，提供用户查询和更新操作。
type UserRepository interface {
	// FindActiveByAccount 根据账号（用户名或邮箱）查找活跃用户。
	FindActiveByAccount(ctx context.Context, account string) (*entity.User, error)
	// FindActiveByID 根据用户 ID 查找活跃用户。
	FindActiveByID(ctx context.Context, id string) (*entity.User, error)
	// UpdateLastLogin 更新用户的最后登录时间。
	UpdateLastLogin(ctx context.Context, id string) error
	// UpdateProfile 更新用户的个人资料。
	UpdateProfile(ctx context.Context, id string, req *request.UpdateUserRequest) (*entity.User, error)
	// UpdatePassword 更新用户的密码哈希。
	UpdatePassword(ctx context.Context, id, passwordHash string) error
}
