package ai

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
)

// einoEmbeddingModelAdapter 将 Eino Embedder 适配为 contracts.EmbeddingModel。
type einoEmbeddingModelAdapter struct {
	embedder  embedding.Embedder
	dimension int
}

// Embed 实现 contracts.EmbeddingModel.Embed。
func (a *einoEmbeddingModelAdapter) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// 调用 Eino Embedder
	resp, err := a.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("eino embed: %w", err)
	}

	// 转换为 float32
	result := make([][]float32, len(resp))
	for i, vec := range resp {
		result[i] = make([]float32, len(vec))
		for j, v := range vec {
			result[i][j] = float32(v)
		}
	}

	return result, nil
}

// Dimension 实现 contracts.EmbeddingModel.Dimension。
func (a *einoEmbeddingModelAdapter) Dimension() int {
	return a.dimension
}
