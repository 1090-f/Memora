package service

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
)

type UserService interface {
	GetCurrent(ctx context.Context, id string) (*dto.UserResponse, error)
	UpdateCurrent(ctx context.Context, id string, req *request.UpdateUserRequest) (*dto.UserResponse, error)
	ChangePassword(ctx context.Context, id string, req *request.ChangePasswordRequest) error
}
