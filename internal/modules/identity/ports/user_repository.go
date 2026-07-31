// Package ports defines the identity module's outbound dependencies.
package ports

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/modules/identity/domain"
)

type UserRepository interface {
	FindActiveByAccount(ctx context.Context, account string) (*domain.User, error)
	FindActiveByID(ctx context.Context, id string) (*domain.User, error)
}

type TokenBlacklist interface {
	Revoke(ctx context.Context, tokenID string, ttl time.Duration) error
	IsRevoked(ctx context.Context, tokenID string) (bool, error)
}
