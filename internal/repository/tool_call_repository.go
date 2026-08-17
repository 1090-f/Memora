// Package repository 实现 ToolCallRepository 的 GORM 数据访问层。
// 工具调用记录由 ToolExecutor 写入，用于执行轨迹展示和审计追溯。
package repository

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// toolCallRepository 是 ToolCallRepository 接口的 GORM 实现。
type toolCallRepository struct {
	db *gorm.DB
}

// NewToolCallRepository 创建 ToolCallRepository 实例。
func NewToolCallRepository(db *gorm.DB) ToolCallRepository {
	return &toolCallRepository{db: db}
}

// Create 创建一条工具调用记录，起始状态默认为 running。
func (r *toolCallRepository) Create(ctx context.Context, call *entity.ToolCall) error {
	return r.db.WithContext(ctx).Create(call).Error
}

// UpdateResult 更新工具调用的执行结果。
// 更新内容包括状态、输出摘要、错误信息、耗时、截断标志和结束时间。
func (r *toolCallRepository) UpdateResult(ctx context.Context, callID uuid.UUID, status string, outputSummary string, errorCode, errorMessage string, durationMs int64, truncated bool) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&entity.ToolCall{}).
		Where("id = ?", callID).
		Updates(map[string]interface{}{
			"status":         status,
			"output_summary": outputSummary,
			"error_code":     errorCode,
			"error_message":  errorMessage,
			"duration_ms":    durationMs,
			"is_truncated":   truncated,
			"ended_at":       now,
		}).Error
}

// ListByRunID 按 Agent 运行 ID 查询该运行的所有工具调用，按开始时间升序排列。
func (r *toolCallRepository) ListByRunID(ctx context.Context, runID uuid.UUID) ([]entity.ToolCall, error) {
	var calls []entity.ToolCall
	err := r.db.WithContext(ctx).
		Where("agent_run_id = ?", runID).
		Order("started_at ASC").
		Find(&calls).Error
	return calls, err
}

// CountByRunID 统计指定运行的工具调用总数。
func (r *toolCallRepository) CountByRunID(ctx context.Context, runID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.ToolCall{}).
		Where("agent_run_id = ?", runID).
		Count(&count).Error
	return count, err
}

// 编译时确保实现接口
var _ ToolCallRepository = (*toolCallRepository)(nil)
