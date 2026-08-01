package contracts

import "context"

type RetrievalMode string

const (
	RetrievalKeyword RetrievalMode = "keyword"
	RetrievalVector  RetrievalMode = "vector"
	RetrievalHybrid  RetrievalMode = "hybrid"
)

type SearchConfig struct {
	KeywordTopK            int      `json:"keyword_top_k"`
	VectorTopK             int      `json:"vector_top_k"`
	RRFK                   int      `json:"rrf_k"`
	RRFTopK                int      `json:"rrf_top_k"`
	RerankerTopK           int      `json:"reranker_top_k"`
	RerankerThreshold      *float64 `json:"reranker_threshold,omitempty"`
	MinimumEffectiveResult int      `json:"minimum_effective_results"`
}

func DefaultSearchConfig() SearchConfig {
	return SearchConfig{KeywordTopK: 30, VectorTopK: 30, RRFK: 60, RRFTopK: 20, RerankerTopK: 8, MinimumEffectiveResult: 1}
}

type RetrievalRequest struct {
	UserID          ID            `json:"user_id"`
	KnowledgeBaseID ID            `json:"knowledge_base_id"`
	Query           string        `json:"query"`
	Mode            RetrievalMode `json:"mode"`
	DocumentIDs     []ID          `json:"document_ids,omitempty"`
	TopK            int           `json:"top_k"`
	Config          SearchConfig  `json:"config"`
}

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

type RetrievalResult struct {
	Items           []RetrievalItem `json:"items"`
	RewrittenQuery  string          `json:"rewritten_query,omitempty"`
	KnowledgeStatus string          `json:"knowledge_status"`
}

type RetrievalService interface {
	Retrieve(ctx context.Context, request RetrievalRequest) (RetrievalResult, error)
}
