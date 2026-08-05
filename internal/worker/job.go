package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNoWork 表示没有可用的任务。
var ErrNoWork = errors.New("没有可处理的任务")

// Job 表示一个待执行的工作任务。
type Job struct {
	ID             string
	Type           string
	Payload        json.RawMessage
	Attempt        int
	MaxAttempts    int
	Timeout        time.Duration
	IdempotencyKey string
}

// Handler 定义任务处理器的接口。
type Handler interface {
	// Handle 执行指定的任务。
	Handle(ctx context.Context, job Job) error
}

// HandlerFunc 是一个函数类型，实现 Handler 接口。
type HandlerFunc func(context.Context, Job) error

// Handle 调用函数处理任务。
func (fn HandlerFunc) Handle(ctx context.Context, job Job) error { return fn(ctx, job) }

// Source 定义任务源的接口，负责任务的生命周期管理。
type Source interface {
	// Reserve 预留一个可执行的任务。
	Reserve(ctx context.Context) (*Job, error)
	// Complete 将任务标记为已完成。
	Complete(ctx context.Context, job Job) error
	// Retry 将任务标记为待重试，指定下次可用时间。
	Retry(ctx context.Context, job Job, availableAt time.Time, cause error) error
	// Fail 将任务标记为失败。
	Fail(ctx context.Context, job Job, cause error) error
}

// IdempotencyStore 定义幂等性存储的接口，防止任务重复执行。
type IdempotencyStore interface {
	// Claim 尝试获取指定键的幂等性锁。
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Complete 将指定键标记为已完成。
	Complete(ctx context.Context, key string, ttl time.Duration) error
	// Release 释放指定键的幂等性锁。
	Release(ctx context.Context, key string) error
}
