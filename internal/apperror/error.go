// Package apperror 定义与传输协议无关的应用错误。
package apperror

import (
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

// AppError 携带稳定错误码、可选响应详情和原始错误。
// HTTP 状态码和对外消息由 API 边界负责映射。
type AppError struct {
	Code    contracts.ErrorCode
	Details any
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Cause)
	}
	return string(e.Code)
}

// Unwrap 支持标准库 errors.Is 和 errors.As。
func (e *AppError) Unwrap() error { return e.Cause }

// New 创建应用错误并保留原始错误链。
func New(code contracts.ErrorCode, cause error) *AppError {
	return &AppError{Code: code, Cause: cause}
}

var (
	ErrInvalidArgument = New(contracts.ErrInvalidArgument, nil)
	ErrUnauthorized    = New(contracts.ErrUnauthorized, nil)
	ErrForbidden       = New(contracts.ErrForbidden, nil)
	ErrNotFound        = New(contracts.ErrResourceNotFound, nil)
	ErrConflict        = New(contracts.ErrDuplicateResource, nil)
	ErrPayloadTooLarge = New(contracts.ErrPayloadTooLarge, nil)
	ErrInternal        = New(contracts.ErrInternal, nil)
)
