package worker

import (
	"context"
	"time"
)

// IdleSource 保持 Worker 进程健康运行，直到业务模块注册自己的持久化任务源。
type IdleSource struct{}

// Reserve 始终返回 ErrNoWork，表示没有可用任务。
func (IdleSource) Reserve(context.Context) (*Job, error) { return nil, ErrNoWork }

// Complete 空操作，不做任何处理。
func (IdleSource) Complete(context.Context, Job) error   { return nil }

// Retry 空操作，不做任何处理。
func (IdleSource) Retry(context.Context, Job, time.Time, error) error {
	return nil
}
// Fail 空操作，不做任何处理。
func (IdleSource) Fail(context.Context, Job, error) error { return nil }
