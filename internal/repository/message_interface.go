package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
)

// MessageRepository 定义消息数据访问接口。
type MessageRepository interface {
	// Create 创建一条会话消息。
	Create(ctx context.Context, message *entity.Message) error

	// ListByConversation 获取会话的消息列表，按创建时间升序排列。
	ListByConversation(ctx context.Context, conversationID string, limit int, offset int) ([]entity.Message, error)
	// CountByConversation 统计会话的消息数量。
	CountByConversation(ctx context.Context, conversationID string) (int64, error)

	// DeleteByConversationID 删除指定会话的所有消息。
	DeleteByConversationID(ctx context.Context, conversationID string) error
}
