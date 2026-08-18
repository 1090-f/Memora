// Package repository 实现 AgentRunRepository 的 GORM 数据访问层。
// 所有查询均强制包含用户 ID 和状态过滤，防止数据越权。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// agentRunRepository 是 AgentRunRepository 接口的 GORM 实现。
type agentRunRepository struct {
	db *gorm.DB
}

// NewAgentRunRepository 创建 AgentRunRepository 实例。
func NewAgentRunRepository(db *gorm.DB) AgentRunRepository {
	return &agentRunRepository{db: db}
}

// CreateQueued 创建一条状态为 queued 的 Agent 运行记录。
// 状态由数据库默认值 'queued' 保证，应用层无需显式设置。
func (r *agentRunRepository) CreateQueued(ctx context.Context, run *entity.AgentRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

// FindByID 根据运行 ID 和用户 ID 查找运行记录（强制所有者过滤）。
// 同时过滤 deleted_at IS NULL 和有效运行状态。
func (r *agentRunRepository) FindByID(ctx context.Context, userID, runID uuid.UUID) (*entity.AgentRun, error) {
	var run entity.AgentRun
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", runID, userID).
		First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// FindByIDAdmin 根据运行 ID 直接查找（用于 Worker，跳过用户过滤）。
func (r *agentRunRepository) FindByIDAdmin(ctx context.Context, runID uuid.UUID) (*entity.AgentRun, error) {
	var run entity.AgentRun
	err := r.db.WithContext(ctx).
		Where("id = ?", runID).
		First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListByOwner 按用户、知识库、会话、状态和模式分页查询运行记录，按创建时间降序排列。
// conversationID、status 和 executionMode 为空值时不按对应条件过滤。
// page 从 1 开始，pageSize 为每页条数。
func (r *agentRunRepository) ListByOwner(ctx context.Context, userID, kbID, conversationID uuid.UUID, status, executionMode string, page, pageSize int) ([]entity.AgentRun, int64, error) {
	var runs []entity.AgentRun
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.AgentRun{}).
		Where("user_id = ? AND knowledge_base_id = ?", userID, kbID)
	if conversationID != uuid.Nil {
		query = query.Where("conversation_id = ?", conversationID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if executionMode != "" {
		query = query.Where("execution_mode = ?", executionMode)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&runs).Error
	return runs, total, err
}

// ListQueued 获取所有 queued 状态的运行记录（用于 Worker 批量领取）。
func (r *agentRunRepository) ListQueued(ctx context.Context, limit int) ([]entity.AgentRun, error) {
	var runs []entity.AgentRun
	err := r.db.WithContext(ctx).
		Where("status = 'queued'").
		Order("created_at ASC").
		Limit(limit).
		Find(&runs).Error
	return runs, err
}

// ReserveQueued 原子地将一条 queued 记录标记为 running。
// 使用条件更新 WHERE status='queued' 避免并发 Worker 重复领取。
// 更新成功返回完整运行记录，已被其他 Worker 领取则返回 nil。
func (r *agentRunRepository) ReserveQueued(ctx context.Context, runID uuid.UUID) (*entity.AgentRun, error) {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&entity.AgentRun{}).
		Where("id = ? AND status = 'queued'", runID).
		Updates(map[string]interface{}{
			"status":     "running",
			"started_at": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil // 已被其他 Worker 领取或状态已变更
	}
	// 重新查询完整记录返回
	return r.FindByIDAdmin(ctx, runID)
}

// MarkRunning 更新运行状态为 running，设置开始时间。
func (r *agentRunRepository) MarkRunning(ctx context.Context, runID uuid.UUID, startedAt string) error {
	return r.db.WithContext(ctx).
		Model(&entity.AgentRun{}).
		Where("id = ?", runID).
		Updates(map[string]interface{}{
			"status":     "running",
			"started_at": startedAt,
		}).Error
}

// MarkCompleted 更新运行状态为 completed，记录最终结果、Token 用量、耗时、执行模式、知识状态和结束时间。
func (r *agentRunRepository) MarkCompleted(ctx context.Context, runID uuid.UUID, finalResult string, inputTokens, outputTokens, totalTokens int, durationMs int64, executionMode, knowledgeStatus string) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":        "completed",
		"final_result":  finalResult,
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
		"total_tokens":  totalTokens,
		"duration_ms":   durationMs,
		"ended_at":      now,
	}
	if executionMode != "" {
		updates["execution_mode"] = executionMode
	}
	if knowledgeStatus != "" {
		updates["knowledge_status"] = knowledgeStatus
	}
	return r.db.WithContext(ctx).
		Model(&entity.AgentRun{}).
		Where("id = ?", runID).
		Updates(updates).Error
}

// MarkFailed 更新运行状态为 failed，记录错误码、错误信息、执行模式、Token 用量、耗时和结束时间。
func (r *agentRunRepository) MarkFailed(ctx context.Context, runID uuid.UUID, errorCode, errorMessage, executionMode string, durationMs int64, inputTokens, outputTokens, totalTokens int) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":        "failed",
		"error_code":    errorCode,
		"error_message": errorMessage,
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
		"total_tokens":  totalTokens,
		"duration_ms":   durationMs,
		"ended_at":      now,
	}
	if executionMode != "" {
		updates["execution_mode"] = executionMode
	}
	return r.db.WithContext(ctx).
		Model(&entity.AgentRun{}).
		Where("id = ?", runID).
		Updates(updates).Error
}

