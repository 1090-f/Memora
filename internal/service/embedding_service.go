package service

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

// embeddingService 实现 EmbeddingService 接口，通过 ModelFactory 获取 EmbeddingModel 进行向量化。
type embeddingService struct {
	modelFactory contracts.ModelFactory
	modelID      contracts.ID
}

// NewEmbeddingService 创建一个新的 Embedding 服务实例。
func NewEmbeddingService(modelFactory contracts.ModelFactory, modelID contracts.ID) contracts.EmbeddingService {
	return &embeddingService{
		modelFactory: modelFactory,
		modelID:      modelID,
	}
}

// Embed 将文本转换为向量。
func (s *embeddingService) Embed(ctx context.Context, text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("embedding text cannot be empty")
	}
	if s == nil || s.modelFactory == nil {
		return nil, fmt.Errorf("embedding model factory is not configured")
	}
	if s.modelID == "" {
		return nil, fmt.Errorf("embedding model ID is not configured")
	}

	model, err := s.modelFactory.GetEmbeddingModel(ctx, s.modelID)
	if err != nil {
		return nil, fmt.Errorf("get embedding model: %w", err)
	}

	vectors, err := model.Embed(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("embed text: %w", err)
	}

	if len(vectors) == 0 {
		return nil, fmt.Errorf("embedding returned no vectors")
	}

	// 将 float32 转换为 float64
	result := make([]float64, len(vectors[0]))
	for i, v := range vectors[0] {
		result[i] = float64(v)
	}

	return result, nil
}
