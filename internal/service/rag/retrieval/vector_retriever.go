package retrieval

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// VectorRetrieverOptions 是 PgVectorRetriever 的自定义选项（仅 Service 注入）。
type VectorRetrieverOptions struct {
	// UserID 租户用户 ID。
	UserID string
	// KnowledgeBaseID 知识库 ID。
	KnowledgeBaseID string
	// DocumentIDs 可选：限定检索的文档范围。
	DocumentIDs []string
	// IndexVersion 可选：限定索引版本。
	IndexVersion *int
}

// WithVectorScope 注入向量检索租户过滤选项。
func WithVectorScope(scope VectorRetrieverOptions) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *VectorRetrieverOptions) {
		o.UserID = scope.UserID
		o.KnowledgeBaseID = scope.KnowledgeBaseID
		o.DocumentIDs = scope.DocumentIDs
		o.IndexVersion = scope.IndexVersion
	})
}

// PgVectorRetriever 是 Eino retriever.Retriever 的实现：
// 使用 WithEmbedding 注入的 Embedder 将查询转向量，委托 Repository 做 cosine 检索。
// 只负责 options/metadata 适配，不拼 SQL。
type PgVectorRetriever struct {
	vectors repository.VectorRepository
}

// NewPgVectorRetriever 构造向量检索器。
func NewPgVectorRetriever(vectors repository.VectorRepository) (*PgVectorRetriever, error) {
	if vectors == nil {
		return nil, fmt.Errorf("向量仓储不能为空")
	}
	return &PgVectorRetriever{vectors: vectors}, nil
}

// Retrieve 实现 Eino retriever.Retriever。
func (r *PgVectorRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{Query: query})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
	impl := retriever.GetImplSpecificOptions(&VectorRetrieverOptions{}, opts...)

	if common.Embedding == nil {
		return nil, fmt.Errorf("向量检索需要 WithEmbedding 注入 Embedder")
	}
	if impl.UserID == "" || impl.KnowledgeBaseID == "" {
		return nil, fmt.Errorf("向量检索缺少 UserID/KnowledgeBaseID 租户选项")
	}
	if query == "" {
		return nil, fmt.Errorf("检索查询不能为空")
	}

	topK := 10
	if common.TopK != nil && *common.TopK > 0 {
		topK = *common.TopK
	}
	if topK > MaxVectorTopK {
		return nil, fmt.Errorf("向量检索 TopK %d 超过上限 %d", topK, MaxVectorTopK)
	}
	if len(impl.DocumentIDs) > MaxScopeDocumentIDs {
		return nil, fmt.Errorf("文档范围过滤数量 %d 超过上限 %d", len(impl.DocumentIDs), MaxScopeDocumentIDs)
	}

	// 查询向量（带显式超时）。
	embedCtx, cancel := context.WithTimeout(ctx, vectorEmbedTimeout)
	vectors, err := common.Embedding.EmbedStrings(embedCtx, []string{query})
	cancel()
	if err != nil {
		return nil, fmt.Errorf("查询向量化失败: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("查询向量化返回数量异常: %d", len(vectors))
	}
	// 转 float32 与索引写入维度对齐，保证 pgvector cosine 比较类型一致。
	queryVector := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		queryVector[i] = float32(v)
	}

	hits, err := r.vectors.SearchCosine(ctx, repository.VectorSearchParams{
		UserID:          impl.UserID,
		KnowledgeBaseID: impl.KnowledgeBaseID,
		DocumentIDs:     impl.DocumentIDs,
		QueryVector:     queryVector,
		TopK:            topK,
		ScoreThreshold:  common.ScoreThreshold,
		IndexVersion:    impl.IndexVersion,
	})
	if err != nil {
		return nil, err
	}

	docs := make([]*schema.Document, 0, len(hits))
	for _, hit := range hits {
		rank := len(docs) + 1
		meta := map[string]any{
			einoadapter.MetaUserID:         impl.UserID,
			einoadapter.MetaKnowledgeBase:  impl.KnowledgeBaseID,
			einoadapter.MetaDocumentID:     hit.DocumentID,
			einoadapter.MetaChunkID:        hit.ChunkID,
			einoadapter.MetaIndexVersion:   hit.IndexVersion,
			einoadapter.MetaVectorRank:     rank,
			einoadapter.MetaVectorScore:    hit.Score,
			einoadapter.MetaDocumentUpdAt:  hit.UpdatedAt,
			einoadapter.MetaSourceLocation: map[string]any{"engine": "pgvector_cosine"},
		}
		doc := &schema.Document{
			ID:       hit.ChunkID,
			Content:  hit.Content,
			MetaData: meta,
		}
		doc = doc.WithScore(hit.Score)
		docs = append(docs, doc)
	}
	_ = callbacks.OnEnd(ctx, &retriever.CallbackOutput{
		Docs:  docs,
		Extra: map[string]any{"top_k": topK},
	})
	return docs, nil
}

// GetType 返回组件类型名。
func (r *PgVectorRetriever) GetType() string { return "PgVectorRetriever" }

// IsCallbacksEnabled 启用 Eino Callbacks。
func (r *PgVectorRetriever) IsCallbacksEnabled() bool { return true }
