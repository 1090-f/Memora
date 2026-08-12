package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
)

// ConversationRepository 定义会话数据访问接口。
type ConversationRepository interface {
	// Create 创建新的会话。
	Create(ctx context.Context, conversation *entity.Conversation) error
	// FindByID 根据 ID 和用户 ID 查找会话（带权限校验）。
	FindByID(ctx context.Context, id, userID string) (*entity.Conversation, error)
	// FindByIDWithoutUser 根据 ID 查找会话（不校验用户，用于内部调用）。
	FindByIDWithoutUser(ctx context.Context, id string) (*entity.Conversation, error)
	// ListByKnowledgeBase 列出知识库的会话列表。
	ListByKnowledgeBase(ctx context.Context, userID, kbID string, page, pageSize int) ([]entity.Conversation, int64, error)
	// Update 更新会话。
	Update(ctx context.Context, conversation *entity.Conversation) error
	// Delete 软删除会话。
	Delete(ctx context.Context, id, userID string) error
}
