package contracts

import "context"

// AgentContextRequest 表示构建 Agent 运行执行上下文的请求。
type AgentContextRequest struct {
	UserID          ID     `json:"user_id"`
	KnowledgeBaseID ID     `json:"knowledge_base_id"`
	ConversationID  ID     `json:"conversation_id"`
	RunID           ID     `json:"run_id"`
	Query           string `json:"query"`
}

// AgentContext 包含 Agent 执行运行所需的全部信息。
type AgentContext struct {
	UserID          ID                  `json:"user_id"`
	KnowledgeBaseID ID                  `json:"knowledge_base_id"`
	ConversationID  ID                  `json:"conversation_id"`
	RunID           ID                  `json:"run_id"`
	Query           string              `json:"query"`
	Conversation    ConversationContext `json:"conversation"`
	Memories        []MemoryQueryResult `json:"memories"`
	AllowedTools    []string            `json:"allowed_tools"`
}

// ContextBuilder 根据请求构建 AgentContext。
type ContextBuilder interface {
	// Build 根据给定的请求构建 AgentContext。
	Build(ctx context.Context, request AgentContextRequest) (AgentContext, error)
}
