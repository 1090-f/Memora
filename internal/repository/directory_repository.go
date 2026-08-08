package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

var (
	// ErrDirectoryNotFound 表示未找到指定目录。
	ErrDirectoryNotFound = errors.New("目录不存在")
	// ErrDirectoryConflict 表示目录与已有目录冲突。
	ErrDirectoryConflict = errors.New("目录与现有目录冲突")
)

// directoryRepository 是 DocumentDirectoryRepository 接口的 GORM 实现。
type directoryRepository struct{ db *gorm.DB }

// NewDocumentDirectoryRepository 创建一个新的文档目录仓储实例。
func NewDocumentDirectoryRepository(db *gorm.DB) DocumentDirectoryRepository {
	return &directoryRepository{db: db}
}

// Create 创建目录。
func (r *directoryRepository) Create(ctx context.Context, dir *entity.DocumentDirectory) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).Create(dir).Error
	if err != nil {
		// 唯一约束冲突说明与现有目录冲突，转换为业务错误。
		if isUniqueViolation(err) {
			return ErrDirectoryConflict
		}
		return fmt.Errorf("创建目录失败: %w", err)
	}
	return nil
}

// FindByID 按 ID 与用户查询目录。
func (r *directoryRepository) FindByID(ctx context.Context, userID, dirID string) (*entity.DocumentDirectory, error) {
	var dir entity.DocumentDirectory
	// WHERE 强制带 user_id 与 deleted_at，防止越权访问他人目录或命中已删除记录。
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", dirID, userID).
		First(&dir).Error
	return mapDirectoryResult(&dir, err)
}

// FindByIDInKB 按 ID、用户与知识库查询目录。
func (r *directoryRepository) FindByIDInKB(ctx context.Context, userID, kbID, dirID string) (*entity.DocumentDirectory, error) {
	var dir entity.DocumentDirectory
	// 归属过滤同时包含 user_id 与 knowledge_base_id，确保目录确实属于该知识库。
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL", dirID, userID, kbID).
		First(&dir).Error
	return mapDirectoryResult(&dir, err)
}

// ListByKB 列出知识库下全部未删除目录。
func (r *directoryRepository) ListByKB(ctx context.Context, userID, kbID string) ([]*entity.DocumentDirectory, error) {
	var items []*entity.DocumentDirectory
	// 按 sort_order 再按创建时间排序，保证同一层级目录展示顺序稳定。
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL", userID, kbID).
		Order("sort_order ASC, created_at ASC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("查询目录列表失败: %w", err)
	}
	return items, nil
}

// CountChildren 统计指定目录的直接子目录数量。
func (r *directoryRepository) CountChildren(ctx context.Context, userID, kbID, parentID string) (int64, error) {
	var count int64
	// 通过 parent_id 精确匹配直接子目录（不递归），用于校验目录树结构的数量约束。
	err := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.DocumentDirectory{}).
		Where("user_id = ? AND knowledge_base_id = ? AND parent_id = ? AND deleted_at IS NULL", userID, kbID, parentID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("统计子目录失败: %w", err)
	}
	return count, nil
}

// mapDirectoryResult 统一把 GORM 的未找到错误转换为领域错误。
func mapDirectoryResult(dir *entity.DocumentDirectory, err error) (*entity.DocumentDirectory, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDirectoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询目录失败: %w", err)
	}
	return dir, nil
}
