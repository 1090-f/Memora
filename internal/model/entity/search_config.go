package entity

import "time"

// SearchConfig 表示知识库检索配置实体，映射到 search_configs 数据库表。
// 每个知识库对应一条，字段值与 internal/contracts.SearchConfig 对齐。
// 该表无 user_id/deleted_at 列，归属通过 knowledge_base_id 关联的知识库推导。
type SearchConfig struct {
	ID                   string    `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()" json:"id"` // ID 主键（UUID）
	KnowledgeBaseID      string    `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`                  // KnowledgeBaseID 关联的知识库 ID
	KeywordTopK          int       `gorm:"column:keyword_top_k" json:"keyword_top_k"`                          // KeywordTopK 关键词检索返回的候选条数
	VectorTopK           int       `gorm:"column:vector_top_k" json:"vector_top_k"`                            // VectorTopK 向量检索返回的候选条数
	RRFK                 int       `gorm:"column:rrf_k" json:"rrf_k"`                                          // RRFK 混合检索融合算法（RRF）的常数 k
	RRFTopK              int       `gorm:"column:rrf_top_k" json:"rrf_top_k"`                                  // RRFTopK RRF 融合后保留的候选条数
	RerankerTopK         int       `gorm:"column:reranker_top_k" json:"reranker_top_k"`                        // RerankerTopK 重排序后返回的条数
	RerankerThreshold    *float64  `gorm:"column:reranker_threshold" json:"reranker_threshold,omitempty"`      // RerankerThreshold 重排序分数阈值，低于该分数的结果被过滤，可选
	MinimumEffectiveRate int       `gorm:"column:minimum_effective_results" json:"minimum_effective_results"`  // MinimumEffectiveRate 最低有效结果数，防止检索结果过少
	MinVectorScore       float64   `gorm:"column:min_vector_score" json:"min_vector_score"`                    // MinVectorScore 向量相似度最低阈值（0~1），0 表示不启用过滤
	AmbiguousScore       float64   `gorm:"column:ambiguous_score" json:"ambiguous_score"`                      // AmbiguousScore 最高向量相似度低于该值时判为 ambiguous（0~1），0 表示不启用
	RerankerModelID      *string   `gorm:"column:reranker_model_id" json:"reranker_model_id,omitempty"`        // RerankerModelID 重排序模型配置 ID，可选
	CreatedAt            time.Time `gorm:"column:created_at" json:"created_at"`                                // CreatedAt 创建时间
	UpdatedAt            time.Time `gorm:"column:updated_at" json:"updated_at"`                                // UpdatedAt 更新时间
}

// TableName 返回检索配置实体对应的数据库表名。
func (SearchConfig) TableName() string { return "search_configs" }
