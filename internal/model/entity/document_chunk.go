package entity

import "time"

// DocumentChunk 表示文档分块实体，映射到 document_chunks 数据库表。
type DocumentChunk struct {
	ID              string    `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()" json:"id"` // ID 主键（UUID）
	UserID          string    `gorm:"column:user_id" json:"user_id"`                                      // UserID 所属用户 ID
	KnowledgeBaseID string    `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`                  // KnowledgeBaseID 所属知识库 ID
	DocumentID      string    `gorm:"column:document_id" json:"document_id"`                              // DocumentID 所属文档 ID
	ChunkNo         int       `gorm:"column:chunk_no" json:"chunk_no"`                                    // ChunkNo 分块在文档内的序号（从 0 开始）
	Content         string    `gorm:"column:content" json:"content"`                                      // Content 分块正文内容
	CharCount       int       `gorm:"column:char_count" json:"char_count"`                                // CharCount 分块字符数
	TokenCount      int       `gorm:"column:token_count" json:"token_count"`                              // TokenCount 分块 token 数
	ContextTitle    *string   `gorm:"column:context_title" json:"context_title,omitempty"`                // ContextTitle 分块上下文标题（如所在章节标题），可选
	HeadingPath     []byte    `gorm:"column:heading_path" json:"heading_path,omitempty"`                  // HeadingPath 分块的标题路径（JSON 编码），可选
	SourceLocation  []byte    `gorm:"column:source_location" json:"source_location,omitempty"`            // SourceLocation 分块在原文中的位置信息（JSON 编码），可选
	ContentVersion  int       `gorm:"column:content_version" json:"content_version"`                      // ContentVersion 内容版本号，与文档对齐
	ChunkVersion    int       `gorm:"column:chunk_version" json:"chunk_version"`                          // ChunkVersion 分块版本号，与文档对齐
	IndexVersion    int       `gorm:"column:index_version" json:"index_version"`                          // IndexVersion 索引版本号，用于区分多套索引
	ChunkConfigHash string    `gorm:"column:chunk_config_hash" json:"chunk_config_hash"`                  // ChunkConfigHash 生成该分块的分块配置哈希
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`                                // CreatedAt 创建时间
}

// TableName 返回文档分块实体对应的数据库表名。
func (DocumentChunk) TableName() string { return "document_chunks" }
