package entity

import "time"

// DocumentPreview 表示一个需要后台生成的视觉预览派生产物。
// 直接预览类型（text/markdown/image/原始 PDF）不落表。
type DocumentPreview struct {
	ID              string     `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID          string     `gorm:"column:user_id" json:"user_id"`
	DocumentID      string     `gorm:"column:document_id" json:"document_id"`
	ContentVersion  int        `gorm:"column:content_version" json:"content_version"`
	PreviewType     string     `gorm:"column:preview_type" json:"preview_type"`
	Status          string     `gorm:"column:status" json:"status"`
	RenderHash      string     `gorm:"column:render_hash" json:"render_hash"`
	Renderer        string     `gorm:"column:renderer" json:"renderer"`
	RendererVersion string     `gorm:"column:renderer_version" json:"renderer_version"`
	ObjectKey       *string    `gorm:"column:object_key" json:"object_key,omitempty"`
	ManifestKey     *string    `gorm:"column:manifest_key" json:"manifest_key,omitempty"`
	MediaType       *string    `gorm:"column:media_type" json:"media_type,omitempty"`
	ObjectSize      *int64     `gorm:"column:object_size" json:"object_size,omitempty"`
	Attempt         int        `gorm:"column:attempt" json:"attempt"`
	ErrorCode       *string    `gorm:"column:error_code" json:"error_code,omitempty"`
	ErrorMessage    *string    `gorm:"column:error_message" json:"error_message,omitempty"`
	StartedAt       *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	CompletedAt     *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (DocumentPreview) TableName() string { return "document_previews" }
