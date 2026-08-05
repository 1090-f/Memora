package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	// ErrUserNotFound 表示未找到指定用户。
	ErrUserNotFound = errors.New("用户不存在")
	// ErrUserConflict 表示用户字段与已有用户冲突。
	ErrUserConflict = errors.New("用户信息与现有用户冲突")
)

// userRepository 是 UserRepository 接口的 GORM 实现。
type userRepository struct{ db *gorm.DB }

// NewUserRepository 创建一个新的用户仓储实例。
func NewUserRepository(db *gorm.DB) UserRepository { return &userRepository{db: db} }

// FindActiveByAccount 根据账号（用户名或邮箱）查找活跃用户。
func (r *userRepository) FindActiveByAccount(ctx context.Context, account string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).
		Where("(LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)) AND status = ? AND deleted_at IS NULL", account, account, "active").
		First(&user).Error
	return mapUserResult(&user, err)
}

// FindActiveByID 根据用户 ID 查找活跃用户。
func (r *userRepository) FindActiveByID(ctx context.Context, id string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("id = ? AND status = ? AND deleted_at IS NULL", id, "active").First(&user).Error
	return mapUserResult(&user, err)
}

// UpdateLastLogin 更新用户的最后登录时间。
func (r *userRepository) UpdateLastLogin(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Model(&entity.User{}).Where("id = ?", id).Update("last_login_at", time.Now().UTC()).Error; err != nil {
		return fmt.Errorf("更新最近登录时间失败: %w", err)
	}
	return nil
}

// UpdateProfile 更新用户的个人资料，处理唯一约束冲突。
func (r *userRepository) UpdateProfile(ctx context.Context, id string, req *request.UpdateUserRequest) (*entity.User, error) {
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	result := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, "active").Updates(updates)
	if result.Error != nil {
		var postgresError *pgconn.PgError
		if errors.As(result.Error, &postgresError) && postgresError.Code == "23505" {
			return nil, ErrUserConflict
		}
		return nil, fmt.Errorf("更新用户资料失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrUserNotFound
	}
	return r.FindActiveByID(ctx, id)
}

// UpdatePassword 更新用户的密码哈希值。
func (r *userRepository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	result := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, "active").
		Updates(map[string]any{"password_hash": passwordHash, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("更新用户密码失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func mapUserResult(user *entity.User, err error) (*entity.User, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return user, nil
}
