package entity

import "time"

// Message 表示对话中的单条消息，存储用户或助手的消息内容。
type Message struct {
	ID              string    `gorm:"type:uuid;primaryKey" json:"id"`
	ConversationID  string    `gorm:"type:uuid;not null;index" json:"conversation_id"`
	UserID          string    `gorm:"type:uuid;not null;index" json:"user_id"`
	KnowledgeBaseID string    `gorm:"type:uuid;not null" json:"knowledge_base_id"`
	AgentRunID      *string   `gorm:"type:uuid" json:"agent_run_id"`
	Role            string    `gorm:"type:varchar(20);not null" json:"role"` // user / assistant
	Content         string    `gorm:"type:text;not null" json:"content"`
	Citations       []byte    `gorm:"type:jsonb" json:"citations,omitempty"`
	ModelConfigID   *string   `gorm:"type:uuid" json:"model_config_id,omitempty"`
	InputTokens     *int      `gorm:"check:input_tokens >= 0" json:"input_tokens,omitempty"`
	OutputTokens    *int      `gorm:"check:output_tokens >= 0" json:"output_tokens,omitempty"`
	ResponseTimeMs  *int64    `gorm:"check:response_time_ms >= 0" json:"response_time_ms,omitempty"`
	Status          string    `gorm:"type:varchar(20);not null" json:"status"` // streaming / completed / failed
	CreatedAt       time.Time `json:"created_at"`
}

// TableName 指定表名。
func (Message) TableName() string {
	return "messages"
}
