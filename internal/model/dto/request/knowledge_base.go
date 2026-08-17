package request

// CreateKnowledgeBaseRequest 表示创建知识库的请求。
type CreateKnowledgeBaseRequest struct {
	// Name 知识库名称，必填，最长 128 字符。
	Name string `json:"name" binding:"required,max=128"`
	// Description 知识库描述，可选，最长 2000 字符。
	Description *string `json:"description" binding:"omitempty,max=2000"`
	// Icon 知识库图标 URL，可选，最长 255 字符。
	Icon *string `json:"icon" binding:"omitempty,max=255"`
	// DefaultLanguage 知识库默认语言（如 "zh"、"en"），可选。
	DefaultLanguage string `json:"default_language" binding:"omitempty,max=32"`
	// QAEnabled 是否启用问答功能。
	QAEnabled *bool `json:"qa_enabled"`
	// AgentEnabled 是否启用 Agent 功能。
	AgentEnabled *bool `json:"agent_enabled"`
	// NetworkEnabled 是否启用联网搜索。
	NetworkEnabled *bool `json:"network_enabled"`
	// DefaultChatModelID 默认对话模型 ID，可选。
	DefaultChatModelID *string `json:"default_chat_model_id" binding:"omitempty"`
	// DefaultEmbeddingModelID 默认向量化模型 ID，可选。
	DefaultEmbeddingModelID *string `json:"default_embedding_model_id" binding:"omitempty"`
	// DefaultRerankerModelID 默认重排序模型 ID，可选。
	DefaultRerankerModelID *string `json:"default_reranker_model_id" binding:"omitempty"`
}

// UpdateKnowledgeBaseRequest 表示修改知识库的请求，所有字段均为可选。
// 仅传入需要修改的字段，未传入的字段保持不变。
type UpdateKnowledgeBaseRequest struct {
	// Name 知识库名称，最长 128 字符。
	Name *string `json:"name" binding:"omitempty,max=128"`
	// Description 知识库描述，最长 2000 字符。
	Description *string `json:"description" binding:"omitempty,max=2000"`
	// Icon 知识库图标 URL，最长 255 字符。
	Icon *string `json:"icon" binding:"omitempty,max=255"`
	// DefaultLanguage 知识库默认语言（如 "zh"、"en"）。
	DefaultLanguage *string `json:"default_language" binding:"omitempty,max=32"`
	// QAEnabled 是否启用问答功能。
	QAEnabled *bool `json:"qa_enabled"`
	// AgentEnabled 是否启用 Agent 功能。
	AgentEnabled *bool `json:"agent_enabled"`
	// NetworkEnabled 是否启用联网搜索。
	NetworkEnabled *bool `json:"network_enabled"`
	// DefaultChatModelID 默认对话模型 ID。
	DefaultChatModelID *string `json:"default_chat_model_id" binding:"omitempty"`
	// DefaultEmbeddingModelID 默认向量化模型 ID。
	DefaultEmbeddingModelID *string `json:"default_embedding_model_id" binding:"omitempty"`
	// DefaultRerankerModelID 默认重排序模型 ID。
	DefaultRerankerModelID *string `json:"default_reranker_model_id" binding:"omitempty"`
	// DuplicatePolicy 文档导入重复处理策略（skip/create_new）。
	DuplicatePolicy *string `json:"duplicate_policy" binding:"omitempty,oneof=skip create_new"`
}
