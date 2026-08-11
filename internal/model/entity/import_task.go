package entity

import "time"

// ImportTask 表示导入任务实体，映射到 import_tasks 数据库表。
// 任务包 02 只使用创建与对象信息更新；状态机与 Worker 在任务包 03 实现。
// SourceType 取值：file（文件导入）/ url（URL 导入）。
// DuplicatePolicy 取值：create_new（创建新文档）/ skip（跳过重复）。
// Status 取值：pending/running/succeeded/failed/skipped。
type ImportTask struct {
	ID                string     `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()" json:"id"` // ID 主键（UUID）
	UserID            string     `gorm:"column:user_id" json:"user_id"`                                      // UserID 所属用户 ID
	KnowledgeBaseID   string     `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`                  // KnowledgeBaseID 所属知识库 ID
	TargetDirectoryID *string    `gorm:"column:target_directory_id" json:"target_directory_id,omitempty"`    // TargetDirectoryID 目标目录 ID，为空表示默认目录
	SourceType        string     `gorm:"column:source_type" json:"source_type"`                              // SourceType 导入来源类型：file/url
	FileName          *string    `gorm:"column:file_name" json:"file_name,omitempty"`                        // FileName 源文件名，可选
	FileSize          *int64     `gorm:"column:file_size" json:"file_size,omitempty"`                        // FileSize 源文件大小（字节），可选
	MIMEType          *string    `gorm:"column:mime_type" json:"mime_type,omitempty"`                        // MIMEType 源文件 MIME 类型，可选
	SourceURL         *string    `gorm:"column:source_url" json:"source_url,omitempty"`                      // SourceURL 导入源 URL，可选
	SourceHash        *string    `gorm:"column:source_hash" json:"source_hash,omitempty"`                    // SourceHash 源内容哈希，用于重复检测，可选
	MinIOBucket       *string    `gorm:"column:minio_bucket" json:"minio_bucket,omitempty"`                  // MinIOBucket 源文件所在 MinIO 桶，可选
	MinIOObjectKey    *string    `gorm:"column:minio_object_key" json:"minio_object_key,omitempty"`          // MinIOObjectKey 源文件在 MinIO 中的对象键，可选
	DuplicatePolicy   string     `gorm:"column:duplicate_policy" json:"duplicate_policy"`                    // DuplicatePolicy 重复处理策略：create_new/skip
	Status            string     `gorm:"column:status" json:"status"`                                        // Status 任务状态：pending/running/succeeded/failed/skipped
	Attempt           int        `gorm:"column:attempt" json:"attempt"`                                      // Attempt 已执行次数
	CurrentStep       *string    `gorm:"column:current_step" json:"current_step,omitempty"`                  // CurrentStep 当前执行的步骤，可选
	FailureReason     *string    `gorm:"column:failure_reason" json:"failure_reason,omitempty"`              // FailureReason 失败原因，可选
	DocumentID        *string    `gorm:"column:document_id" json:"document_id,omitempty"`                    // DocumentID 导入成功后生成的文档 ID，可选
	// Attachments 是 zip 导入的附件映射（zip 内相对路径 → MinIO object key），可选。
	Attachments StringMap `gorm:"column:attachments;type:jsonb" json:"attachments,omitempty"`
	CreatedAt         time.Time  `gorm:"column:created_at" json:"created_at"`                                // CreatedAt 创建时间
	StartedAt         *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`                      // StartedAt 开始处理时间，可选
	CompletedAt       *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`                  // CompletedAt 完成时间，可选
}

// TableName 返回导入任务实体对应的数据库表名。
func (ImportTask) TableName() string { return "import_tasks" }

// TaskOutbox 是可靠发布到 Redis Stream 的事务事件。
type TaskOutbox struct {
	ID          string     `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	EventType   string     `gorm:"column:event_type" json:"event_type"`
	AggregateID string     `gorm:"column:aggregate_id;type:uuid" json:"aggregate_id"`
	Payload     string     `gorm:"column:payload;type:jsonb" json:"payload"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	PublishedAt *time.Time `gorm:"column:published_at" json:"published_at,omitempty"`
}

func (TaskOutbox) TableName() string { return "task_outbox" }
