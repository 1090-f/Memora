package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	apperrors "github.com/1090-f/Memora/pkg/errors"
	jwtmanager "github.com/1090-f/Memora/pkg/jwt"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/argon2"
)

const (
	argonMemory     = 64 * 1024
	argonTime       = 3
	argonThreads    = 2
	argonSaltLength = 16
	argonKeyLength  = 32
	blacklistPrefix = "auth:blacklist:"
)

type authService struct {
	users     repository.UserRepository
	redis     *redis.Client
	tokens    *jwtmanager.Manager
	dummyHash string
}

func NewAuthService(users repository.UserRepository, redisClient *redis.Client, tokens *jwtmanager.Manager) (AuthService, error) {
	if users == nil || redisClient == nil || tokens == nil {
		return nil, errors.New("authentication dependencies are required")
	}
	dummy, err := HashPassword("memora-invalid-credential-placeholder")
	if err != nil {
		return nil, err
	}
	return &authService{users: users, redis: redisClient, tokens: tokens, dummyHash: dummy}, nil
}

func (s *authService) Login(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.users.FindActiveByAccount(ctx, strings.TrimSpace(req.Account))
	if err != nil {
		_ = VerifyPassword(req.Password, s.dummyHash)
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, apperrors.ErrUnauthorized
		}
		return nil, apperrors.New(apperrors.CodeInternal, 500, err)
	}
	if !VerifyPassword(req.Password, user.PasswordHash) {
		return nil, apperrors.ErrUnauthorized
	}
	if err := s.users.UpdateLastLogin(ctx, user.ID); err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, 500, err)
	}
	tokenID, err := randomTokenID()
	if err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, 500, err)
	}
	token, expiresIn, err := s.tokens.Generate(user.ID, user.Username, tokenID, time.Now().UTC())
	if err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, 500, err)
	}
	return &dto.LoginResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: expiresIn, User: UserResponse(user)}, nil
}

func (s *authService) Authenticate(ctx context.Context, token string) (*entity.User, *jwtmanager.Claims, error) {
	claims, err := s.tokens.Parse(token)
	if err != nil {
		return nil, nil, apperrors.ErrUnauthorized
	}
	revoked, err := s.redis.Exists(ctx, blacklistPrefix+claims.ID).Result()
	if err != nil {
		return nil, nil, apperrors.New(apperrors.CodeInternal, 500, err)
	}
	if revoked > 0 {
		return nil, nil, apperrors.ErrUnauthorized
	}
	user, err := s.users.FindActiveByID(ctx, claims.Subject)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, nil, apperrors.ErrUnauthorized
		}
		return nil, nil, apperrors.New(apperrors.CodeInternal, 500, err)
	}
	return user, claims, nil
}

func (s *authService) Logout(ctx context.Context, claims *jwtmanager.Claims) error {
	if claims == nil || claims.ExpiresAt == nil {
		return apperrors.ErrUnauthorized
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil
	}
	if err := s.redis.Set(ctx, blacklistPrefix+claims.ID, "1", ttl).Err(); err != nil {
		return apperrors.New(apperrors.CodeInternal, 500, err)
	}
	return nil
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil || memory == 0 || iterations == 0 || threads == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func randomTokenID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
