package entity

import (
	"time"

	"gorm.io/datatypes"
)

// TraceSpan 是经过白名单裁剪的 OpenTelemetry Span，用于应用内链路浏览。
type TraceSpan struct {
	TraceID              string         `gorm:"column:trace_id;type:varchar(32);primaryKey" json:"trace_id"`
	SpanID               string         `gorm:"column:span_id;type:varchar(16);primaryKey" json:"span_id"`
	ParentSpanID         *string        `gorm:"column:parent_span_id;type:varchar(16)" json:"parent_span_id,omitempty"`
	Name                 string         `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Kind                 string         `gorm:"column:kind;type:varchar(24);not null" json:"kind"`
	StatusCode           string         `gorm:"column:status_code;type:varchar(16);not null" json:"status_code"`
	StatusMessage        *string        `gorm:"column:status_message;type:varchar(500)" json:"status_message,omitempty"`
	StartedAt            time.Time      `gorm:"column:started_at;not null" json:"started_at"`
	EndedAt              time.Time      `gorm:"column:ended_at;not null" json:"ended_at"`
	DurationMS           int64          `gorm:"column:duration_ms;not null" json:"duration_ms"`
	Attributes           datatypes.JSON `gorm:"column:attributes;type:jsonb;not null" json:"attributes"`
	Events               datatypes.JSON `gorm:"column:events;type:jsonb;not null" json:"events"`
	ServiceName          *string        `gorm:"column:service_name;type:varchar(128)" json:"service_name,omitempty"`
	InstrumentationScope *string        `gorm:"column:instrumentation_scope;type:varchar(255)" json:"instrumentation_scope,omitempty"`
	CreatedAt            time.Time      `gorm:"column:created_at;not null" json:"created_at"`
}

func (TraceSpan) TableName() string { return "trace_spans" }
