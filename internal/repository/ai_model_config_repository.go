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

func (r *aiModelConfigRepository) FindByIDForUserAndType(ctx context.Context, userID, id, modelType string) (*entity.AIModelConfig, error) {
	var config entity.AIModelConfig
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND model_type = ? AND enabled = true AND deleted_at IS NULL", id, userID, modelType).
		First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query scoped model config: %w", err)
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

// Update 更新模型配置。
func (r *aiModelConfigRepository) Update(ctx context.Context, config *entity.AIModelConfig) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current entity.AIModelConfig
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", config.ID, config.UserID).
			First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrModelConfigNotFound
			}
			return fmt.Errorf("lock model config: %w", err)
		}
		if current.ModelType == "embedding" && embeddingIdentityChanged(&current, config) {
			referenced, err := isEmbeddingReferenced(tx, current.ID, current.UserID)
			if err != nil {
				return err
			}
			if referenced {
				return ErrModelConfigReferenced
			}
		}
		if err := tx.Save(config).Error; err != nil {
			return fmt.Errorf("update model config: %w", err)
		}
		return nil
	})
}

// Delete 软删除模型配置。
func (r *aiModelConfigRepository) Delete(ctx context.Context, id, userID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current entity.AIModelConfig
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
			First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrModelConfigNotFound
			}
			return fmt.Errorf("lock model config: %w", err)
		}
		if current.ModelType == "embedding" {
			referenced, err := isEmbeddingReferenced(tx, id, userID)
			if err != nil {
				return err
			}
			if referenced {
				return ErrModelConfigReferenced
			}
		}
		now := time.Now().UTC()
		if err := tx.Model(&entity.AIModelConfig{}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
			Updates(map[string]interface{}{"deleted_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("delete model config: %w", err)
		}
		return nil
	})
}

// IsEmbeddingReferenced 判断 Embedding 配置是否被知识库或历史索引记录引用。
func (r *aiModelConfigRepository) IsEmbeddingReferenced(ctx context.Context, id, userID string) (bool, error) {
	return isEmbeddingReferenced(r.db.WithContext(ctx), id, userID)
}

func isEmbeddingReferenced(db *gorm.DB, id, userID string) (bool, error) {
	var referenced bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM knowledge_bases WHERE user_id = ? AND embedding_model_id = ?
			UNION ALL
			SELECT 1 FROM documents WHERE user_id = ? AND embedding_model_id = ?
			UNION ALL
			SELECT 1 FROM document_vectors WHERE user_id = ? AND embedding_model_id = ?
		)`, userID, id, userID, id, userID, id).Scan(&referenced).Error
	if err != nil {
		return false, fmt.Errorf("query embedding model references: %w", err)
	}
	return referenced, nil
}

func embeddingIdentityChanged(current, target *entity.AIModelConfig) bool {
	if current.Name != target.Name || current.Provider != target.Provider || current.ModelType != target.ModelType ||
		current.BaseURL != target.BaseURL || current.Enabled && !target.Enabled {
		return true
	}
	if current.VectorDimension == nil || target.VectorDimension == nil {
		return current.VectorDimension != nil || target.VectorDimension != nil
	}
	return *current.VectorDimension != *target.VectorDimension
}
