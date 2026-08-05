package entity

// ModelConfig 表示 AI 模型配置实体，映射到 ai_model_configs 数据库表。
// 成员一只做最小只读查询（默认 Chat 模型、模型归属与类型校验），完整模型配置业务由成员二负责。
type ModelConfig struct {
	BaseEntity
	UserID          string `gorm:"column:user_id" json:"user_id"`
	ModelType       string `gorm:"column:model_type" json:"model_type"`
	Provider        string `gorm:"column:provider" json:"provider"`
	Name            string `gorm:"column:name" json:"name"`
	IsDefault       bool   `gorm:"column:is_default" json:"is_default"`
	Enabled         bool   `gorm:"column:enabled" json:"enabled"`
	VectorDimension *int   `gorm:"column:vector_dimension" json:"vector_dimension,omitempty"`
}

// TableName 返回模型配置实体对应的数据库表名。
func (ModelConfig) TableName() string { return "ai_model_configs" }
