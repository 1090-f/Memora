// Package retrieval 提供基于 PostgreSQL 的关键词/向量检索 Retriever。
package retrieval

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/1090-f/Memora/internal/service/rag/tokenizer"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// KeywordRetrieverOptions 是 PostgresKeywordRetriever 的自定义选项。
// 身份类选项只能由 Service 注入，禁止来自用户请求。
type KeywordRetrieverOptions struct {
	// UserID 租户用户 ID。
	UserID string
	// KnowledgeBaseID 知识库 ID。
	KnowledgeBaseID string
	// DocumentIDs 可选：限定检索的文档范围。
	DocumentIDs []string
	// IndexVersion 可选：限定索引版本。
	IndexVersion *int
}

// WithKeywordScope 注入租户过滤选项（仅 Service 使用）。
func WithKeywordScope(scope KeywordRetrieverOptions) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *KeywordRetrieverOptions) {
		o.UserID = scope.UserID
		o.KnowledgeBaseID = scope.KnowledgeBaseID
		o.DocumentIDs = scope.DocumentIDs
		o.IndexVersion = scope.IndexVersion
	})
}

// PostgresKeywordRetriever 是 Eino retriever.Retriever 的实现：
// 只负责 Eino options/metadata 与 Repository 查询参数/结果的适配，不拼 SQL。
type PostgresKeywordRetriever struct {
	search    repository.KeywordSearchRepository
	tokenizer tokenizer.Tokenizer
}

// NewPostgresKeywordRetriever 构造关键词检索器。
func NewPostgresKeywordRetriever(search repository.KeywordSearchRepository, tok tokenizer.Tokenizer) (*PostgresKeywordRetriever, error) {
	if search == nil {
		return nil, fmt.Errorf("关键词检索仓储不能为空")
	}
	if tok == nil {
		tok = tokenizer.NewNgramTokenizer(tokenizer.DefaultNgramConfig())
	}
	return &PostgresKeywordRetriever{search: search, tokenizer: tok}, nil
}

// Retrieve 实现 Eino retriever.Retriever。
func (r *PostgresKeywordRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{Query: query})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
	impl := retriever.GetImplSpecificOptions(&KeywordRetrieverOptions{}, opts...)

	// TopK 默认 10，仅接受正向覆盖，避免默认值不可控。
	topK := 10
	if common.TopK != nil && *common.TopK > 0 {
		topK = *common.TopK
	}
	if topK > MaxKeywordTopK {
		return nil, fmt.Errorf("关键词检索 TopK %d 超过上限 %d", topK, MaxKeywordTopK)
	}
	if len(impl.DocumentIDs) > MaxScopeDocumentIDs {
		return nil, fmt.Errorf("文档范围过滤数量 %d 超过上限 %d", len(impl.DocumentIDs), MaxScopeDocumentIDs)
	}
	if impl.UserID == "" || impl.KnowledgeBaseID == "" {
		return nil, fmt.Errorf("关键词检索缺少 UserID/KnowledgeBaseID 租户选项")
	}
	if query == "" {
		return nil, fmt.Errorf("检索查询不能为空")
	}

	// 本地分词得到查询 Token，仓储基于 Token 做 FTS 检索，避免原始查询直拼 SQL。
	tokens := r.tokenizer.Tokenize(query)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("查询分词后无有效 Token")
	}

	hits, err := r.search.Search(ctx, repository.KeywordSearchParams{
		UserID:          impl.UserID,
		KnowledgeBaseID: impl.KnowledgeBaseID,
		DocumentIDs:     impl.DocumentIDs,
		QueryTokens:     tokens,
		TopK:            topK,
		IndexVersion:    impl.IndexVersion,
	})
	if err != nil {
		return nil, err
	}

	docs := make([]*schema.Document, 0, len(hits))
	for _, hit := range hits {
		// 低于阈值的命中直接丢弃；rank 按过滤后的连续序号重排，保证无空洞。
		if common.ScoreThreshold != nil && hit.Score < *common.ScoreThreshold {
			continue
		}
		rank := len(docs) + 1
		meta := map[string]any{
			einoadapter.MetaUserID:         impl.UserID,
			einoadapter.MetaKnowledgeBase:  impl.KnowledgeBaseID,
			einoadapter.MetaDocumentID:     hit.DocumentID,
			einoadapter.MetaChunkID:        hit.ChunkID,
			einoadapter.MetaIndexVersion:   hit.IndexVersion,
			einoadapter.MetaKeywordRank:    rank,
			einoadapter.MetaKeywordScore:   hit.Score,
			einoadapter.MetaDocumentUpdAt:  hit.UpdatedAt,
			einoadapter.MetaSourceLocation: map[string]any{"engine": "postgres_fts"},
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
		Extra: map[string]any{"top_k": topK, "tokens": len(tokens)},
	})
	return docs, nil
}

// GetType 返回组件类型名。
func (r *PostgresKeywordRetriever) GetType() string { return "PostgresKeywordRetriever" }

// IsCallbacksEnabled 启用 Eino Callbacks。
func (r *PostgresKeywordRetriever) IsCallbacksEnabled() bool { return true }
