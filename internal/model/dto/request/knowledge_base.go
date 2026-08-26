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
	// EmbeddingModelID 知识库唯一绑定的向量模型 ID，必填，创建后不可修改。
	EmbeddingModelID string `json:"embedding_model_id" binding:"required"`
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
	// DuplicatePolicy 文档导入重复处理策略（skip/create_new）。
	DuplicatePolicy *string `json:"duplicate_policy" binding:"omitempty,oneof=skip create_new"`
	// ChunkStrategy 知识库级分块策略；inherit 清除覆盖并继承环境配置。
	ChunkStrategy *string `json:"chunk_strategy" binding:"omitempty,oneof=structured paragraph recursive_fallback auto inherit"`
}
