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
	ErrUserNotFound      = errors.New("user not found")
	ErrUserConflict      = errors.New("user field conflicts with an existing user")
	ErrDuplicateResource = errors.New("duplicate resource")
)

type userRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &userRepository{db: db} }

func (r *userRepository) FindActiveByAccount(ctx context.Context, account string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).
		Where("(LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)) AND status = ? AND deleted_at IS NULL", account, account, "active").
		First(&user).Error
	return mapUserResult(&user, err)
}

func (r *userRepository) FindActiveByID(ctx context.Context, id string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("id = ? AND status = ? AND deleted_at IS NULL", id, "active").First(&user).Error
	return mapUserResult(&user, err)
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Model(&entity.User{}).Where("id = ?", id).Update("last_login_at", time.Now().UTC()).Error; err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

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
		return nil, fmt.Errorf("update user profile: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrUserNotFound
	}
	return r.FindActiveByID(ctx, id)
}

func (r *userRepository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	result := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, "active").
		Updates(map[string]any{"password_hash": passwordHash, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("update user password: %w", result.Error)
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
		return nil, fmt.Errorf("query user: %w", err)
	}
	return user, nil
}
