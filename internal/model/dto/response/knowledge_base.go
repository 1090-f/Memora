package response

import "time"

// KnowledgeBaseResponse 表示知识库详情的响应。
type KnowledgeBaseResponse struct {
	ID                      string    `json:"id"`
	Name                    string    `json:"name"`
	Description             *string   `json:"description,omitempty"`
	Icon                    *string   `json:"icon,omitempty"`
	DefaultLanguage         string    `json:"default_language"`
	QAEnabled               bool      `json:"qa_enabled"`
	AgentEnabled            bool      `json:"agent_enabled"`
	NetworkEnabled          bool      `json:"network_enabled"`
	DefaultChatModelID      *string   `json:"default_chat_model_id,omitempty"`
	DefaultEmbeddingModelID *string   `json:"default_embedding_model_id,omitempty"`
	DefaultRerankerModelID  *string   `json:"default_reranker_model_id,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// KnowledgeBaseListItem 表示知识库列表项。
type KnowledgeBaseListItem struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Icon           *string   `json:"icon,omitempty"`
	Description    *string   `json:"description,omitempty"`
	DocumentCount  int64     `json:"document_count"`
	AgentEnabled   bool      `json:"agent_enabled"`
	NetworkEnabled bool      `json:"network_enabled"`
	UpdatedAt      time.Time `json:"updated_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// KnowledgeBaseList 表示知识库分页列表响应。
type KnowledgeBaseList struct {
	Items    []*KnowledgeBaseListItem `json:"items"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Total    int64                    `json:"total"`
}

// SearchConfigResponse 表示搜索配置的响应。
type SearchConfigResponse struct {
	KeywordTopK            int      `json:"keyword_top_k"`
	VectorTopK             int      `json:"vector_top_k"`
	RRFK                   int      `json:"rrf_k"`
	RRFTopK                int      `json:"rrf_top_k"`
	RerankerTopK           int      `json:"reranker_top_k"`
	RerankerThreshold      *float64 `json:"reranker_threshold,omitempty"`
	MinimumEffectiveResult int      `json:"minimum_effective_results"`
	RerankerModelID        *string  `json:"reranker_model_id,omitempty"`
}
