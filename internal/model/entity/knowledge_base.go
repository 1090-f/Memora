package entity

// KnowledgeBase 表示知识库实体，映射到 knowledge_bases 数据库表。
type KnowledgeBase struct {
	BaseEntity
	UserID                  string  `gorm:"column:user_id" json:"user_id"`
	Name                    string  `gorm:"column:name" json:"name"`
	Description             *string `gorm:"column:description" json:"description"`
	Icon                    *string `gorm:"column:icon" json:"icon"`
	DefaultLanguage         string  `gorm:"column:default_language" json:"default_language"`
	QAEnabled               bool    `gorm:"column:qa_enabled" json:"qa_enabled"`
	AgentEnabled            bool    `gorm:"column:agent_enabled" json:"agent_enabled"`
	NetworkEnabled          bool    `gorm:"column:network_enabled" json:"network_enabled"`
	DefaultChatModelID      *string `gorm:"column:default_chat_model_id" json:"default_chat_model_id,omitempty"`
	DefaultEmbeddingModelID *string `gorm:"column:default_embedding_model_id" json:"default_embedding_model_id,omitempty"`
	DefaultRerankerModelID  *string `gorm:"column:default_reranker_model_id" json:"default_reranker_model_id,omitempty"`
}

// TableName 返回知识库实体对应的数据库表名。
func (KnowledgeBase) TableName() string { return "knowledge_bases" }
