package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/1090-f/Memora/pkg/audit"
	"gorm.io/gorm"
)

// auditLog 是审计日志表的 GORM 映射。
type auditLog struct {
	ID        int64     `gorm:"column:id"`
	Action    string    `gorm:"column:action"`
	ActorID   string    `gorm:"column:actor_id"`
	Resource  string    `gorm:"column:resource"`
	RequestID string    `gorm:"column:request_id"`
	TraceID   string    `gorm:"column:trace_id"`
	Outcome   string    `gorm:"column:outcome"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// TableName 返回审计日志表的表名。
func (auditLog) TableName() string { return "audit_logs" }

// auditRepository 是 audit.Store 接口的 GORM 实现。
type auditRepository struct{ db *gorm.DB }

// NewAuditRepository 创建一个新的审计日志仓储实例。
func NewAuditRepository(db *gorm.DB) audit.Store { return &auditRepository{db: db} }

// Insert 插入一条审计日志记录。
func (r *auditRepository) Insert(ctx context.Context, entry *audit.AuditEntry) error {
	record := &auditLog{
		Action: entry.Action, ActorID: entry.ActorID, Resource: entry.Resource,
		RequestID: entry.RequestID, TraceID: entry.TraceID, Outcome: entry.Outcome,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("写入审计日志失败: %w", err)
	}
	return nil
}
