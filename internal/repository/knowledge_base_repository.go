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
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", kbID, userID).
		First(&kb).Error
	return mapKnowledgeBaseResult(&kb, err)
}

// List 分页查询用户知识库，keyword 匹配名称。
func (r *knowledgeBaseRepository) List(ctx context.Context, userID string, page, pageSize int, keyword string) ([]*entity.KnowledgeBase, int64, error) {
	db := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.KnowledgeBase{})
	db = db.Where("user_id = ? AND deleted_at IS NULL", userID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		db = db.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
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
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.KnowledgeBase{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", kbID, userID).Updates(updates)
	if result.Error != nil {
		if isUniqueViolation(result.Error) {
			return nil, ErrKnowledgeBaseConflict
		}
		return nil, fmt.Errorf("更新知识库失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrKnowledgeBaseNotFound
	}
	return r.FindByID(ctx, userID, kbID)
}

// SoftDelete 软删除知识库。
func (r *knowledgeBaseRepository) SoftDelete(ctx context.Context, userID, kbID string) error {
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

func mapKnowledgeBaseResult(kb *entity.KnowledgeBase, err error) (*entity.KnowledgeBase, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrKnowledgeBaseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询知识库失败: %w", err)
	}
	return kb, nil
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
