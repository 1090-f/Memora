package contracts

import (
	"context"
	"time"
)

// ConversationMessage 是会话中的单条消息。
type ConversationMessage struct {
	ID        ID        `json:"id"`         // 消息 ID
	Role      string    `json:"role"`       // 角色：system / user / assistant
	Content   string    `json:"content"`    // 消息内容
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// ConversationContext 是一次 Agent 运行所使用到的会话上下文。
type ConversationContext struct {
	ConversationID ID                    `json:"conversation_id"` // 会话 ID
	Messages       []ConversationMessage `json:"messages"`       // 会话消息列表
	Summary        string                `json:"summary,omitempty"` // 可选：会话摘要（用于压缩长上下文）
	TokenCount     int                   `json:"token_count"`     // 上下文 token 数
}

// ConversationContextService 负责构建会话上下文。
type ConversationContextService interface {
	// Build 按用户、知识库、会话获取历史消息并组装为会话上下文。
	Build(ctx context.Context, userID, knowledgeBaseID, conversationID ID) (ConversationContext, error)
}