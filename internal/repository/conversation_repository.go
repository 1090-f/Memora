package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrConversationNotFound 表示未找到指定的会话。
var ErrConversationNotFound = errors.New("conversation not found")

// conversationRepository 是 ConversationRepository 接口的 GORM 实现。
type conversationRepository struct {
	db           *gorm.DB
	agentRunRepo AgentRunRepository
	messageRepo  MessageRepository
}

// NewConversationRepository 创建一个新的会话仓储实例。
func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepository{
		db:           db,
		agentRunRepo: NewAgentRunRepository(db),
		messageRepo:  NewMessageRepository(db),
	}
}

// Create 创建新的会话。
func (r *conversationRepository) Create(ctx context.Context, conversation *entity.Conversation) error {
	if err := r.db.WithContext(ctx).Create(conversation).Error; err != nil {
		return fmt.Errorf("create conversation: %w", err)
	}
	return nil
}

// FindByID 根据 ID 和用户 ID 查找会话。
func (r *conversationRepository) FindByID(ctx context.Context, id, userID string) (*entity.Conversation, error) {
	var conversation entity.Conversation
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query conversation: %w", err)
	}
	return &conversation, nil
}

// FindByIDWithoutUser 根据 ID 查找会话（不校验用户）。
func (r *conversationRepository) FindByIDWithoutUser(ctx context.Context, id string) (*entity.Conversation, error) {
	var conversation entity.Conversation
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query conversation: %w", err)
	}
	return &conversation, nil
}

// ListByKnowledgeBase 列出知识库的会话列表。
func (r *conversationRepository) ListByKnowledgeBase(ctx context.Context, userID, kbID string, page, pageSize int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.WithContext(ctx).
		Where("user_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL", userID, kbID)

	if err := query.Model(&entity.Conversation{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count conversations: %w", err)
	}

	offset := (page - 1) * pageSize
	if err := query.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&conversations).Error; err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}

	return conversations, total, nil
}

// Update 更新会话。
func (r *conversationRepository) Update(ctx context.Context, conversation *entity.Conversation) error {
	conversation.UpdatedAt = time.Now().UTC()
	if err := r.db.WithContext(ctx).Save(conversation).Error; err != nil {
		return fmt.Errorf("update conversation: %w", err)
	}
	return nil
}

// Delete 软删除会话，同时删除关联的所有数据。
// 删除顺序严格按照外键依赖关系：先删子表数据，再删父表数据。
func (r *conversationRepository) Delete(ctx context.Context, id, userID string) error {
	now := time.Now().UTC()

	// 使用事务确保级联删除的原子性
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 验证会话是否存在且属于该用户
		var conversation entity.Conversation
		err := tx.Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
			First(&conversation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConversationNotFound
		}
		if err != nil {
			return fmt.Errorf("查询会话失败: %w", err)
		}

		conversationUUID, err := uuid.Parse(id)
		if err != nil {
			return fmt.Errorf("解析会话ID失败: %w", err)
		}

		// ============================================================
		// 2. 获取该会话下所有 agent_run_id（预查 ID 避免嵌套子查询问题）
		// ============================================================
		var runIDs []uuid.UUID
		if err := tx.Model(&entity.AgentRun{}).
			Where("conversation_id = ?", conversationUUID).
			Pluck("id", &runIDs).Error; err != nil {
			return fmt.Errorf("查询运行记录失败: %w", err)
		}

		if len(runIDs) > 0 {
			// ============================================================
			// 3. 获取这些 agent_run 下所有 agent_plan_id
			// ============================================================
			var planIDs []uuid.UUID
			if err := tx.Model(&entity.AgentPlan{}).
				Where("agent_run_id IN ?", runIDs).
				Pluck("id", &planIDs).Error; err != nil {
				return fmt.Errorf("查询计划记录失败: %w", err)
			}

			if len(planIDs) > 0 {
				// 4. 删除 agent_plan_execution_logs（FK → agent_plans, 有 ON DELETE CASCADE）
				if err := tx.Where("plan_id IN ?", planIDs).
					Delete(&entity.AgentPlanExecutionLog{}).Error; err != nil {
					return fmt.Errorf("删除计划执行日志失败: %w", err)
				}

				// 5. 删除 agent_plan_steps（FK → agent_plans, 无 CASCADE）
				if err := tx.Where("plan_id IN ?", planIDs).
					Delete(&entity.AgentPlanStep{}).Error; err != nil {
					return fmt.Errorf("删除计划步骤失败: %w", err)
				}

				// 6. 删除 agent_plans（FK → agent_runs, 无 CASCADE）
				if err := tx.Where("id IN ?", planIDs).
					Delete(&entity.AgentPlan{}).Error; err != nil {
					return fmt.Errorf("删除关联的计划失败: %w", err)
				}
			}

			// 7. 删除 tool_calls（FK → agent_runs, 无 CASCADE）
			if err := tx.Where("agent_run_id IN ?", runIDs).
				Delete(&entity.ToolCall{}).Error; err != nil {
				return fmt.Errorf("删除工具调用记录失败: %w", err)
			}

			// 8. 删除 agent_events（FK → agent_runs, ON DELETE CASCADE）
			if err := tx.Where("run_id IN ?", runIDs).
				Delete(&entity.AgentEvent{}).Error; err != nil {
				return fmt.Errorf("删除运行事件失败: %w", err)
			}

			// 9. 解除 memories 对 agent_runs 的引用（source_agent_run_id 可以为 NULL）
			if err := tx.Model(&entity.Memory{}).
				Where("source_agent_run_id IN ?", runIDs).
				Update("source_agent_run_id", nil).Error; err != nil {
				return fmt.Errorf("解除记忆的agent运行引用失败: %w", err)
			}
		}

		// 10. 解除 messages 对 agent_runs 的引用（agent_run_id 可以为 NULL）
		if err := tx.Model(&entity.Message{}).
			Where("conversation_id = ?", id).
			Update("agent_run_id", nil).Error; err != nil {
			return fmt.Errorf("解除消息的agent运行引用失败: %w", err)
		}

		// 11. 删除 agent_runs（此时所有引用它的表都已被清理）
		if err := tx.Where("conversation_id = ?", conversationUUID).
			Delete(&entity.AgentRun{}).Error; err != nil {
			return fmt.Errorf("删除关联的agent运行记录失败: %w", err)
		}

		// 12. 删除 messages（此时 agent_runs 已被删除，无引用约束）
		if err := tx.Where("conversation_id = ?", id).
			Delete(&entity.Message{}).Error; err != nil {
			return fmt.Errorf("删除关联的消息失败: %w", err)
		}

		// 13. 软删除会话
		result := tx.Model(&entity.Conversation{}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
			Updates(map[string]interface{}{
				"deleted_at": now,
				"updated_at": now,
			})
		if result.Error != nil {
			return fmt.Errorf("删除会话失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrConversationNotFound
		}

		return nil
	})
}
