package entity

import "time"

// DocumentVector 表示文档向量实体，映射到 document_vectors 数据库表。
// embedding 列使用 pgvector 的 vector 类型，维度由 embedding_dim 记录。
// Status 取值：pending（待生成）/ ready（可用）/ failed（生成失败）。
type DocumentVector struct {
	ID               string    `gorm:"column:id" json:"id"`                                 // ID 主键（UUID）
	UserID           string    `gorm:"column:user_id" json:"user_id"`                       // UserID 所属用户 ID
	KnowledgeBaseID  string    `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`   // KnowledgeBaseID 所属知识库 ID
	DocumentID       string    `gorm:"column:document_id" json:"document_id"`               // DocumentID 所属文档 ID
	ChunkID          string    `gorm:"column:chunk_id" json:"chunk_id"`                     // ChunkID 对应的文档分块 ID
	IndexVersion     int       `gorm:"column:index_version" json:"index_version"`           // IndexVersion 索引版本号，用于区分多套索引
	EmbeddingModelID string    `gorm:"column:embedding_model_id" json:"embedding_model_id"` // EmbeddingModelID 生成该向量使用的模型配置 ID
	EmbeddingDim     int       `gorm:"column:embedding_dim" json:"embedding_dim"`           // EmbeddingDim 向量的维度
	Embedding        []float32 `gorm:"column:embedding" json:"-"`                           // Embedding 向量数据，不参与 JSON 序列化
	Status           string    `gorm:"column:status" json:"status"`                         // Status 向量状态：pending/ready/failed
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`                 // CreatedAt 创建时间
}

// TableName 返回文档向量实体对应的数据库表名。
func (DocumentVector) TableName() string { return "document_vectors" }
