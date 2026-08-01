package service

import (
	"context"
	"errors"
	"strings"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	apperrors "github.com/1090-f/Memora/pkg/errors"
)

type userService struct{ users repository.UserRepository }

func NewUserService(users repository.UserRepository) UserService { return &userService{users: users} }

func (s *userService) GetCurrent(ctx context.Context, id string) (*dto.UserResponse, error) {
	user, err := s.users.FindActiveByID(ctx, id)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, apperrors.ErrUnauthorized
	}
	if err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, 500, err)
	}
	response := UserResponse(user)
	return &response, nil
}

func (s *userService) UpdateCurrent(ctx context.Context, id string, req *request.UpdateUserRequest) (*dto.UserResponse, error) {
	if req == nil || (req.Nickname == nil && req.AvatarURL == nil && req.Bio == nil && req.Email == nil) {
		return nil, apperrors.ErrInvalidArgument
	}
	trimOptional(&req.Nickname)
	trimOptional(&req.AvatarURL)
	trimOptional(&req.Bio)
	trimOptional(&req.Email)
	if req.Email != nil && *req.Email == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	user, err := s.users.UpdateProfile(ctx, id, req)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, apperrors.ErrUnauthorized
	}
	if errors.Is(err, repository.ErrUserConflict) {
		return nil, apperrors.ErrConflict
	}
	if err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, 500, err)
	}
	response := UserResponse(user)
	return &response, nil
}

func (s *userService) ChangePassword(ctx context.Context, id string, req *request.ChangePasswordRequest) error {
	if req == nil || req.OldPassword == "" || len(req.NewPassword) < 12 || req.OldPassword == req.NewPassword {
		return apperrors.ErrInvalidArgument
	}
	user, err := s.users.FindActiveByID(ctx, id)
	if errors.Is(err, repository.ErrUserNotFound) {
		return apperrors.ErrUnauthorized
	}
	if err != nil {
		return apperrors.New(apperrors.CodeInternal, 500, err)
	}
	if !VerifyPassword(req.OldPassword, user.PasswordHash) {
		return apperrors.ErrUnauthorized
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		return apperrors.New(apperrors.CodeInternal, 500, err)
	}
	if err := s.users.UpdatePassword(ctx, id, hash); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return apperrors.ErrUnauthorized
		}
		return apperrors.New(apperrors.CodeInternal, 500, err)
	}
	return nil
}

func trimOptional(value **string) {
	if *value == nil {
		return
	}
	trimmed := strings.TrimSpace(**value)
	*value = &trimmed
}

func UserResponse(user *entity.User) dto.UserResponse {
	nickname := user.Username
	if user.Nickname != nil && *user.Nickname != "" {
		nickname = *user.Nickname
	}
	return dto.UserResponse{ID: user.ID, Username: user.Username, Nickname: nickname, Email: user.Email, AvatarURL: user.AvatarURL, Bio: user.Bio}
}
