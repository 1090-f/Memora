package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
)

// DocumentDirectoryRepository 定义文档目录数据访问接口。
// 查询必须组合 user_id + knowledge_base_id + 软删除过滤。
type DocumentDirectoryRepository interface {
	// Create 创建目录。
	Create(ctx context.Context, dir *entity.DocumentDirectory) error
	// FindByID 按 ID 与用户查询目录。
	FindByID(ctx context.Context, userID, dirID string) (*entity.DocumentDirectory, error)
	// FindByIDInKB 按 ID、用户与知识库查询目录。
	FindByIDInKB(ctx context.Context, userID, kbID, dirID string) (*entity.DocumentDirectory, error)
	// ListByKB 列出知识库下全部未删除目录。
	ListByKB(ctx context.Context, userID, kbID string) ([]*entity.DocumentDirectory, error)
	// CountChildren 统计指定目录的直接子目录数量。
	CountChildren(ctx context.Context, userID, kbID, parentID string) (int64, error)
}