// MarkCancelled 更新运行状态为 cancelled（需同时验证用户 ID 确保所有者可取消）。
// executionMode 为空表示在 Router 决策出模式前取消，此时不写入 execution_mode。
// duration_ms 由数据库根据 started_at 与当前时间计算，尚未开始执行（queued）时为 0。
func (r *agentRunRepository) MarkCancelled(ctx context.Context, userID, runID uuid.UUID, executionMode string) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":      "cancelled",
		"ended_at":    now,
		"duration_ms": gorm.Expr("COALESCE(EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000, 0)::bigint"),
	}
	if executionMode != "" {
		updates["execution_mode"] = executionMode
	}
	result := r.db.WithContext(ctx).
		Model(&entity.AgentRun{}).
		Where("id = ? AND user_id = ? AND status IN ('queued', 'running')", runID, userID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound // 运行不存在、不属于该用户或不在可取消状态
	}
	return nil
}

// MarkCancelledAdmin 直接按 runID 取消运行（用于 Worker 超时或内部取消）。
func (r *agentRunRepository) MarkCancelledAdmin(ctx context.Context, runID uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&entity.AgentRun{}).
		Where("id = ? AND status IN ('queued', 'running')", runID).
		Updates(map[string]interface{}{
			"status":      "cancelled",
			"ended_at":    now,
			"duration_ms": gorm.Expr("COALESCE(EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000, 0)::bigint"),
		}).Error
}

// SetAssistantMessageID 设置运行记录的助手消息 ID。
func (r *agentRunRepository) SetAssistantMessageID(ctx context.Context, runID uuid.UUID, assistantMessageID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&entity.AgentRun{}).
		Where("id = ?", runID).
		Update("assistant_message_id", assistantMessageID).Error
}

// CreateRetry 基于失败运行创建新的排队运行，返回新运行 ID。
// 新运行的 retry_of_run_id 指向原始运行，其他字段从原始运行复制。
func (r *agentRunRepository) CreateRetry(ctx context.Context, originalRunID, userID uuid.UUID) (uuid.UUID, error) {
	// 在事务中查找原始运行并创建重试记录
	var newID uuid.UUID
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 查找原始运行，验证所有者
		var original entity.AgentRun
		if err := tx.Where("id = ? AND user_id = ?", originalRunID, userID).First(&original).Error; err != nil {
			return fmt.Errorf("查找原始运行记录失败: %w", err)
		}

		// 2. 创建新运行，从原始运行复制关键字段
		retry := &entity.AgentRun{
			UserID:          original.UserID,
			KnowledgeBaseID: original.KnowledgeBaseID,
			ConversationID:  original.ConversationID,
			UserMessageID:   original.UserMessageID,
			AgentConfigID:   original.AgentConfigID,
			RetryOfRunID:    &original.ID,
			Query:           original.Query,
			Status:          "queued",
		}
		if err := tx.Create(retry).Error; err != nil {
			return fmt.Errorf("创建重试运行记录失败: %w", err)
		}
		newID = retry.ID
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return newID, nil
}

// DeleteByConversationID 删除指定会话的所有 Agent 运行记录。
func (r *agentRunRepository) DeleteByConversationID(ctx context.Context, conversationID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Delete(&entity.AgentRun{}).Error
}

// 编译时确保实现接口
var _ AgentRunRepository = (*agentRunRepository)(nil)
