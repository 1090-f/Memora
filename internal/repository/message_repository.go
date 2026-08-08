package repository

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

// messageRepository 是 MessageRepository 接口的 GORM 实现。
type messageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建一个新的消息仓储实例。
func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}

// ListByConversation 获取会话的消息列表，按创建时间升序排列。
func (r *messageRepository) ListByConversation(ctx context.Context, conversationID string, limit int, offset int) ([]entity.Message, error) {
	var messages []entity.Message

	query := r.db.WithContext(ctx).
		Where("conversation_id = ? AND status = ?", conversationID, "completed").
		Order("created_at ASC")

	if offset > 0 {
		query = query.Offset(offset)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list messages by conversation: %w", err)
	}

	return messages, nil
}

// CountByConversation 统计会话的消息数量。
func (r *messageRepository) CountByConversation(ctx context.Context, conversationID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.Message{}).
		Where("conversation_id = ? AND status = ?", conversationID, "completed").
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count messages by conversation: %w", err)
	}
	return count, nil
}
