package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	// ErrKnowledgeBaseNotFound 表示未找到指定知识库。
	ErrKnowledgeBaseNotFound = errors.New("知识库不存在")
	// ErrKnowledgeBaseConflict 表示知识库与已有知识库冲突。
	ErrKnowledgeBaseConflict = errors.New("知识库与现有知识库冲突")
)

// knowledgeBaseRepository 是 KnowledgeBaseRepository 接口的 GORM 实现。
type knowledgeBaseRepository struct{ db *gorm.DB }

// NewKnowledgeBaseRepository 创建一个新的知识库仓储实例。
func NewKnowledgeBaseRepository(db *gorm.DB) KnowledgeBaseRepository {
	return &knowledgeBaseRepository{db: db}
}

// Create 创建知识库。
func (r *knowledgeBaseRepository) Create(ctx context.Context, kb *entity.KnowledgeBase) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).Create(kb).Error
	if err != nil {
		// 唯一约束冲突说明名称等唯一字段重复，转换为业务错误。
		if isUniqueViolation(err) {
			return ErrKnowledgeBaseConflict
		}
		return fmt.Errorf("创建知识库失败: %w", err)
	}
	return nil
}

// FindByID 按 ID 与用户查询知识库。
func (r *knowledgeBaseRepository) FindByID(ctx context.Context, userID, kbID string) (*entity.KnowledgeBase, error) {
	var kb entity.KnowledgeBase
	// WHERE 带 user_id 与 deleted_at，防止越权访问他人知识库或命中已删除记录。
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", kbID, userID).
		First(&kb).Error
	return mapKnowledgeBaseResult(&kb, err)
}

// List 分页查询用户知识库，keyword 匹配名称。
func (r *knowledgeBaseRepository) List(ctx context.Context, userID string, page, pageSize int, keyword string) ([]*entity.KnowledgeBase, int64, error) {
	db := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.KnowledgeBase{})
	db = db.Where("user_id = ? AND deleted_at IS NULL", userID)
	// 名称模糊匹配：两侧都转小写，保证大小写不敏感。
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		db = db.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	// 先统计总数再取当前页，保证分页元数据完整。
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计知识库失败: %w", err)
	}
	var items []*entity.KnowledgeBase
	if err := db.Order("updated_at DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("查询知识库列表失败: %w", err)
	}
	return items, total, nil
}

// Update 按字段映射更新知识库。
func (r *knowledgeBaseRepository) Update(ctx context.Context, userID, kbID string, updates map[string]any) (*entity.KnowledgeBase, error) {
	if len(updates) == 0 {
		return nil, ErrKnowledgeBaseNotFound
	}
	updates["updated_at"] = time.Now().UTC()
	// 更新语句同样带 user_id 与软删除过滤，并兼容唯一约束冲突转为业务错误。
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.KnowledgeBase{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", kbID, userID).Updates(updates)
	if result.Error != nil {
		if isUniqueViolation(result.Error) {
			return nil, ErrKnowledgeBaseConflict
		}
		return nil, fmt.Errorf("更新知识库失败: %w", result.Error)
	}
	// RowsAffected 为 0 表示目标不存在或已删除，按未找到处理。
	if result.RowsAffected == 0 {
		return nil, ErrKnowledgeBaseNotFound
	}
	return r.FindByID(ctx, userID, kbID)
}

// SoftDelete 软删除知识库。
func (r *knowledgeBaseRepository) SoftDelete(ctx context.Context, userID, kbID string) error {
	// WHERE 带 deleted_at IS NULL 使删除操作幂等：已删除或不存在时 RowsAffected 为 0 报未找到。
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.KnowledgeBase{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", kbID, userID).
		Update("deleted_at", time.Now().UTC())
	if result.Error != nil {
		return fmt.Errorf("删除知识库失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrKnowledgeBaseNotFound
	}
	return nil
}

// CountDocuments 统计知识库内未删除文档数量。
func (r *knowledgeBaseRepository) CountDocuments(ctx context.Context, userID, kbID string) (int64, error) {
	var count int64
	err := dbFromContext(ctx, r.db).WithContext(ctx).Table("documents").
		Where("user_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL", userID, kbID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("统计文档数量失败: %w", err)
	}
	return count, nil
}

// GetDashboardSnapshot 使用聚合查询构造知识库仪表盘读模型，避免前端为每个指标分别请求文档列表。
func (r *knowledgeBaseRepository) GetDashboardSnapshot(ctx context.Context, userID, kbID string, since time.Time, recentLimit int) (*KnowledgeBaseDashboardSnapshot, error) {
	db := dbFromContext(ctx, r.db).WithContext(ctx)
	var counts struct {
		DocumentTotal             int64 `gorm:"column:document_total"`
		IndexedTotal              int64 `gorm:"column:indexed_total"`
		ProcessingTotal           int64 `gorm:"column:processing_total"`
		FailedTotal               int64 `gorm:"column:failed_total"`
		HighestActiveIndexVersion int   `gorm:"column:highest_active_index_version"`
	}
	if err := db.Raw(`
		SELECT
			COUNT(*) AS document_total,
			COUNT(*) FILTER (WHERE active_index_version IS NOT NULL) AS indexed_total,
			COUNT(*) FILTER (WHERE processing_status IN ('pending', 'parsing', 'cleaning', 'chunking', 'embedding', 'keyword_indexing')) AS processing_total,
			COUNT(*) FILTER (WHERE processing_status = 'failed') AS failed_total,
			COALESCE(MAX(active_index_version), 0) AS highest_active_index_version
		FROM documents
		WHERE user_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL`, userID, kbID).
		Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("聚合知识库文档指标失败: %w", err)
	}

	var trendRows []struct {
		Day   time.Time `gorm:"column:day"`
		Count int64     `gorm:"column:count"`
	}
	if err := db.Model(&entity.ImportTask{}).
		Select("DATE(created_at) AS day, COUNT(*) AS count").
		Where("user_id = ? AND knowledge_base_id = ? AND created_at >= ?", userID, kbID, since).
		Group("DATE(created_at)").Order("day ASC").Scan(&trendRows).Error; err != nil {
		return nil, fmt.Errorf("聚合知识库导入趋势失败: %w", err)
	}

	if recentLimit <= 0 {
		recentLimit = 4
	}
	var recentTasks []*entity.ImportTask
	if err := db.Where("user_id = ? AND knowledge_base_id = ?", userID, kbID).
		Order("COALESCE(completed_at, started_at, created_at) DESC").Limit(recentLimit).
		Find(&recentTasks).Error; err != nil {
		return nil, fmt.Errorf("查询知识库近期活动失败: %w", err)
	}

	trend := make([]KnowledgeBaseImportTrendPoint, 0, len(trendRows))
	for _, row := range trendRows {
		trend = append(trend, KnowledgeBaseImportTrendPoint{Day: row.Day, Count: row.Count})
	}
	return &KnowledgeBaseDashboardSnapshot{
		DocumentTotal: counts.DocumentTotal, IndexedTotal: counts.IndexedTotal,
		ProcessingTotal: counts.ProcessingTotal, FailedTotal: counts.FailedTotal,
		HighestActiveIndexVersion: counts.HighestActiveIndexVersion,
		ImportTrend:               trend, RecentTasks: recentTasks,
	}, nil
}

func mapKnowledgeBaseResult(kb *entity.KnowledgeBase, err error) (*entity.KnowledgeBase, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrKnowledgeBaseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询知识库失败: %w", err)
	}
	return kb, nil
}

// isUniqueViolation 判断是否为 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
