package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/model/entity"
)

type UserRepository interface {
	FindActiveByAccount(ctx context.Context, account string) (*entity.User, error)
	FindActiveByID(ctx context.Context, id string) (*entity.User, error)
	UpdateLastLogin(ctx context.Context, id string) error
	UpdateProfile(ctx context.Context, id string, req *request.UpdateUserRequest) (*entity.User, error)
	UpdatePassword(ctx context.Context, id, passwordHash string) error
}
