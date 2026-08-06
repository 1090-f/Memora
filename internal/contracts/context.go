package contracts

import "context"

// AgentContextRequest 表示构建 Agent 运行执行上下文的请求。
type AgentContextRequest struct {
	UserID          ID     `json:"user_id"`           // 用户标识
	KnowledgeBaseID ID     `json:"knowledge_base_id"` // 知识库标识
	ConversationID  ID     `json:"conversation_id"`   // 会话标识
	RunID           ID     `json:"run_id"`            // 运行标识
	Query           string `json:"query"`             // 用户查询
}

// AgentContext 包含 Agent 执行运行所需的全部信息。
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

// ContextBuilder 根据请求构建 AgentContext。
type ContextBuilder interface {
	// Build 根据给定的请求构建 AgentContext。
	Build(ctx context.Context, request AgentContextRequest) (AgentContext, error)
}
