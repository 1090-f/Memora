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

// 预定义的应用级错误变量，供业务层直接返回。
// 调用方可通过 AppError.Details 附加额外响应信息。
var (
	// ErrInvalidArgument 参数无效（HTTP 400）。
	ErrInvalidArgument = New(contracts.ErrInvalidArgument, nil)
	// ErrUnauthorized 未授权，通常表示缺少或无效的认证凭证（HTTP 401）。
	ErrUnauthorized = New(contracts.ErrUnauthorized, nil)
	// ErrForbidden 无权访问，认证通过但权限不足（HTTP 403）。
	ErrForbidden = New(contracts.ErrForbidden, nil)
	// ErrNotFound 资源不存在（HTTP 404）。
	ErrNotFound = New(contracts.ErrResourceNotFound, nil)
	// ErrConflict 资源冲突，如重复创建（HTTP 409）。
	ErrConflict = New(contracts.ErrDuplicateResource, nil)
	// ErrInternal 服务器内部错误（HTTP 500）。
	ErrInternal = New(contracts.ErrInternal, nil)
)
