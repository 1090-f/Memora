package service

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"go.uber.org/zap"
)

// userService 是 UserService 接口的实现。
type userService struct {
	users repository.UserRepository
	store ObjectStore
}

// NewUserService 创建一个新的用户服务实例。
func NewUserService(users repository.UserRepository, store ObjectStore) UserService {
	return &userService{users: users, store: store}
}

// GetCurrent 根据用户 ID 获取当前用户信息。
func (s *userService) GetCurrent(ctx context.Context, id string) (*dto.UserResponse, error) {
	user, err := s.users.FindActiveByID(ctx, id)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, apperrors.ErrUnauthorized
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	response := UserResponse(user)
	return &response, nil
}

// UpdateCurrent 更新当前用户的个人资料，包含参数校验和冲突检测。
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
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	response := UserResponse(user)
	logger.Info("用户资料已更新",
		zap.String("user_id", id), zap.String("username", user.Username))
	return &response, nil
}

// ChangePassword 验证旧密码并更新为新密码。
func (s *userService) ChangePassword(ctx context.Context, id string, req *request.ChangePasswordRequest) error {
	if req == nil || req.OldPassword == "" || len(req.NewPassword) < 12 || req.OldPassword == req.NewPassword {
		return apperrors.ErrInvalidArgument
	}
	user, err := s.users.FindActiveByID(ctx, id)
	if errors.Is(err, repository.ErrUserNotFound) {
		return apperrors.ErrUnauthorized
	}
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	if !VerifyPassword(req.OldPassword, user.PasswordHash) {
		return apperrors.ErrUnauthorized
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	if err := s.users.UpdatePassword(ctx, id, hash); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return apperrors.ErrUnauthorized
		}
		return apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("用户密码已修改", zap.String("user_id", id))
	return nil
}

// UploadAvatar 校验图片内容后写入对象存储，并更新用户头像地址。
func (s *userService) UploadAvatar(ctx context.Context, id string, input AvatarUploadInput) (*dto.UserResponse, error) {
	if s.store == nil || input.Reader == nil || input.Size <= 0 {
		return nil, apperrors.ErrInvalidArgument
	}
	if input.Size > MaxAvatarFileSize {
		return nil, apperrors.ErrPayloadTooLarge
	}
	if _, err := s.users.FindActiveByID(ctx, id); errors.Is(err, repository.ErrUserNotFound) {
		return nil, apperrors.ErrUnauthorized
	} else if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}

	reader := bufio.NewReader(input.Reader)
	header, err := reader.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, apperrors.ErrInvalidArgument
	}
	contentType := http.DetectContentType(header)
	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	if !allowed[contentType] {
		return nil, apperrors.ErrInvalidArgument
	}

	objectKey := objectstore.BuildAvatarObjectKey(id)
	if err := s.store.PutObject(ctx, objectKey, reader, input.Size, contentType); err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}

	avatarURL := "/api/v1/users/" + id + "/avatar?v=" + time.Now().UTC().Format("20060102150405.000000000")
	user, err := s.users.UpdateProfile(ctx, id, &request.UpdateUserRequest{AvatarURL: &avatarURL})
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, apperrors.ErrUnauthorized
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	response := UserResponse(user)
	logger.Info("用户头像已更新", zap.String("user_id", id), zap.String("file_name", input.FileName))
	return &response, nil
}

// OpenAvatar 打开指定用户的头像文件，供浏览器直接展示。
func (s *userService) OpenAvatar(ctx context.Context, id string) (*AvatarFile, error) {
	if s.store == nil || strings.TrimSpace(id) == "" {
		return nil, apperrors.ErrNotFound
	}
	user, err := s.users.FindActiveByID(ctx, id)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if user.AvatarURL == nil || strings.TrimSpace(*user.AvatarURL) == "" {
		return nil, apperrors.ErrNotFound
	}

	objectKey := objectstore.BuildAvatarObjectKey(id)
	info, err := s.store.StatObject(ctx, objectKey)
	if errors.Is(err, objectstore.ErrObjectNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	reader, err := s.store.OpenObject(ctx, objectKey)
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return &AvatarFile{Reader: reader, Size: info.Size, ContentType: info.ContentType}, nil
}

// trimOptional 去除可选字符串指针值的首尾空白。
func trimOptional(value **string) {
	if *value == nil {
		return
	}
	trimmed := strings.TrimSpace(**value)
	*value = &trimmed
}

// UserResponse 将用户实体转换为用户响应 DTO。
func UserResponse(user *entity.User) dto.UserResponse {
	nickname := user.Username
	if user.Nickname != nil && *user.Nickname != "" {
		nickname = *user.Nickname
	}
	return dto.UserResponse{ID: user.ID, Username: user.Username, Nickname: nickname, Email: user.Email, AvatarURL: user.AvatarURL, Bio: user.Bio}
}
