package errors

import (
	"fmt"
	"net/http"
)

// AppError 是应用级错误类型，包含错误码、HTTP状态码、错误消息和原始错误
type AppError struct {
	Code       Code
	Message    string
	HTTPStatus int
	Details    any
	Cause      error
}

// Error 实现error接口，返回格式化的错误信息
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.safeMessage(), e.Cause)
	}
	return e.safeMessage()
}

// Unwrap 返回被包装的原始错误，支持errors.Is和errors.As链式检查
func (e *AppError) Unwrap() error { return e.Cause }

func (e *AppError) safeMessage() string {
	if e.Message != "" {
		return e.Message
	}
	return Message(e.Code)
}

// New 创建一个新的AppError实例
func New(code Code, status int, cause error) *AppError {
	return &AppError{Code: code, HTTPStatus: status, Cause: cause}
}

var (
	ErrInvalidArgument  = New(CodeInvalidArgument, http.StatusBadRequest, nil)
	ErrUnauthorized     = New(CodeUnauthorized, http.StatusUnauthorized, nil)
	ErrForbidden        = New(CodeForbidden, http.StatusForbidden, nil)
	ErrNotFound         = New(CodeNotFound, http.StatusNotFound, nil)
	ErrConflict         = New(CodeConflict, http.StatusConflict, nil)
	ErrPayloadTooLarge  = New(CodePayloadTooLarge, http.StatusRequestEntityTooLarge, nil)
	ErrMCPImportFailed  = New(CodeMCPImportFailed, http.StatusUnprocessableEntity, nil)
	ErrMCPConnFailed    = New(CodeMCPConnFailed, http.StatusBadGateway, nil)
	ErrMCPDiscoveryFail = New(CodeMCPDiscoveryFail, http.StatusBadGateway, nil)
	ErrMCPCallFailed    = New(CodeMCPCallFailed, http.StatusBadGateway, nil)
	ErrInternal         = New(CodeInternal, http.StatusInternalServerError, nil)
)
