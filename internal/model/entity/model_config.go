package entity

// ModelConfig 表示 AI 模型配置实体，映射到 ai_model_configs 数据库表。
// 成员一只做最小只读查询（默认 Chat 模型、模型归属与类型校验），完整模型配置业务由成员二负责。
// ModelType 取值：chat（对话）/ embedding（向量化）/ reranker（重排序）。
type ModelConfig struct {
	BaseEntity
	UserID          string `gorm:"column:user_id" json:"user_id"`                             // UserID 所属用户 ID
	ModelType       string `gorm:"column:model_type" json:"model_type"`                       // ModelType 模型类型：chat/embedding/reranker
	Provider        string `gorm:"column:provider" json:"provider"`                           // Provider 模型提供商（如 OpenAI、通义千问等）
	Name            string `gorm:"column:name" json:"name"`                                   // Name 模型名称
	IsDefault       bool   `gorm:"column:is_default" json:"is_default"`                       // IsDefault 是否为该类型下的默认模型
	Enabled         bool   `gorm:"column:enabled" json:"enabled"`                             // Enabled 是否启用
	VectorDimension *int   `gorm:"column:vector_dimension" json:"vector_dimension,omitempty"` // VectorDimension 向量化模型的向量维度，非 embedding 模型为空
}

// TableName 返回模型配置实体对应的数据库表名。
func (ModelConfig) TableName() string { return "ai_model_configs" }
