package contracts

import (
	"context"
	"time"
)

// RetrievalMode 表示文档检索使用的搜索策略。
type RetrievalMode string

// 检索模式常量。
const (
	// RetrievalKeyword 使用基于关键词的搜索进行文档检索。
	RetrievalKeyword RetrievalMode = "keyword"
	// RetrievalVector 使用向量相似性搜索进行文档检索。
	RetrievalVector RetrievalMode = "vector"
	// RetrievalHybrid 结合关键词和向量搜索进行文档检索。
	RetrievalHybrid RetrievalMode = "hybrid"
)

// SearchConfig 定义文档检索和重排序的参数。
type SearchConfig struct {
	KeywordTopK            int      `json:"keyword_top_k"`
	VectorTopK             int      `json:"vector_top_k"`
	RRFK                   int      `json:"rrf_k"`
	RRFTopK                int      `json:"rrf_top_k"`
	RerankerTopK           int      `json:"reranker_top_k"`
	RerankerThreshold      *float64 `json:"reranker_threshold,omitempty"`
	RerankerModelID        ID       `json:"reranker_model_id,omitempty"`
	MinimumEffectiveResult int      `json:"minimum_effective_results"`
}

// DefaultSearchConfig 返回具有合理默认值的 SearchConfig。
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{KeywordTopK: 30, VectorTopK: 30, RRFK: 60, RRFTopK: 20, RerankerTopK: 8, MinimumEffectiveResult: 1}
}

// RetrievalRequest 表示检索相关文档的请求。
type RetrievalRequest struct {
	UserID          ID            `json:"user_id"`                // 用户标识
	KnowledgeBaseID ID            `json:"knowledge_base_id"`      // 知识库标识
	Query           string        `json:"query"`                  // 查询文本
	Mode            RetrievalMode `json:"mode"`                   // 检索模式
	DocumentIDs     []ID          `json:"document_ids,omitempty"` // 可选：限定检索的文档集合
	TopK            int           `json:"top_k"`                  // 最终返回条数
	Config          SearchConfig  `json:"config"`                 // 检索参数
}

// RetrievalItem 表示带有相关性分数的单个检索文档块。
type RetrievalItem struct {
	DocumentID        ID             `json:"document_id"`
	DocumentTitle     string         `json:"document_title,omitempty"`
	DirectoryID       ID             `json:"directory_id,omitempty"`
	ChunkID           ID             `json:"chunk_id"`
	Content           string         `json:"content"`
	SourceLocation    map[string]any `json:"source_location,omitempty"`
	Score             float64        `json:"score,omitempty"`
	KeywordScore      *float64       `json:"keyword_score,omitempty"`
	VectorScore       *float64       `json:"vector_score,omitempty"`
	KeywordRank       *int           `json:"keyword_rank,omitempty"`
	VectorRank        *int           `json:"vector_rank,omitempty"`
	RRFRank           *int           `json:"rrf_rank,omitempty"`
	RerankerScore     *float64       `json:"reranker_score,omitempty"`
	FinalRank         *int           `json:"final_rank,omitempty"`
	IndexVersion      int            `json:"index_version"`
	DocumentUpdatedAt *time.Time     `json:"document_updated_at,omitempty"`
	Citation          Citation       `json:"citation"`
}

// RetrievalResult 包含文档检索操作的结果。
type RetrievalResult struct {
	Query           string          `json:"query,omitempty"`
	Mode            RetrievalMode   `json:"mode,omitempty"`
	Items           []RetrievalItem `json:"items"`
	RewrittenQuery  string          `json:"rewritten_query,omitempty"`
	KnowledgeStatus string          `json:"knowledge_status"`
	ElapsedMS       int64           `json:"elapsed_ms,omitempty"`
}

// RetrievalService 从知识库中检索相关文档。
type RetrievalService interface {
	// Retrieve 搜索与查询匹配的文档并返回排序后的结果。
	Retrieve(ctx context.Context, request RetrievalRequest) (RetrievalResult, error)
}
