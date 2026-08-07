package document

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	workerengine "github.com/1090-f/Memora/internal/worker"
)

// taskTimeout 是单个导入任务的处理超时。
const taskTimeout = 30 * time.Minute

// Processor 是 Handler 依赖的最小编排接口。
type Processor interface {
	// ProcessImportTask 编排一次导入任务（创建文档行与状态机流转）。
	ProcessImportTask(ctx context.Context, taskID contracts.ID) error
	// RecoverStaleTasks 恢复卡在 running 且超过租约的任务，返回恢复数量。
	RecoverStaleTasks(ctx context.Context) (int64, error)
}

// Handler 处理文档导入任务，只依赖 Processor 编排。
type Handler struct {
	processor Processor
}

// NewHandler 创建一个文档导入任务处理器。
func NewHandler(processor Processor) *Handler {
	return &Handler{processor: processor}
}

// Handle 执行导入任务，响应 context 取消。
func (h *Handler) Handle(ctx context.Context, job workerengine.Job) error {
	payload, err := parsePayload(job)
	if err != nil {
		return err
	}
	// 任务开始前先探测 context：已取消（如 Worker 停机）则立即返回，不做无谓编排。
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return h.processor.ProcessImportTask(ctx, contracts.ID(payload.TaskID))
}

// RecoverStale 恢复过期任务，供 Worker 启动时调用。
func (h *Handler) RecoverStale(ctx context.Context) (int64, error) {
	return h.processor.RecoverStaleTasks(ctx)
}
