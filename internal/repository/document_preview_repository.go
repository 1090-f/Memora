package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDocumentPreviewNotFound = errors.New("文档预览任务不存在")

const PreviewRenderEventType = "document.preview.render"

type documentPreviewRepository struct{ db *gorm.DB }

func NewDocumentPreviewRepository(db *gorm.DB) DocumentPreviewRepository {
	return &documentPreviewRepository{db: db}
}

func (r *documentPreviewRepository) FindByID(ctx context.Context, previewID string) (*entity.DocumentPreview, error) {
	var item entity.DocumentPreview
	err := dbFromContext(ctx, r.db).WithContext(ctx).Where("id = ?", previewID).First(&item).Error
	return mapPreviewResult(&item, err)
}

func (r *documentPreviewRepository) FindCurrent(ctx context.Context, documentID string, contentVersion int, previewType, renderHash string) (*entity.DocumentPreview, error) {
	var item entity.DocumentPreview
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("document_id = ? AND content_version = ? AND preview_type = ? AND render_hash = ?", documentID, contentVersion, previewType, renderHash).
		First(&item).Error
	return mapPreviewResult(&item, err)
}

func (r *documentPreviewRepository) EnsurePendingWithOutbox(ctx context.Context, preview *entity.DocumentPreview) (*entity.DocumentPreview, bool, error) {
	if preview == nil {
		return nil, false, fmt.Errorf("preview 不能为空")
	}
	var created bool
	err := dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{
			{Name: "document_id"}, {Name: "content_version"}, {Name: "preview_type"}, {Name: "render_hash"},
		}, DoNothing: true}).Create(preview)
		if result.Error != nil {
			return fmt.Errorf("创建预览任务失败: %w", result.Error)
		}
		created = result.RowsAffected == 1
		if !created {
			return tx.Where("document_id = ? AND content_version = ? AND preview_type = ? AND render_hash = ?",
				preview.DocumentID, preview.ContentVersion, preview.PreviewType, preview.RenderHash).First(preview).Error
		}
		return createPreviewOutbox(tx, preview.ID)
	})
	if err != nil {
		return nil, false, err
	}
	return preview, created, nil
}

func (r *documentPreviewRepository) ClaimPendingByID(ctx context.Context, previewID string) (*entity.DocumentPreview, error) {
	var claimed *entity.DocumentPreview
	err := dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item entity.DocumentPreview
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", previewID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDocumentPreviewNotFound
			}
			return err
		}
		if item.Status != "pending" {
			return nil
		}
		now := time.Now().UTC()
		if err := tx.Model(&entity.DocumentPreview{}).Where("id = ? AND status = 'pending'", previewID).Updates(map[string]any{
			"status": "processing", "attempt": gorm.Expr("attempt + 1"), "started_at": now,
			"completed_at": nil, "error_code": nil, "error_message": nil,
		}).Error; err != nil {
			return err
		}
		item.Status = "processing"
		item.Attempt++
		item.StartedAt = &now
		claimed = &item
		return nil
	})
	return claimed, err
}

func (r *documentPreviewRepository) MarkReady(ctx context.Context, previewID, objectKey, manifestKey, mediaType string, objectSize int64) error {
	now := time.Now().UTC()
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.DocumentPreview{}).
		Where("id = ? AND status = 'processing'", previewID).Updates(map[string]any{
		"status": "ready", "object_key": objectKey, "manifest_key": manifestKey,
		"media_type": mediaType, "object_size": objectSize, "completed_at": now,
		"error_code": nil, "error_message": nil,
	})
	if result.Error != nil {
		return fmt.Errorf("标记预览就绪失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrDocumentPreviewNotFound
	}
	return nil
}

func (r *documentPreviewRepository) MarkFailed(ctx context.Context, previewID, errorCode, errorMessage string) error {
	now := time.Now().UTC()
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.DocumentPreview{}).
		Where("id = ? AND status = 'processing'", previewID).Updates(map[string]any{
		"status": "failed", "error_code": errorCode, "error_message": truncatePreviewError(errorMessage), "completed_at": now,
	})
	if result.Error != nil {
		return fmt.Errorf("标记预览失败: %w", result.Error)
	}
	return nil
}

