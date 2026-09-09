package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"gorm.io/gorm"
)

type taskOutboxRepository struct{ db *gorm.DB }

func marshalOutboxPayload(ctx context.Context, fields map[string]string) ([]byte, error) {
	carrier := propagation.MapCarrier(fields)
	if ctx != nil {
		otel.GetTextMapPropagator().Inject(ctx, carrier)
	}
	return json.Marshal(fields)
}

func NewTaskOutboxRepository(db *gorm.DB) TaskOutboxRepository {
	return &taskOutboxRepository{db: db}
}

func (r *taskOutboxRepository) ListUnpublished(ctx context.Context, limit int) ([]*entity.TaskOutbox, error) {
	var events []*entity.TaskOutbox
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("published_at IS NULL").Order("created_at ASC").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("查询未发布 Outbox 事件失败: %w", err)
	}
	return events, nil
}

func (r *taskOutboxRepository) CountUnpublished(ctx context.Context) (int64, error) {
	var count int64
	if err := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.TaskOutbox{}).
		Where("published_at IS NULL").Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计未发布 Outbox 事件失败: %w", err)
	}
	return count, nil
}

func (r *taskOutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.TaskOutbox{}).
		Where("id = ? AND published_at IS NULL", eventID).Update("published_at", time.Now().UTC())
	if result.Error != nil {
		return fmt.Errorf("标记 Outbox 事件已发布失败: %w", result.Error)
	}
	return nil
}
