// Package repository 定义数据访问接口与实现。ToolCallRepository 用于工具调用记录的持久化。
package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/google/uuid"
)

// ToolCallRepository 定义工具调用记录的持久化接口。
// 工具调用记录由 ToolExecutor 统一写入，用于审计和可观测性展示。
type ToolCallRepository interface {
	// Create 创建一条工具调用记录。
	Create(ctx context.Context, call *entity.ToolCall) error

	// UpdateResult 更新工具调用的执行结果（输出摘要、状态、耗时、结束时间等）。
	UpdateResult(ctx context.Context, callID uuid.UUID, status string, outputSummary string, errorCode, errorMessage string, durationMs int64, truncated bool) error

	// ListByRunID 按 Agent 运行 ID 查询该运行的所有工具调用，按开始时间升序排列。
	ListByRunID(ctx context.Context, runID uuid.UUID) ([]entity.ToolCall, error)

	// ListByPlanStepID 按计划步骤 ID 查询该步骤的所有工具调用。
	ListByPlanStepID(ctx context.Context, stepID uuid.UUID) ([]entity.ToolCall, error)

	// CountByRunID 统计指定运行的工具调用总数。
	CountByRunID(ctx context.Context, runID uuid.UUID) (int64, error)
}
