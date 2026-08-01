package contracts

import "context"

type AgentContextRequest struct {
	UserID          ID     `json:"user_id"`
	KnowledgeBaseID ID     `json:"knowledge_base_id"`
	ConversationID  ID     `json:"conversation_id"`
	RunID           ID     `json:"run_id"`
	Query           string `json:"query"`
}

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

type ContextBuilder interface {
	Build(ctx context.Context, request AgentContextRequest) (AgentContext, error)
}