func (r *documentPreviewRepository) Requeue(ctx context.Context, previewID, errorCode, errorMessage string) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&entity.DocumentPreview{}).Where("id = ? AND status = 'processing'", previewID).Updates(map[string]any{
			"status": "pending", "started_at": nil, "error_code": errorCode, "error_message": truncatePreviewError(errorMessage),
		})
		if result.Error != nil {
			return fmt.Errorf("重新排队预览任务失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrDocumentPreviewNotFound
		}
		return createPreviewOutbox(tx, previewID)
	})
}

func (r *documentPreviewRepository) RetryFailed(ctx context.Context, userID, documentID string, contentVersion int) (int64, error) {
	var count int64
	err := dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := tx.Model(&entity.DocumentPreview{}).
			Where("user_id = ? AND document_id = ? AND content_version = ? AND status = 'failed'", userID, documentID, contentVersion).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			result := tx.Model(&entity.DocumentPreview{}).Where("id = ? AND status = 'failed'", id).Updates(map[string]any{
				"status": "pending", "attempt": 0, "started_at": nil, "completed_at": nil, "error_code": nil, "error_message": nil,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				count++
				if err := createPreviewOutbox(tx, id); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return count, err
}

func (r *documentPreviewRepository) RecoverStale(ctx context.Context, staleBefore time.Time) (int64, error) {
	var count int64
	err := dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := tx.Model(&entity.DocumentPreview{}).
			Where("status = 'processing' AND started_at IS NOT NULL AND started_at < ?", staleBefore).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			result := tx.Model(&entity.DocumentPreview{}).Where("id = ? AND status = 'processing'", id).Updates(map[string]any{
				"status": "pending", "started_at": nil, "error_code": "PREVIEW_WORKER_LEASE_EXPIRED", "error_message": "预览任务租约过期，已恢复等待处理",
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				count++
				if err := createPreviewOutbox(tx, id); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return count, err
}

func (r *documentPreviewRepository) HealthSnapshot(ctx context.Context) (PreviewTaskHealthSnapshot, error) {
	var row struct {
		Pending       int64
		Running       int64
		Failed        int64
		Retried       int64
		OldestPending *time.Time
	}
	err := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.DocumentPreview{}).
		Select(`
			COUNT(*) FILTER (WHERE status = 'pending') AS pending,
			COUNT(*) FILTER (WHERE status = 'processing') AS running,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			COUNT(*) FILTER (WHERE attempt > 1) AS retried,
			MIN(created_at) FILTER (WHERE status = 'pending') AS oldest_pending
		`).Scan(&row).Error
	if err != nil {
		return PreviewTaskHealthSnapshot{}, fmt.Errorf("查询预览任务健康状态失败: %w", err)
	}
	snapshot := PreviewTaskHealthSnapshot{Pending: row.Pending, Running: row.Running, Failed: row.Failed, Retried: row.Retried}
	if row.OldestPending != nil {
		snapshot.OldestPendingAgeSeconds = max(0, int64(time.Since(*row.OldestPending).Seconds()))
	}
	return snapshot, nil
}

func createPreviewOutbox(tx *gorm.DB, previewID string) error {
	payload, err := marshalOutboxPayload(tx.Statement.Context, map[string]string{"preview_id": previewID})
	if err != nil {
		return err
	}
	event := &entity.TaskOutbox{EventType: PreviewRenderEventType, AggregateID: previewID, Payload: string(payload)}
	if err := tx.Create(event).Error; err != nil {
		return fmt.Errorf("创建预览 Outbox 事件失败: %w", err)
	}
	return nil
}

func mapPreviewResult(item *entity.DocumentPreview, err error) (*entity.DocumentPreview, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDocumentPreviewNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func truncatePreviewError(value string) string {
	const max = 1000
	if len(value) > max {
		return value[:max] + "..."
	}
	return value
}
