package response

import "time"

// KnowledgeBaseResponse 表示知识库详情的响应。
type KnowledgeBaseResponse struct {
	ID                      string    `json:"id"`                                   // ID 知识库 ID
	Name                    string    `json:"name"`                                 // Name 知识库名称
	Description             *string   `json:"description,omitempty"`                // Description 知识库描述，可选
	Icon                    *string   `json:"icon,omitempty"`                       // Icon 知识库图标 URL，可选
	DefaultLanguage         string    `json:"default_language"`                     // DefaultLanguage 默认语言（如 "zh"、"en"）
	QAEnabled               bool      `json:"qa_enabled"`                           // QAEnabled 是否启用问答功能
	AgentEnabled            bool      `json:"agent_enabled"`                        // AgentEnabled 是否启用 Agent 功能
	NetworkEnabled          bool      `json:"network_enabled"`                      // NetworkEnabled 是否启用联网搜索
	DefaultChatModelID      *string   `json:"default_chat_model_id,omitempty"`      // DefaultChatModelID 默认对话模型配置 ID，可选
	DefaultEmbeddingModelID *string   `json:"default_embedding_model_id,omitempty"` // DefaultEmbeddingModelID 默认向量化模型配置 ID，可选
	DefaultRerankerModelID  *string   `json:"default_reranker_model_id,omitempty"`  // DefaultRerankerModelID 默认重排序模型配置 ID，可选
	CreatedAt               time.Time `json:"created_at"`                           // CreatedAt 创建时间
	UpdatedAt               time.Time `json:"updated_at"`                           // UpdatedAt 更新时间
}

// KnowledgeBaseListItem 表示知识库列表项。
type KnowledgeBaseListItem struct {
	ID             string    `json:"id"`                    // ID 知识库 ID
	Name           string    `json:"name"`                  // Name 知识库名称
	Icon           *string   `json:"icon,omitempty"`        // Icon 知识库图标 URL，可选
	Description    *string   `json:"description,omitempty"` // Description 知识库描述，可选
	DocumentCount  int64     `json:"document_count"`        // DocumentCount 知识库内的文档数量
	AgentEnabled   bool      `json:"agent_enabled"`         // AgentEnabled 是否启用 Agent 功能
	NetworkEnabled bool      `json:"network_enabled"`       // NetworkEnabled 是否启用联网搜索
	UpdatedAt      time.Time `json:"updated_at"`            // UpdatedAt 更新时间
	CreatedAt      time.Time `json:"created_at"`            // CreatedAt 创建时间
}

// KnowledgeBaseList 表示知识库分页列表响应。
type KnowledgeBaseList struct {
	Items    []*KnowledgeBaseListItem `json:"items"`     // Items 知识库列表项
	Page     int                      `json:"page"`      // Page 当前页码（从 1 开始）
	PageSize int                      `json:"page_size"` // PageSize 每页条数
	Total    int64                    `json:"total"`     // Total 知识库总数
}

// SearchConfigResponse 表示搜索配置的响应。
type SearchConfigResponse struct {
	KeywordTopK            int      `json:"keyword_top_k"`                // KeywordTopK 关键词检索返回的候选条数
	VectorTopK             int      `json:"vector_top_k"`                 // VectorTopK 向量检索返回的候选条数
	RRFK                   int      `json:"rrf_k"`                        // RRFK 混合检索融合算法（RRF）的常数 k
	RRFTopK                int      `json:"rrf_top_k"`                    // RRFTopK RRF 融合后保留的候选条数
	RerankerTopK           int      `json:"reranker_top_k"`               // RerankerTopK 重排序后返回的条数
	RerankerThreshold      *float64 `json:"reranker_threshold,omitempty"` // RerankerThreshold 重排序分数阈值，低于该分数的结果被过滤，可选
	MinimumEffectiveResult int      `json:"minimum_effective_results"`    // MinimumEffectiveResult 最低有效结果数，防止检索结果过少
	RerankerModelID        *string  `json:"reranker_model_id,omitempty"`  // RerankerModelID 重排序模型配置 ID，可选
}
