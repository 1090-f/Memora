package core

import (
	"errors"

	"github.com/1090-f/Memora/internal/contracts"
)

// errorCode 将内部错误转换为稳定错误码，避免把堆栈和敏感信息发布到事件。
func errorCode(err error) contracts.ErrorCode {
	if err == nil {
		return contracts.CodeOK
	}
	var coreErr *CoreError
	if errors.As(err, &coreErr) && coreErr.Code != "" {
		return coreErr.Code
	}
	switch {
	case errors.Is(err, ErrBudgetExceeded):
		return contracts.ErrRateLimited
	case errors.Is(err, ErrExecutionDependency):
		return contracts.ErrServiceUnavailable
	case errors.Is(err, ErrPersistenceUnavailable):
		return contracts.ErrServiceUnavailable
	default:
		return contracts.ErrInternal
	}
}
