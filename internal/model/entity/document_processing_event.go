package entity

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// DocumentProcessingEvent 是追加式文档阶段记录；写入失败不得阻断主处理流程。
type DocumentProcessingEvent struct {
	ID              string         `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	KnowledgeBaseID string         `gorm:"column:knowledge_base_id;type:uuid;not null"`
	DocumentID      string         `gorm:"column:document_id;type:uuid;not null"`
	TaskID          *string        `gorm:"column:task_id;type:uuid"`
	Stage           string         `gorm:"column:stage;type:varchar(40);not null"`
	Status          string         `gorm:"column:status;type:varchar(20);not null"`
	StartedAt       *time.Time     `gorm:"column:started_at"`
	EndedAt         *time.Time     `gorm:"column:ended_at"`
	DurationMS      *int64         `gorm:"column:duration_ms"`
	Attempt         int            `gorm:"column:attempt;not null"`
	ErrorCode       *string        `gorm:"column:error_code;type:varchar(64)"`
	ErrorMessage    *string        `gorm:"column:error_message"`
	Metadata        datatypes.JSON `gorm:"column:metadata;type:jsonb"`
	TraceID         *string        `gorm:"column:trace_id;type:varchar(32)"`
	RequestID       *string        `gorm:"column:request_id;type:varchar(128)"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
}

func (DocumentProcessingEvent) TableName() string { return "document_processing_events" }

func (e *DocumentProcessingEvent) BeforeCreate(_ *gorm.DB) error {
	if len(e.Metadata) == 0 {
		e.Metadata = datatypes.JSON([]byte(`{}`))
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	return nil
}
