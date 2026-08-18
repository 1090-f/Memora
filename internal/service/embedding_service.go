package service

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// embeddingService 实现 EmbeddingService 接口，通过 ModelFactory 获取 EmbeddingModel 进行向量化。
type embeddingService struct {
	modelFactory contracts.ModelFactory
	models       repository.AIModelConfigRepository
}

// NewEmbeddingService 创建一个新的 Embedding 服务实例。
func NewEmbeddingService(modelFactory contracts.ModelFactory, models repository.AIModelConfigRepository) contracts.EmbeddingService {
	return &embeddingService{
		modelFactory: modelFactory,
		models:       models,
	}
}

// Embed 将文本转换为向量，userID 用于获取用户配置的 embedding 模型。
func (s *embeddingService) Embed(ctx context.Context, userID string, text string) ([]float64, error) {
	vector, _, err := s.EmbedWithModelID(ctx, userID, text)
	return vector, err
}

// EmbedWithModelID 将文本转换为向量，并返回使用的模型ID。
func (s *embeddingService) EmbedWithModelID(ctx context.Context, userID string, text string) ([]float64, string, error) {
	logger.Debug("[Embedding-EmbedWithModelID] 开始向量化",
		zap.String("user_id", userID),
		zap.Int("text_length", len(text)),
	)

	if text == "" {
		return nil, "", fmt.Errorf("embedding text cannot be empty")
	}
	if s == nil || s.modelFactory == nil {
		logger.Error("[Embedding-EmbedWithModelID] embedding model factory 未配置")
		return nil, "", fmt.Errorf("embedding model factory is not configured")
	}
	if s.models == nil {
		logger.Error("[Embedding-EmbedWithModelID] embedding model config repository 未配置")
		return nil, "", fmt.Errorf("embedding model config repository is not configured")
	}

	// 按用户获取默认的 embedding 模型配置
	logger.Debug("[Embedding-EmbedWithModelID] 步骤1: 获取用户默认 embedding 模型配置")
	config, err := s.models.FindDefaultByUserAndType(ctx, userID, "embedding")
	if err != nil {
		logger.Error("[Embedding-EmbedWithModelID] 获取 embedding 模型配置失败",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return nil, "", fmt.Errorf("find default embedding model for user %s: %w", userID, err)
	}
	logger.Debug("[Embedding-EmbedWithModelID] 获取 embedding 模型配置成功",
		zap.String("model_id", config.ID),
		zap.String("model_name", config.Name),
	)

	// 获取 EmbeddingModel
	logger.Debug("[Embedding-EmbedWithModelID] 步骤2: 获取 EmbeddingModel")
	model, err := s.modelFactory.GetEmbeddingModel(ctx, contracts.ID(config.ID))
	if err != nil {
		logger.Error("[Embedding-EmbedWithModelID] 获取 EmbeddingModel 失败", zap.Error(err))
		return nil, "", fmt.Errorf("get embedding model: %w", err)
	}

	// 调用模型进行向量化
	logger.Debug("[Embedding-EmbedWithModelID] 步骤3: 调用模型进行向量化")
	vectors, err := model.Embed(ctx, []string{text})
	if err != nil {
		logger.Error("[Embedding-EmbedWithModelID] 向量化失败", zap.Error(err))
		return nil, "", fmt.Errorf("embed text: %w", err)
	}

	if len(vectors) == 0 {
		logger.Error("[Embedding-EmbedWithModelID] 向量化返回空结果")
		return nil, "", fmt.Errorf("embedding returned no vectors")
	}

	// 将 float32 转换为 float64
	result := make([]float64, len(vectors[0]))
	for i, v := range vectors[0] {
		result[i] = float64(v)
	}

	logger.Debug("[Embedding-EmbedWithModelID] 向量化成功",
		zap.Int("vector_dim", len(result)),
		zap.String("model_id", config.ID),
	)

	return result, config.ID, nil
}
