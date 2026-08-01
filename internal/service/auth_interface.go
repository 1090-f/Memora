package service

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	jwtmanager "github.com/1090-f/Memora/pkg/jwt"
)

type AuthService interface {
	Login(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error)
	Authenticate(ctx context.Context, token string) (*entity.User, *jwtmanager.Claims, error)
	Logout(ctx context.Context, claims *jwtmanager.Claims) error
}
