package request

// UpdateSearchConfigRequest 表示更新搜索配置的请求，所有字段均为可选。
type UpdateSearchConfigRequest struct {
	KeywordTopK            *int     `json:"keyword_top_k" binding:"omitempty,gt=0"`
	VectorTopK             *int     `json:"vector_top_k" binding:"omitempty,gt=0"`
	RRFK                   *int     `json:"rrf_k" binding:"omitempty,gt=0"`
	RRFTopK                *int     `json:"rrf_top_k" binding:"omitempty,gt=0"`
	RerankerTopK           *int     `json:"reranker_top_k" binding:"omitempty,gt=0"`
	RerankerThreshold      *float64 `json:"reranker_threshold" binding:"omitempty"`
	MinimumEffectiveResult *int     `json:"minimum_effective_results" binding:"omitempty,gt=0"`
	RerankerModelID        *string  `json:"reranker_model_id" binding:"omitempty"`
}
