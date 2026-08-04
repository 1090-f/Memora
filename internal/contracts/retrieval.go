package contracts

import "context"

// RetrievalMode 表示文档检索使用的搜索策略。
type RetrievalMode string

const (
	// RetrievalKeyword 使用基于关键词的搜索进行文档检索。
	RetrievalKeyword RetrievalMode = "keyword"
	// RetrievalVector 使用向量相似性搜索进行文档检索。
	RetrievalVector  RetrievalMode = "vector"
	// RetrievalHybrid 结合关键词和向量搜索进行文档检索。
	RetrievalHybrid  RetrievalMode = "hybrid"
)

// SearchConfig 定义文档检索和重排序的参数。
type SearchConfig struct {
	KeywordTopK            int      `json:"keyword_top_k"`
	VectorTopK             int      `json:"vector_top_k"`
	RRFK                   int      `json:"rrf_k"`
	RRFTopK                int      `json:"rrf_top_k"`
	RerankerTopK           int      `json:"reranker_top_k"`
	RerankerThreshold      *float64 `json:"reranker_threshold,omitempty"`
	MinimumEffectiveResult int      `json:"minimum_effective_results"`
}

// DefaultSearchConfig 返回具有合理默认值的 SearchConfig。
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{KeywordTopK: 30, VectorTopK: 30, RRFK: 60, RRFTopK: 20, RerankerTopK: 8, MinimumEffectiveResult: 1}
}

// RetrievalRequest 表示检索相关文档的请求。
type RetrievalRequest struct {
	UserID          ID            `json:"user_id"`
	KnowledgeBaseID ID            `json:"knowledge_base_id"`
	Query           string        `json:"query"`
	Mode            RetrievalMode `json:"mode"`
	DocumentIDs     []ID          `json:"document_ids,omitempty"`
	TopK            int           `json:"top_k"`
	Config          SearchConfig  `json:"config"`
}

// RetrievalItem 表示带有相关性分数的单个检索文档块。
type RetrievalItem struct {
	DocumentID   ID       `json:"document_id"`
	ChunkID      ID       `json:"chunk_id"`
	Content      string   `json:"content"`
	Score        float64  `json:"score"`
	KeywordRank  *int     `json:"keyword_rank,omitempty"`
	VectorRank   *int     `json:"vector_rank,omitempty"`
	IndexVersion int      `json:"index_version"`
	Citation     Citation `json:"citation"`
}

// RetrievalResult 包含文档检索操作的结果。
type RetrievalResult struct {
	Items           []RetrievalItem `json:"items"`
	RewrittenQuery  string          `json:"rewritten_query,omitempty"`
	KnowledgeStatus string          `json:"knowledge_status"`
}

// RetrievalService 从知识库中检索相关文档。
type RetrievalService interface {
	// Retrieve 搜索与查询匹配的文档并返回排序后的结果。
	Retrieve(ctx context.Context, request RetrievalRequest) (RetrievalResult, error)
}
