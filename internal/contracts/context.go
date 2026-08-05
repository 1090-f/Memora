package contracts

import "context"

// AgentContextRequest 是构建 Agent 上下文的原始入参。
type AgentContextRequest struct {
	UserID          ID     `json:"user_id"`           // 用户标识
	KnowledgeBaseID ID     `json:"knowledge_base_id"` // 知识库标识
	ConversationID  ID     `json:"conversation_id"`   // 会话标识
	RunID           ID     `json:"run_id"`            // 运行标识
	Query           string `json:"query"`             // 用户查询
}

// AgentContext 是一次 Agent 运行所需的全量上下文数据。
type AgentContext struct {
	UserID          ID                  `json:"user_id"`           // 用户标识
	KnowledgeBaseID ID                  `json:"knowledge_base_id"` // 知识库标识
	ConversationID  ID                  `json:"conversation_id"`   // 会话标识
	RunID           ID                  `json:"run_id"`            // 运行标识
	Query           string              `json:"query"`             // 用户查询
	Conversation    ConversationContext `json:"conversation"`      // 会话历史上下文
	Memories        []MemoryQueryResult `json:"memories"`          // 检索到的记忆
	AllowedTools    []string            `json:"allowed_tools"`     // 允许使用的工具白名单
}

// ContextBuilder 负责根据请求构建完整的 Agent 上下文。
type ContextBuilder interface {
	// Build 汇总会话、记忆、工具等数据并组装为 AgentContext。
	Build(ctx context.Context, request AgentContextRequest) (AgentContext, error)
}
