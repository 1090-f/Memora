package entity

// KnowledgeBase 表示知识库实体，映射到 knowledge_bases 数据库表。
type KnowledgeBase struct {
	BaseEntity
	UserID                  string  `gorm:"column:user_id" json:"user_id"`                                                 // UserID 所属用户 ID
	Name                    string  `gorm:"column:name" json:"name"`                                                       // Name 知识库名称
	Description             *string `gorm:"column:description" json:"description"`                                         // Description 知识库描述，可选
	Icon                    *string `gorm:"column:icon" json:"icon"`                                                       // Icon 知识库图标 URL，可选
	DefaultLanguage         string  `gorm:"column:default_language" json:"default_language"`                               // DefaultLanguage 默认语言（如 "zh"、"en"）
	QAEnabled               bool    `gorm:"column:qa_enabled" json:"qa_enabled"`                                           // QAEnabled 是否启用问答功能
	AgentEnabled            bool    `gorm:"column:agent_enabled" json:"agent_enabled"`                                     // AgentEnabled 是否启用 Agent 功能
	NetworkEnabled          bool    `gorm:"column:network_enabled" json:"network_enabled"`                                 // NetworkEnabled 是否启用联网搜索
	DefaultChatModelID      *string `gorm:"column:default_chat_model_id" json:"default_chat_model_id,omitempty"`           // DefaultChatModelID 默认对话模型配置 ID，可选
	DefaultEmbeddingModelID *string `gorm:"column:default_embedding_model_id" json:"default_embedding_model_id,omitempty"` // DefaultEmbeddingModelID 默认向量化模型配置 ID，可选
	DefaultRerankerModelID  *string `gorm:"column:default_reranker_model_id" json:"default_reranker_model_id,omitempty"`   // DefaultRerankerModelID 默认重排序模型配置 ID，可选
	DuplicatePolicy         string  `gorm:"column:duplicate_policy" json:"duplicate_policy"`                               // DuplicatePolicy 文档导入重复处理策略（skip/create_new）
}

// TableName 返回知识库实体对应的数据库表名。
func (KnowledgeBase) TableName() string { return "knowledge_bases" }
