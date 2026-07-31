// Package persistence implements identity ports using GORM and Redis.
package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/modules/identity/domain"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const blacklistKeyPrefix = "auth:blacklist:"

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindActiveByAccount(ctx context.Context, account string) (*domain.User, error) {
	var record userRecord
	err := r.db.WithContext(ctx).
		Where("LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)", account, account).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find active user by account: %w", err)
	}
	return record.toDomain(), nil
}

func (r *UserRepository) FindActiveByID(ctx context.Context, id string) (*domain.User, error) {
	var record userRecord
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find active user by ID: %w", err)
	}
	return record.toDomain(), nil
}

type userRecord struct {
	ID           string    `gorm:"type:uuid;primaryKey"`
	Username     string    `gorm:"not null"`
	Email        string    `gorm:"not null"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (userRecord) TableName() string { return "users" }

func (u userRecord) toDomain() *domain.User {
	return &domain.User{
		ID: u.ID, Username: u.Username, Email: u.Email, PasswordHash: u.PasswordHash,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

type redisCommands interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
}

type TokenBlacklist struct {
	redis redisCommands
}

func NewTokenBlacklist(client redisCommands) *TokenBlacklist {
	return &TokenBlacklist{redis: client}
}

func (b *TokenBlacklist) Revoke(ctx context.Context, tokenID string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	if err := b.redis.Set(ctx, blacklistKeyPrefix+tokenID, "1", ttl).Err(); err != nil {
		return fmt.Errorf("store revoked token: %w", err)
	}
	return nil
}

func (b *TokenBlacklist) IsRevoked(ctx context.Context, tokenID string) (bool, error) {
	count, err := b.redis.Exists(ctx, blacklistKeyPrefix+tokenID).Result()
	if err != nil {
		return false, fmt.Errorf("read revoked token: %w", err)
	}
	return count > 0, nil
}
