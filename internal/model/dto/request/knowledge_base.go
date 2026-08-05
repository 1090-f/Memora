package request

// CreateKnowledgeBaseRequest 表示创建知识库的请求。
type CreateKnowledgeBaseRequest struct {
	Name                    string  `json:"name" binding:"required,max=128"`
	Description             *string `json:"description" binding:"omitempty,max=2000"`
	Icon                    *string `json:"icon" binding:"omitempty,max=255"`
	DefaultLanguage         string  `json:"default_language" binding:"omitempty,max=32"`
	QAEnabled               *bool   `json:"qa_enabled"`
	AgentEnabled            *bool   `json:"agent_enabled"`
	NetworkEnabled          *bool   `json:"network_enabled"`
	DefaultChatModelID      *string `json:"default_chat_model_id" binding:"omitempty"`
	DefaultEmbeddingModelID *string `json:"default_embedding_model_id" binding:"omitempty"`
	DefaultRerankerModelID  *string `json:"default_reranker_model_id" binding:"omitempty"`
}

// UpdateKnowledgeBaseRequest 表示修改知识库的请求，所有字段均为可选。
type UpdateKnowledgeBaseRequest struct {
	Name                    *string `json:"name" binding:"omitempty,max=128"`
	Description             *string `json:"description" binding:"omitempty,max=2000"`
	Icon                    *string `json:"icon" binding:"omitempty,max=255"`
	DefaultLanguage         *string `json:"default_language" binding:"omitempty,max=32"`
	QAEnabled               *bool   `json:"qa_enabled"`
	AgentEnabled            *bool   `json:"agent_enabled"`
	NetworkEnabled          *bool   `json:"network_enabled"`
	DefaultChatModelID      *string `json:"default_chat_model_id" binding:"omitempty"`
	DefaultEmbeddingModelID *string `json:"default_embedding_model_id" binding:"omitempty"`
	DefaultRerankerModelID  *string `json:"default_reranker_model_id" binding:"omitempty"`
}
