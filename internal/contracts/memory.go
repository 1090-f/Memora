package contracts

import (
	"context"
	"time"
)

// MemoryType 表示记忆条目的类别。
type MemoryType string

// MemoryScope 表示记忆条目存在的作用域。
type MemoryScope string

// 预定义的记忆类型与作用域常量。
const (
	// MemoryPreference 存储用户偏好和设置。
	MemoryPreference MemoryType  = "preference"
	// MemoryProject 存储项目相关信息。
	MemoryProject    MemoryType  = "project"
	// MemoryDecision 存储对话中做出的决策。
	MemoryDecision   MemoryType  = "decision"
	// MemoryGoal 存储用户目标和目的。
	MemoryGoal       MemoryType  = "goal"
	// MemoryFact 存储事实信息。
	MemoryFact       MemoryType  = "fact"
	// MemoryProgress 存储进度跟踪信息。
	MemoryProgress   MemoryType  = "progress"
	// MemoryScopeUser 表示记忆的作用域为特定用户。
	MemoryScopeUser  MemoryScope = "user"
	// MemoryScopeKB 表示记忆的作用域为知识库。
	MemoryScopeKB    MemoryScope = "knowledge_base"
)

// MemoryQuery 表示检索相关记忆的查询。
type MemoryQuery struct {
	UserID          ID     `json:"user_id"`                     // 用户标识
	KnowledgeBaseID ID     `json:"knowledge_base_id,omitempty"` // 可选：限定知识库
	Query           string `json:"query"`                       // 语义检索查询文本
	TopK            int    `json:"top_k"`                       // 返回条数上限
}

// MemoryQueryResult 表示从查询返回的单个记忆条目。
type MemoryQueryResult struct {
	MemoryID   ID          `json:"memory_id"`          // 记忆 ID
	MemoryType MemoryType  `json:"memory_type"`        // 记忆类型
	ScopeType  MemoryScope `json:"scope_type"`         // 作用域类型
	ScopeID    ID          `json:"scope_id,omitempty"` // 可选：作用域内对应的实体 ID
	Content    string      `json:"content"`            // 记忆内容
	Similarity float64     `json:"similarity"`         // 相似度得分
	Importance float64     `json:"importance"`         // 重要程度
	UpdatedAt  time.Time   `json:"updated_at"`         // 更新时间
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
