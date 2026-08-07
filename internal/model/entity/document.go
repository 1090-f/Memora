package entity

// Document 表示文档实体，映射到 documents 数据库表。
// SourceType 取值：manual（手工创建）/ file（文件导入）/ url（URL 导入）。
// ProcessingStatus 取值：pending/parsing/cleaning/chunking/embedding/keyword_indexing/succeeded/failed。
type Document struct {
	BaseEntity
	UserID             string  `gorm:"column:user_id" json:"user_id"`                                     // UserID 所属用户 ID
	KnowledgeBaseID    string  `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`                 // KnowledgeBaseID 所属知识库 ID
	DirectoryID        *string `gorm:"column:directory_id" json:"directory_id,omitempty"`                 // DirectoryID 所属目录 ID，为空表示未归入目录
	Title              string  `gorm:"column:title" json:"title"`                                         // Title 文档标题
	Content            *string `gorm:"column:content" json:"content,omitempty"`                           // Content 文档正文内容，可选
	SourceType         string  `gorm:"column:source_type" json:"source_type"`                             // SourceType 文档来源类型
	SourceURL          *string `gorm:"column:source_url" json:"source_url,omitempty"`                     // SourceURL 原始来源 URL，可选
	OriginalFileName   *string `gorm:"column:original_file_name" json:"original_file_name,omitempty"`     // OriginalFileName 原始文件名，可选
	FileSize           *int64  `gorm:"column:file_size" json:"file_size,omitempty"`                       // FileSize 文件大小（字节），可选
	MIMEType           *string `gorm:"column:mime_type" json:"mime_type,omitempty"`                       // MIMEType 文件 MIME 类型，可选
	FileHash           *string `gorm:"column:file_hash" json:"file_hash,omitempty"`                       // FileHash 文件内容哈希，用于去重，可选
	MinIOBucket        *string `gorm:"column:minio_bucket" json:"minio_bucket,omitempty"`                 // MinIOBucket 原始文件所在 MinIO 桶，可选
	MinIOObjectKey     *string `gorm:"column:minio_object_key" json:"minio_object_key,omitempty"`         // MinIOObjectKey 原始文件在 MinIO 中的对象键，可选
	ProcessingStatus   string  `gorm:"column:processing_status" json:"processing_status"`                 // ProcessingStatus 文档处理状态
	FailureStep        *string `gorm:"column:failure_step" json:"failure_step,omitempty"`                 // FailureStep 处理失败的步骤，可选
	FailureReason      *string `gorm:"column:failure_reason" json:"failure_reason,omitempty"`             // FailureReason 处理失败原因，可选
	ContentVersion     int     `gorm:"column:content_version" json:"content_version"`                     // ContentVersion 内容版本号，内容变更时递增
	ChunkVersion       int     `gorm:"column:chunk_version" json:"chunk_version"`                         // ChunkVersion 分块版本号，重新分块时递增
	ActiveIndexVersion *int    `gorm:"column:active_index_version" json:"active_index_version,omitempty"` // ActiveIndexVersion 当前生效的索引版本，可选
	EmbeddingModelID   *string `gorm:"column:embedding_model_id" json:"embedding_model_id,omitempty"`     // EmbeddingModelID 向量化使用的模型配置 ID，可选
	ChunkConfigHash    *string `gorm:"column:chunk_config_hash" json:"chunk_config_hash,omitempty"`       // ChunkConfigHash 分块配置哈希，用于判断配置是否变化，可选
}

// TableName 返回文档实体对应的数据库表名。
func (Document) TableName() string { return "documents" }
