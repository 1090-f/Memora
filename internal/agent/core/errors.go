package core

import (
	"errors"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

// ErrExecutionDependency 表示模型或工具执行依赖未正确注入。
var ErrExecutionDependency = errors.New("agent execution dependency unavailable")

// ErrPersistenceUnavailable 表示运行持久化依赖不可用。
var ErrPersistenceUnavailable = errors.New("agent persistence unavailable")

// ErrBudgetExceeded 表示 Agent 达到配置的执行预算。
var ErrBudgetExceeded = errors.New("agent execution budget exceeded")

// CoreError 携带稳定错误码，供上层映射为统一响应。
type CoreError struct {
	Code contracts.ErrorCode
	Err  error
}

func (e *CoreError) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Err) }
func (e *CoreError) Unwrap() error { return e.Err }

func newCoreError(code contracts.ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	return &CoreError{Code: code, Err: err}
}
