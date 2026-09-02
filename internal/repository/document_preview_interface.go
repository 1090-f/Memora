package repository

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
)

// DocumentPreviewRepository 管理 Preview Artifact 元数据与异步任务状态。
type DocumentPreviewRepository interface {
	FindByID(ctx context.Context, previewID string) (*entity.DocumentPreview, error)
	FindCurrent(ctx context.Context, documentID string, contentVersion int, previewType, renderHash string) (*entity.DocumentPreview, error)
	// EnsurePendingWithOutbox 幂等创建 pending 记录，并在同一事务写入 Outbox。
	EnsurePendingWithOutbox(ctx context.Context, preview *entity.DocumentPreview) (*entity.DocumentPreview, bool, error)
	ClaimPendingByID(ctx context.Context, previewID string) (*entity.DocumentPreview, error)
	MarkReady(ctx context.Context, previewID, objectKey, manifestKey, mediaType string, objectSize int64) error
	MarkFailed(ctx context.Context, previewID, errorCode, errorMessage string) error
	Requeue(ctx context.Context, previewID, errorCode, errorMessage string) error
	RetryFailed(ctx context.Context, userID, documentID string, contentVersion int) (int64, error)
	RecoverStale(ctx context.Context, staleBefore time.Time) (int64, error)
	HealthSnapshot(ctx context.Context) (PreviewTaskHealthSnapshot, error)
}

type PreviewTaskHealthSnapshot struct {
	Pending                 int64
	Running                 int64
	Failed                  int64
	Retried                 int64
	OldestPendingAgeSeconds int64
}
