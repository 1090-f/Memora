package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/metrics"
	"go.uber.org/zap"
)

// RunnerConfig 定义 Worker 运行器的配置参数。
type RunnerConfig struct {
	Concurrency     int
	PollInterval    time.Duration
	DefaultTimeout  time.Duration
	MaxRetryDelay   time.Duration
	IdempotencyTTL  time.Duration
	FinalizeTimeout time.Duration
}

// Runner 是并发消费任务的 Worker 运行器，负责任务调度、幂等性检查和重试。
type Runner struct {
	config      RunnerConfig
	source      Source
	registry    *Registry
	idempotency IdempotencyStore
	mutex       sync.Mutex
	active      map[string]context.CancelFunc
}

// NewRunner 创建一个新的 Worker 运行器实例。
func NewRunner(config RunnerConfig, source Source, registry *Registry, idempotency IdempotencyStore) (*Runner, error) {
	if source == nil || registry == nil || idempotency == nil {
		return nil, fmt.Errorf("worker source, registry and idempotency store are required")
	}
	if config.Concurrency <= 0 || config.PollInterval <= 0 || config.DefaultTimeout <= 0 || config.IdempotencyTTL <= 0 {
		return nil, fmt.Errorf("worker concurrency and durations must be positive")
	}
	if config.MaxRetryDelay <= 0 {
		config.MaxRetryDelay = time.Minute
	}
	if config.FinalizeTimeout <= 0 {
		config.FinalizeTimeout = 5 * time.Second
	}
	return &Runner{config: config, source: source, registry: registry, idempotency: idempotency, active: make(map[string]context.CancelFunc)}, nil
}

// Run 启动指定并发数的 Worker 协程，阻塞等待直到上下文取消。
func (r *Runner) Run(ctx context.Context) error {
	var wait sync.WaitGroup
	for workerID := 1; workerID <= r.config.Concurrency; workerID++ {
		wait.Add(1)
		go func(id int) {
			defer wait.Done()
			r.loop(ctx, id)
		}(workerID)
	}
	<-ctx.Done()
	r.cancelAll()
	wait.Wait()
	return nil
}

// Cancel 取消指定 ID 的正在运行的任务。
func (r *Runner) Cancel(jobID string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	cancel, exists := r.active[jobID]
	if exists {
		cancel()
	}
	return exists
}

func (r *Runner) loop(ctx context.Context, workerID int) {
	for ctx.Err() == nil {
		job, err := r.source.Reserve(ctx)
		if errors.Is(err, ErrNoWork) {
			if !waitFor(ctx, r.config.PollInterval) {
				return
			}
			continue
		}
		if err != nil {
			logger.Error("worker reserve failed", zap.Int("worker_id", workerID), zap.Error(err))
			if !waitFor(ctx, r.config.PollInterval) {
				return
			}
			continue
		}
		if job == nil {
			continue
		}
		r.execute(ctx, *job)
	}
}

func (r *Runner) execute(parent context.Context, job Job) {
	started := time.Now()
	result := "failed"
	defer func() { metrics.WorkerFinished(job.Type, result, time.Since(started)) }()
	if job.Attempt <= 0 {
		job.Attempt = 1
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 3
	}
	if job.Timeout <= 0 {
		job.Timeout = r.config.DefaultTimeout
	}
	key := job.IdempotencyKey
	if key == "" {
		key = job.ID
	}
	claimed, err := r.idempotency.Claim(parent, key, r.config.IdempotencyTTL)
	if err != nil {
		finalizeCtx, finalizeCancel := r.finalizeContext()
		defer finalizeCancel()
		result = retryResult(job)
		r.retryOrFail(finalizeCtx, job, err)
		return
	}
	if !claimed {
		finalizeCtx, finalizeCancel := r.finalizeContext()
		defer finalizeCancel()
		result = "duplicate"
		if err := r.source.Complete(finalizeCtx, job); err != nil {
			logger.Error("worker duplicate completion failed", zap.String("job_id", job.ID), zap.Error(err))
		}
		return
	}
	handler, exists := r.registry.Handler(job.Type)
	if !exists {
		finalizeCtx, finalizeCancel := r.finalizeContext()
		defer finalizeCancel()
		err := fmt.Errorf("worker handler %q is not registered", job.Type)
		_ = r.idempotency.Release(finalizeCtx, key)
		result = retryResult(job)
		r.retryOrFail(finalizeCtx, job, err)
		return
	}
	jobCtx, cancel := context.WithTimeout(parent, job.Timeout)
	r.addActive(job.ID, cancel)
	err = handler.Handle(jobCtx, job)
	cancel()
	r.removeActive(job.ID)
	finalizeCtx, finalizeCancel := r.finalizeContext()
	defer finalizeCancel()
	if err != nil {
		_ = r.idempotency.Release(finalizeCtx, key)
		result = retryResult(job)
		r.retryOrFail(finalizeCtx, job, err)
		return
	}
	if err := r.idempotency.Complete(finalizeCtx, key, r.config.IdempotencyTTL); err != nil {
		_ = r.idempotency.Release(finalizeCtx, key)
		result = retryResult(job)
		r.retryOrFail(finalizeCtx, job, err)
		return
	}
	if err := r.source.Complete(finalizeCtx, job); err != nil {
		logger.Error("worker completion failed", zap.String("job_id", job.ID), zap.Error(err))
	}
	result = "succeeded"
}

func (r *Runner) finalizeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.config.FinalizeTimeout)
}

func retryResult(job Job) string {
	if job.Attempt < job.MaxAttempts {
		return "retry"
	}
	return "failed"
}

func (r *Runner) retryOrFail(ctx context.Context, job Job, cause error) {
	if job.Attempt < job.MaxAttempts {
		delay := time.Second << min(job.Attempt-1, 6)
		if delay > r.config.MaxRetryDelay {
			delay = r.config.MaxRetryDelay
		}
		job.Attempt++
		if err := r.source.Retry(ctx, job, time.Now().UTC().Add(delay), cause); err != nil {
			logger.Error("worker retry scheduling failed", zap.String("job_id", job.ID), zap.Error(err))
		}
		return
	}
	if err := r.source.Fail(ctx, job, cause); err != nil {
		logger.Error("worker failure persistence failed", zap.String("job_id", job.ID), zap.Error(err))
	}
}

func (r *Runner) addActive(jobID string, cancel context.CancelFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.active[jobID] = cancel
}

func (r *Runner) removeActive(jobID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.active, jobID)
}

func (r *Runner) cancelAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, cancel := range r.active {
		cancel()
	}
}

func waitFor(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
