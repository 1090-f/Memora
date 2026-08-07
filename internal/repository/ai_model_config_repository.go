package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

// aiModelConfigRepository 是 AIModelConfigRepository 接口的 GORM 实现。
type aiModelConfigRepository struct{ db *gorm.DB }

// NewAIModelConfigRepository 创建一个新的模型配置仓储实例。
func NewAIModelConfigRepository(db *gorm.DB) AIModelConfigRepository {
	return &aiModelConfigRepository{db: db}
}

// Create 创建新的模型配置。
func (r *aiModelConfigRepository) Create(ctx context.Context, config *entity.AIModelConfig) error {
	if err := r.db.WithContext(ctx).Create(config).Error; err != nil {
		return fmt.Errorf("create model config: %w", err)
	}
	return nil
}

// FindByID 根据 ID 查找模型配置。
func (r *aiModelConfigRepository) FindByID(ctx context.Context, id string) (*entity.AIModelConfig, error) {
	var config entity.AIModelConfig
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query model config: %w", err)
	}
	return &config, nil
}

// FindDefaultByUserAndType 根据用户 ID 和模型类型查找默认配置。
func (r *aiModelConfigRepository) FindDefaultByUserAndType(ctx context.Context, userID, modelType string) (*entity.AIModelConfig, error) {
	var config entity.AIModelConfig
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND model_type = ? AND is_default = true AND enabled = true AND deleted_at IS NULL", userID, modelType).
		First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query default model config: %w", err)
	}
	return &config, nil
}

// ListByUser 列出用户的所有模型配置。
func (r *aiModelConfigRepository) ListByUser(ctx context.Context, userID string, modelType string) ([]entity.AIModelConfig, error) {
	var configs []entity.AIModelConfig
	query := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID)
	if modelType != "" {
		query = query.Where("model_type = ?", modelType)
	}
	err := query.Order("is_default DESC, updated_at DESC").Find(&configs).Error
	if err != nil {
		return nil, fmt.Errorf("list model configs: %w", err)
	}
	return configs, nil
}
