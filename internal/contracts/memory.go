package contracts

import (
	"context"
	"time"
)

// MemoryType 表示记忆条目的类别。
type MemoryType string

// MemoryScope 表示记忆条目存在的作用域。
type MemoryScope string

const (
	// MemoryPreference 存储用户偏好和设置。
	MemoryPreference MemoryType = "preference"
	// MemoryProject 存储项目相关信息。
	MemoryProject MemoryType = "project"
	// MemoryDecision 存储对话中做出的决策。
	MemoryDecision MemoryType = "decision"
	// MemoryGoal 存储用户目标和目的。
	MemoryGoal MemoryType = "goal"
	// MemoryFact 存储事实信息。
	MemoryFact MemoryType = "fact"
	// MemoryProgress 存储进度跟踪信息。
	MemoryProgress MemoryType = "progress"
	// MemoryScopeUser 表示记忆的作用域为特定用户。
	MemoryScopeUser MemoryScope = "user"
	// MemoryScopeKB 表示记忆的作用域为知识库。
	MemoryScopeKB MemoryScope = "knowledge_base"
)

// MemoryQuery 表示检索相关记忆的查询。
type MemoryQuery struct {
	UserID          ID     `json:"user_id"`
	KnowledgeBaseID ID     `json:"knowledge_base_id,omitempty"`
	Query           string `json:"query"`
	TopK            int    `json:"top_k"`
}

// MemoryQueryResult 表示从查询返回的单个记忆条目。
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

// MemoryRetriever 基于查询检索相关记忆。
type MemoryRetriever interface {
	// Retrieve 搜索与查询匹配的记忆并返回排序后的结果。
	Retrieve(ctx context.Context, query MemoryQuery) ([]MemoryQueryResult, error)
}

// MemoryExtractor 从 Agent 响应中提取并存储记忆。
type MemoryExtractor interface {
	// Extract 处理 Agent 的回答并存储相关记忆。
	Extract(ctx context.Context, agentContext AgentContext, answer string) error
}
