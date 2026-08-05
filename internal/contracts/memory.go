package contracts

import (
	"context"
	"time"
)

// MemoryType 表示一条记忆的业务类型。
type MemoryType string

// MemoryScope 表示一条记忆的生效范围。
type MemoryScope string

// 预定义的记忆类型与作用域常量。
const (
	MemoryPreference MemoryType  = "preference"     // 用户偏好
	MemoryProject    MemoryType  = "project"        // 项目相关
	MemoryDecision   MemoryType  = "decision"       // 决策记录
	MemoryGoal       MemoryType  = "goal"           // 目标
	MemoryFact       MemoryType  = "fact"           // 客观事实
	MemoryProgress   MemoryType  = "progress"       // 进度
	MemoryScopeUser  MemoryScope = "user"           // 作用域：全局用户级
	MemoryScopeKB    MemoryScope = "knowledge_base" // 作用域：限定知识库
)

// MemoryQuery 是记忆检索请求。
type MemoryQuery struct {
	UserID          ID     `json:"user_id"`                     // 用户标识
	KnowledgeBaseID ID     `json:"knowledge_base_id,omitempty"` // 可选：限定知识库
	Query           string `json:"query"`                       // 语义检索查询文本
	TopK            int    `json:"top_k"`                       // 返回条数上限
}

// MemoryQueryResult 是检索到的单条记忆。
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

// MemoryRetriever 抽象记忆语义检索能力。
type MemoryRetriever interface {
	// Retrieve 按查询检索相关记忆。
	Retrieve(ctx context.Context, query MemoryQuery) ([]MemoryQueryResult, error)
}

// MemoryExtractor 负责从 Agent 回答中提取并沉淀记忆。
type MemoryExtractor interface {
	// Extract 根据上下文与回答萃取新的记忆。
	Extract(ctx context.Context, agentContext AgentContext, answer string) error
}
