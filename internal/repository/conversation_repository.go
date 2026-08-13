package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

// ErrConversationNotFound 表示未找到指定的会话。
var ErrConversationNotFound = errors.New("conversation not found")

// conversationRepository 是 ConversationRepository 接口的 GORM 实现。
type conversationRepository struct{ db *gorm.DB }

// NewConversationRepository 创建一个新的会话仓储实例。
func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepository{db: db}
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

// Delete 软删除会话。
func (r *conversationRepository) Delete(ctx context.Context, id, userID string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&entity.Conversation{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("delete conversation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrConversationNotFound
	}
	return nil
}
