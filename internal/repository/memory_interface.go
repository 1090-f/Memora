package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
)

// MemoryRepository 定义长期记忆的数据访问接口。
type MemoryRepository interface {
	// Create 创建新的记忆条目。
	Create(ctx context.Context, memory *entity.Memory) error
	// FindByID 根据 ID 和用户 ID 查找记忆。
	FindByID(ctx context.Context, id, userID string) (*entity.Memory, error)
	// Update 更新记忆条目。
	Update(ctx context.Context, memory *entity.Memory) error
	// Delete 软删除记忆条目。
	Delete(ctx context.Context, id, userID string) error
	// UpdateStatus 更新记忆状态。
	UpdateStatus(ctx context.Context, id, userID, status string) error
	// ListByUser 列出用户的记忆列表。
	ListByUser(ctx context.Context, userID string, opts ListMemoryOpts) (*ListMemoryResult, error)
	// SearchByVector 使用向量相似度搜索记忆。
	SearchByVector(ctx context.Context, req VectorSearchRequest) ([]VectorSearchResult, error)
	// SearchByKeyword 使用 PostgreSQL 全文检索搜索记忆。
	SearchByKeyword(ctx context.Context, req KeywordMemorySearchRequest) ([]KeywordMemorySearchResult, error)
	// FindByContentHash 根据内容哈希查找记忆，用于去重。
	FindByContentHash(ctx context.Context, userID, contentHash, scopeType string, scopeID *string) (*entity.Memory, error)
	// UpdateLastAccessedAt 更新记忆的最后访问时间。
	UpdateLastAccessedAt(ctx context.Context, ids []string) error
}

// ListMemoryOpts 表示列出记忆的选项。
type ListMemoryOpts struct {
	MemoryType string
	ScopeType  string
	ScopeID    *string
	Status     string
	Page       int
	PageSize   int
}

// ListMemoryResult 表示列出记忆的结果。
type ListMemoryResult struct {
	Items []entity.Memory
	Total int64
}

// VectorSearchRequest 表示向量搜索请求。
type VectorSearchRequest struct {
	UserID          string
	KnowledgeBaseID *string
	QueryVector     []byte
	EmbeddingDim    int
	TopK            int
	MinImportance   float64
}

// VectorSearchResult 表示向量搜索结果。
type VectorSearchResult struct {
	Memory     entity.Memory
	Similarity float64
}

// KeywordMemorySearchRequest 表示记忆关键词检索请求。
type KeywordMemorySearchRequest struct {
	UserID          string
	KnowledgeBaseID *string
	QueryTokens     []string
	TopK            int
}

// KeywordMemorySearchResult 表示记忆关键词检索结果。
type KeywordMemorySearchResult struct {
	Memory entity.Memory
	Score  float64
}
