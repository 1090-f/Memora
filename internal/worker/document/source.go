// Package document 承载文档处理任务的 Source 与 Handler。
// Source 只依赖 ImportTask Repository；Handler 只依赖 DocumentProcessService。
// 不在此包拼接 SQL、构建 Gin 响应或创建 Eino Graph。
package document

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/repository"
	workerengine "github.com/1090-f/Memora/internal/worker"
)

// JobType 是文档导入任务的 Worker 任务类型。
const JobType = "document.import"

// taskPayload 是 Worker Job 的负载，携带任务 ID 与处理版本（幂等键组成）。
type taskPayload struct {
	TaskID  string `json:"task_id"`
	Version int64  `json:"version"`
}

// Source 从 import_tasks 表领取任务，并负责任务生命周期回写。
type Source struct {
	tasks repository.ImportTaskRepository
}

// NewSource 创建一个文档导入任务源。
func NewSource(tasks repository.ImportTaskRepository) *Source {
	return &Source{tasks: tasks}
}

// Reserve 使用 PostgreSQL 行锁领取一个 pending 任务并置为 running。
func (s *Source) Reserve(ctx context.Context) (*workerengine.Job, error) {
	// ReservePending 内部使用 FOR UPDATE SKIP LOCKED 领取：
	// 行锁保证并发 Worker 不会重复领到同一任务，领取成功即置为 running 并记录租约时间。
	task, err := s.tasks.ReservePending(ctx)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, workerengine.ErrNoWork
	}
	payload, err := json.Marshal(taskPayload{TaskID: task.ID, Version: task.StartedAt.UnixMilli()})
	if err != nil {
		return nil, fmt.Errorf("序列化任务负载失败: %w", err)
	}
	// 幂等键包含 started_at 毫秒时间戳：任务被重新领取后时间戳变化，
	// 使 Runner 的幂等检查不会把“重新领取的重跑”误判为重复任务。
	return &workerengine.Job{
		ID:             task.ID,
		Payload:        payload,
		MaxAttempts:    1,
		IdempotencyKey: fmt.Sprintf("%s:%s:%d", JobType, task.ID, task.StartedAt.UnixMilli()),
		Timeout:        taskTimeout,
	}, nil
}

// Complete 将任务标记为 succeeded。
func (s *Source) Complete(ctx context.Context, job workerengine.Job) error {
	payload, err := parsePayload(job)
	if err != nil {
		return err
	}
	return s.tasks.CompleteSucceeded(ctx, payload.TaskID, nil)
}

// Retry 将任务重置为 pending（重试窗口）。任务包 03 阶段重试统一回 pending。
func (s *Source) Retry(ctx context.Context, job workerengine.Job, _ time.Time, cause error) error {
	payload, err := parsePayload(job)
	if err != nil {
		return err
	}
	return s.tasks.FailTask(ctx, payload.TaskID, safeCause(cause))
}

// Fail 将任务标记为 failed 并记录失败原因。
func (s *Source) Fail(ctx context.Context, job workerengine.Job, cause error) error {
	payload, err := parsePayload(job)
	if err != nil {
		return err
	}
	return s.tasks.FailTask(ctx, payload.TaskID, safeCause(cause))
}

// parsePayload 从 Job 负载中解析任务 ID 与处理版本。
func parsePayload(job workerengine.Job) (taskPayload, error) {
	var payload taskPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return taskPayload{}, fmt.Errorf("解析任务负载失败: %w", err)
	}
	return payload, nil
}

// safeCause 将错误转为字符串持久化，nil 时返回空串，避免失败原因字段写入非法值。
func safeCause(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
