package contracts

import (
	"context"
	"time"
)

// ConversationMessage 表示对话中的单条消息。
type ConversationMessage struct {
	ID        ID        `json:"id"`         // 消息 ID
	Role      string    `json:"role"`       // 角色：system / user / assistant
	Content   string    `json:"content"`    // 消息内容
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// ConversationContext 包含 Agent 运行的对话历史和元数据。
type ConversationContext struct {
	ConversationID ID                    `json:"conversation_id"`   // 会话 ID
	Messages       []ConversationMessage `json:"messages"`          // 会话消息列表
	Summary        string                `json:"summary,omitempty"` // 可选：会话摘要（用于压缩长上下文）
	TokenCount     int                   `json:"token_count"`       // 上下文 token 数
}

// ConversationContextService 为 Agent 运行构建对话上下文。
type ConversationContextService interface {
	// Build 为指定的用户、知识库和对话构建 ConversationContext。
	Build(ctx context.Context, userID, knowledgeBaseID, conversationID ID) (ConversationContext, error)
}

// CoreferenceResolver 定义指代消解的接口。
type CoreferenceResolver interface {
	// Resolve 对消息列表进行指代消解，返回改写后的消息。
	Resolve(ctx context.Context, messages []ConversationMessage) ([]ConversationMessage, error)
}
