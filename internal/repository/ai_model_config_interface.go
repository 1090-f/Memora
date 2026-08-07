package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
)

// AIModelConfigRepository 定义 AI 模型配置的数据访问接口。
type AIModelConfigRepository interface {
	// Create 创建新的模型配置。
	Create(ctx context.Context, config *entity.AIModelConfig) error
	// FindByID 根据 ID 查找模型配置。
	FindByID(ctx context.Context, id string) (*entity.AIModelConfig, error)
	// FindDefaultByUserAndType 根据用户 ID 和模型类型查找默认配置。
	FindDefaultByUserAndType(ctx context.Context, userID, modelType string) (*entity.AIModelConfig, error)
	// ListByUser 列出用户的所有模型配置。
	ListByUser(ctx context.Context, userID string, modelType string) ([]entity.AIModelConfig, error)
}
