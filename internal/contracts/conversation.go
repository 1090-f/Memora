package contracts

import (
	"context"
	"time"
)

// ConversationMessage 表示对话中的单条消息。
type ConversationMessage struct {
	ID        ID        `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ConversationContext 包含 Agent 运行的对话历史和元数据。
type ConversationContext struct {
	ConversationID ID                    `json:"conversation_id"`
	Messages       []ConversationMessage `json:"messages"`
	Summary        string                `json:"summary,omitempty"`
	TokenCount     int                   `json:"token_count"`
}

// ConversationContextService 为 Agent 运行构建对话上下文。
type ConversationContextService interface {
	// Build 为指定的用户、知识库和对话构建 ConversationContext。
	Build(ctx context.Context, userID, knowledgeBaseID, conversationID ID) (ConversationContext, error)
}
