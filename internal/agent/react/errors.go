package react

import (
	"errors"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

var (
	// ErrDependencyUnavailable 表示 ReAct 所需依赖未完整注入。
	ErrDependencyUnavailable = errors.New("react dependency unavailable")
	// ErrBudgetExceeded 表示执行预算已经耗尽。
	ErrBudgetExceeded = errors.New("react execution budget exceeded")
	// ErrInvalidResponse 表示模型返回了无法处理的响应。
	ErrInvalidResponse = errors.New("invalid model response")
)

type agentError struct {
	code contracts.ErrorCode
	err  error
}

func (e *agentError) Error() string { return fmt.Sprintf("%s: %v", e.code, e.err) }
func (e *agentError) Unwrap() error { return e.err }

func wrap(code contracts.ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	return &agentError{code: code, err: err}
}
