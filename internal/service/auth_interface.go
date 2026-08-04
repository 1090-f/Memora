package service

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	jwtmanager "github.com/1090-f/Memora/pkg/jwt"
)

// AuthService 定义认证业务逻辑的接口。
type AuthService interface {
	// Login 验证用户凭据并返回 JWT Token 和用户信息。
	Login(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error)
	// Authenticate 验证 JWT Token 的有效性并返回用户信息和 Claims。
	Authenticate(ctx context.Context, token string) (*entity.User, *jwtmanager.Claims, error)
	// Logout 吊销指定用户的 JWT Token。
	Logout(ctx context.Context, claims *jwtmanager.Claims) error
}
