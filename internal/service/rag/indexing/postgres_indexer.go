// Package indexing 提供基于 PostgreSQL 的 Eino Indexer 实现。
package indexing

import (
	"context"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

// PostgresIndexerConfig 定义向量索引器配置。
type PostgresIndexerConfig struct {
	// EmbeddingModelID 是当前使用的 Embedding 模型配置 ID（写入 document_vectors）。
	EmbeddingModelID string
	// BatchSize 是 Embedder 批调用大小上限。
	BatchSize int
	// Timeout 是单次 Embedder 调用超时。
	Timeout time.Duration
	// Repository 是向量持久化接口。
	Repository repository.VectorRepository
}

// PostgresIndexer 是 Eino indexer.Indexer 的实现：
// 读取 schema.Document 与 metadata，使用 indexer.WithEmbedding 注入的 Embedder
// 批量生成向量，委托 Repository 写入 document_vectors（status=ready）。
// 本组件不持有 GORM/原生 SQL。
type PostgresIndexer struct {
	cfg PostgresIndexerConfig
}

// NewPostgresIndexer 构造向量索引器。
func NewPostgresIndexer(cfg PostgresIndexerConfig) (*PostgresIndexer, error) {
	if cfg.Repository == nil {
		return nil, fmt.Errorf("向量仓储不能为空")
	}
	if cfg.EmbeddingModelID == "" {
		return nil, fmt.Errorf("Embedding 模型配置 ID 不能为空")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 32
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &PostgresIndexer{cfg: cfg}, nil
}

// Store 实现 Eino indexer.Indexer。
func (p *PostgresIndexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	// 注入 Callbacks 运行上下文并派发 Start 事件；出错时由 defer 上报 OnError。
	ctx = callbacks.EnsureRunInfo(ctx, p.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	// 从 opts 提取通用选项；Indexer 依赖调用方通过 WithEmbedding 注入 Embedder。
	common := indexer.GetCommonOptions(&indexer.Options{}, opts...)
	if common.Embedding == nil {
		return nil, fmt.Errorf("indexer 需要 WithEmbedding 注入 Embedder")
	}
	if len(docs) == 0 {
		return nil, nil
	}

	// 分批调用 Embedder。
	vectors, err := p.embedBatch(ctx, common.Embedding, docs)
	if err != nil {
		return nil, err
	}

	// 组装实体并批量持久化。
	records := make([]*entity.DocumentVector, 0, len(docs))
	ids := make([]string, 0, len(docs))
	for i, doc := range docs {
		record, convErr := vectorFromDocument(doc, vectors[i], p.cfg.EmbeddingModelID)
		if convErr != nil {
			return nil, convErr
		}
		records = append(records, record)
		ids = append(ids, doc.ID)
	}
	if _, err := p.cfg.Repository.BatchUpsert(ctx, records); err != nil {
		return nil, err
	}
	_ = callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids, Extra: map[string]any{"vectors": len(records)}})
	return ids, nil
}

// embedBatch 分批调用 Eino Embedder 生成向量，每批带超时。
func (p *PostgresIndexer) embedBatch(ctx context.Context, emb embedding.Embedder, docs []*schema.Document) ([][]float64, error) {
	var all [][]float64
	for start := 0; start < len(docs); start += p.cfg.BatchSize {
		end := start + p.cfg.BatchSize
		if end > len(docs) {
			end = len(docs)
		}
		texts := make([]string, 0, end-start)
		for _, doc := range docs[start:end] {
			texts = append(texts, doc.Content)
		}
		batchCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
		batch, err := emb.EmbedStrings(batchCtx, texts)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("批量 Embedding 失败(第 %d 批): %w", start/p.cfg.BatchSize+1, err)
		}
		if len(batch) != len(texts) {
			return nil, fmt.Errorf("Embedding 返回数量 %d 与输入 %d 不一致", len(batch), len(texts))
		}
		all = append(all, batch...)
	}
	return all, nil
}

// GetType 返回组件类型名。
func (p *PostgresIndexer) GetType() string { return "PostgresIndexer" }

// IsCallbacksEnabled 启用 Eino Callbacks。
func (p *PostgresIndexer) IsCallbacksEnabled() bool { return true }

// vectorFromDocument 从 schema.Document 与向量构造 document_vectors 实体。
func vectorFromDocument(doc *schema.Document, vector []float64, modelID string) (*entity.DocumentVector, error) {
	userID := einoadapter.GetMetaString(doc.MetaData, einoadapter.MetaUserID)
	kbID := einoadapter.GetMetaString(doc.MetaData, einoadapter.MetaKnowledgeBase)
	documentID := einoadapter.GetMetaString(doc.MetaData, einoadapter.MetaDocumentID)
	chunkID := einoadapter.GetMetaString(doc.MetaData, einoadapter.MetaChunkID)
	indexVersion := einoadapter.GetMetaInt(doc.MetaData, einoadapter.MetaIndexVersion)
	// 向量记录依赖租户、文档与版本元数据定位，缺失时拒绝写入避免脏数据。
	if userID == "" || kbID == "" || documentID == "" || chunkID == "" || indexVersion <= 0 {
		return nil, fmt.Errorf("向量缺少 user_id/knowledge_base_id/document_id/chunk_id/index_version 元数据")
	}
	if len(vector) == 0 {
		return nil, fmt.Errorf("向量为空")
	}
	// pgvector 以 float32 存储，与模型输出维度保持一致。
	embedding := make([]float32, len(vector))
	for i, v := range vector {
		embedding[i] = float32(v)
	}
	return &entity.DocumentVector{
		UserID:           userID,
		KnowledgeBaseID:  kbID,
		DocumentID:       documentID,
		ChunkID:          chunkID,
		IndexVersion:     indexVersion,
		EmbeddingModelID: modelID,
		EmbeddingDim:     len(embedding),
		Embedding:        embedding,
		Status:           "ready",
	}, nil
}
