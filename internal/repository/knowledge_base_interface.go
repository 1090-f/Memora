package repository

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
)

// KnowledgeBaseImportTrendPoint 表示某个 UTC 自然日创建的文档数量。
type KnowledgeBaseImportTrendPoint struct {
	Day   time.Time
	Count int64
}

// KnowledgeBaseDashboardSnapshot 是知识库仪表盘所需的聚合读模型。
type KnowledgeBaseDashboardSnapshot struct {
	DocumentTotal             int64
	IndexedTotal              int64
	ProcessingTotal           int64
	FailedTotal               int64
	HighestActiveIndexVersion int
	ImportTrend               []KnowledgeBaseImportTrendPoint
	RecentTasks               []*entity.ImportTask
}

// KnowledgeBaseRepository 定义知识库数据访问接口。
// 所有查询必须包含 user_id 与软删除过滤，禁止先查后验。
type KnowledgeBaseRepository interface {
	// Create 创建知识库并返回带 ID 的实体。
	Create(ctx context.Context, kb *entity.KnowledgeBase) error
	// FindByID 按 ID 与用户查询知识库。
	FindByID(ctx context.Context, userID, kbID string) (*entity.KnowledgeBase, error)
	// List 分页查询用户知识库，keyword 匹配名称。
	List(ctx context.Context, userID string, page, pageSize int, keyword string) ([]*entity.KnowledgeBase, int64, error)
	// Update 按字段映射更新知识库，返回更新后的实体。
	Update(ctx context.Context, userID, kbID string, updates map[string]any) (*entity.KnowledgeBase, error)
	// SoftDelete 软删除知识库。
	SoftDelete(ctx context.Context, userID, kbID string) error
	// CountDocuments 统计知识库内未删除文档数量（列表项 document_count 使用）。
	CountDocuments(ctx context.Context, userID, kbID string) (int64, error)
	// GetDashboardSnapshot 聚合知识库健康度、导入趋势和近期活动所需数据。
	GetDashboardSnapshot(ctx context.Context, userID, kbID string, since time.Time, recentLimit int) (*KnowledgeBaseDashboardSnapshot, error)
}
