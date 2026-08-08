package service

import (
	"context"

	"github.com/1090-f/Memora/internal/service/rag/pipeline"
)

// DocumentProcessor 是文档加工流水线的依赖接口（由 pipeline.DocumentPipeline 实现）。
type DocumentProcessor interface {
	// Run 执行一次文档加工（解析/清洗/分段/落库）。
	Run(ctx context.Context, input pipeline.ProcessInput) (pipeline.ProcessOutput, error)
	// ChunkConfigHash 返回当前分段配置的稳定哈希。
	ChunkConfigHash() string
	// EmbeddingModelID 返回当前向量索引使用的模型配置 ID；未启用向量索引时返回空。
	EmbeddingModelID() string
}
