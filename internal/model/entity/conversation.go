package entity

import "time"

// Conversation 表示用户与知识库的对话会话。
type Conversation struct {
	ID              string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          string     `gorm:"type:uuid;not null;index" json:"user_id"`
	KnowledgeBaseID string     `gorm:"type:uuid;not null;index" json:"knowledge_base_id"`
	Title           string     `gorm:"type:varchar(255)" json:"title"`
	Status          string     `gorm:"type:varchar(20);not null;default:active" json:"status"` // active / archived
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名。
func (Conversation) TableName() string {
	return "conversations"
}
