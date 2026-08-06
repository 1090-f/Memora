package entity

import "time"

// AIModelConfig 表示 AI 模型配置，存储聊天、嵌入、重排模型的连接参数和推理配置。
type AIModelConfig struct {
	ID                  string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserID              string     `gorm:"type:uuid;not null" json:"user_id"`
	ModelType           string     `gorm:"type:varchar(20);not null" json:"model_type"`
	Provider            string     `gorm:"type:varchar(64);not null" json:"provider"`
	Name                string     `gorm:"type:varchar(128);not null" json:"name"`
	BaseURL             string     `gorm:"type:text;not null" json:"base_url"`
	APIKeyCiphertext    []byte     `gorm:"type:bytea" json:"-"`
	APIKeyMasked        string     `gorm:"type:varchar(64)" json:"api_key_masked"`
	TimeoutSeconds      int        `gorm:"not null;default:60" json:"timeout_seconds"`
	RetryTimes          int        `gorm:"not null;default:2" json:"retry_times"`
	MaxTokens           *int       `json:"max_tokens"`
	Temperature         *float64   `json:"temperature"`
	VectorDimension     *int       `json:"vector_dimension"`
	SupportsToolCalling bool       `gorm:"not null;default:false" json:"supports_tool_calling"`
	SupportsStreaming   bool       `gorm:"not null;default:false" json:"supports_streaming"`
	IsDefault           bool       `gorm:"not null;default:false" json:"is_default"`
	Enabled             bool       `gorm:"not null;default:true" json:"enabled"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"-"`
}

// TableName 指定表名。
func (AIModelConfig) TableName() string {
	return "ai_model_configs"
}
