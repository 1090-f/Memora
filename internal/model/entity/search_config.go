package entity

import "time"

// SearchConfig 表示知识库检索配置实体，映射到 search_configs 数据库表。
// 每个知识库对应一条，字段值与 internal/contracts.SearchConfig 对齐。
// 该表无 user_id/deleted_at 列，归属通过 knowledge_base_id 关联的知识库推导。
type SearchConfig struct {
	ID                   string    `gorm:"column:id" json:"id"`
	KnowledgeBaseID      string    `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`
	KeywordTopK          int       `gorm:"column:keyword_top_k" json:"keyword_top_k"`
	VectorTopK           int       `gorm:"column:vector_top_k" json:"vector_top_k"`
	RRFK                 int       `gorm:"column:rrf_k" json:"rrf_k"`
	RRFTopK              int       `gorm:"column:rrf_top_k" json:"rrf_top_k"`
	RerankerTopK         int       `gorm:"column:reranker_top_k" json:"reranker_top_k"`
	RerankerThreshold    *float64  `gorm:"column:reranker_threshold" json:"reranker_threshold,omitempty"`
	MinimumEffectiveRate int       `gorm:"column:minimum_effective_results" json:"minimum_effective_results"`
	RerankerModelID      *string   `gorm:"column:reranker_model_id" json:"reranker_model_id,omitempty"`
	CreatedAt            time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 返回检索配置实体对应的数据库表名。
func (SearchConfig) TableName() string { return "search_configs" }
