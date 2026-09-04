package repository

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

type DocumentProcessingEventRepository interface {
	Append(ctx context.Context, event *entity.DocumentProcessingEvent) error
	ListByDocument(ctx context.Context, userID, documentID string) ([]entity.DocumentProcessingEvent, error)
}

type documentProcessingEventRepository struct{ db *gorm.DB }

func NewDocumentProcessingEventRepository(db *gorm.DB) DocumentProcessingEventRepository {
	return &documentProcessingEventRepository{db: db}
}

func (r *documentProcessingEventRepository) Append(ctx context.Context, event *entity.DocumentProcessingEvent) error {
	if err := dbFromContext(ctx, r.db).WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("写入文档处理阶段事件失败: %w", err)
	}
	return nil
}

func (r *documentProcessingEventRepository) ListByDocument(ctx context.Context, userID, documentID string) ([]entity.DocumentProcessingEvent, error) {
	var events []entity.DocumentProcessingEvent
	err := dbFromContext(ctx, r.db).WithContext(ctx).Table("document_processing_events AS e").
		Select("e.*").Joins("JOIN documents d ON d.id = e.document_id").
		Where("e.document_id = ? AND d.user_id = ? AND d.deleted_at IS NULL", documentID, userID).
		Order("e.created_at ASC").Scan(&events).Error
	if err != nil {
		return nil, fmt.Errorf("查询文档处理阶段事件失败: %w", err)
	}
	return events, nil
}
