package entity

import "time"

// Memory 表示用户的长期记忆条目，存储偏好、项目、决策、目标等信息。
type Memory struct {
	ID                   string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserID               string     `gorm:"type:uuid;not null" json:"user_id"`
	MemoryType           string     `gorm:"type:varchar(20);not null" json:"memory_type"`
	ScopeType            string     `gorm:"type:varchar(20);not null" json:"scope_type"`
	ScopeID              *string    `gorm:"type:uuid" json:"scope_id"`
	Content              string     `gorm:"type:text;not null" json:"content"`
	Summary              string     `gorm:"type:text" json:"summary"`
	Importance           float64    `gorm:"type:numeric(5,4);not null" json:"importance"`
	ContentHash          string     `gorm:"type:varchar(64);not null" json:"content_hash"`
	Embedding            []byte     `gorm:"type:vector" json:"-"`
	EmbeddingDim         int        `gorm:"not null" json:"embedding_dim"`
	EmbeddingModelID     string     `gorm:"type:uuid;not null" json:"embedding_model_id"`
	SourceConversationID *string    `gorm:"type:uuid" json:"source_conversation_id"`
	SourceMessageID      *string    `gorm:"type:uuid" json:"source_message_id"`
	SourceAgentRunID     *string    `gorm:"type:uuid" json:"source_agent_run_id"`
	Status               string     `gorm:"type:varchar(20);not null;default:active" json:"status"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	LastAccessedAt       *time.Time `json:"last_accessed_at"`
	DeletedAt            *time.Time `json:"-"`
}

// TableName 指定表名。
func (Memory) TableName() string {
	return "memories"
}
