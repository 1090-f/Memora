package contracts

import (
	"context"
	"time"
)

type MemoryType string
type MemoryScope string

const (
	MemoryPreference MemoryType  = "preference"
	MemoryProject    MemoryType  = "project"
	MemoryDecision   MemoryType  = "decision"
	MemoryGoal       MemoryType  = "goal"
	MemoryFact       MemoryType  = "fact"
	MemoryProgress   MemoryType  = "progress"
	MemoryScopeUser  MemoryScope = "user"
	MemoryScopeKB    MemoryScope = "knowledge_base"
)

type MemoryQuery struct {
	UserID          ID     `json:"user_id"`
	KnowledgeBaseID ID     `json:"knowledge_base_id,omitempty"`
	Query           string `json:"query"`
	TopK            int    `json:"top_k"`
}

type MemoryQueryResult struct {
	MemoryID   ID          `json:"memory_id"`
	MemoryType MemoryType  `json:"memory_type"`
	ScopeType  MemoryScope `json:"scope_type"`
	ScopeID    ID          `json:"scope_id,omitempty"`
	Content    string      `json:"content"`
	Similarity float64     `json:"similarity"`
	Importance float64     `json:"importance"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type MemoryRetriever interface {
	Retrieve(ctx context.Context, query MemoryQuery) ([]MemoryQueryResult, error)
}

type MemoryExtractor interface {
	Extract(ctx context.Context, agentContext AgentContext, answer string) error
}
