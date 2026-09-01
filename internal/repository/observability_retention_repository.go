package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

// ObservabilityRetentionRepository 按统一保留期清理可重建的阶段/运行事件。
type ObservabilityRetentionRepository interface {
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type observabilityRetentionRepository struct{ db *gorm.DB }

func NewObservabilityRetentionRepository(db *gorm.DB) ObservabilityRetentionRepository {
	return &observabilityRetentionRepository{db: db}
}

func (r *observabilityRetentionRepository) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var deleted int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("timestamp < ?", cutoff).Delete(&entity.AgentEvent{})
		if result.Error != nil {
			return fmt.Errorf("清理 Agent 事件失败: %w", result.Error)
		}
		deleted += result.RowsAffected
		result = tx.Where("created_at < ?", cutoff).Delete(&entity.DocumentProcessingEvent{})
		if result.Error != nil {
			return fmt.Errorf("清理文档阶段事件失败: %w", result.Error)
		}
		deleted += result.RowsAffected
		return nil
	})
	return deleted, err
}
