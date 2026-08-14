package retrieval

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	queryutil "github.com/1090-f/Memora/internal/service/rag/query"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

const (
	keywordCandidateMultiple = 4
	keywordWeakResultLimit   = 3
)

// KeywordRetrieverOptions contains trusted retrieval scope injected by the service.
type KeywordRetrieverOptions struct {
	UserID          string
	KnowledgeBaseID string
	DocumentIDs     []string
	IndexVersion    *int
}

func WithKeywordScope(scope KeywordRetrieverOptions) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *KeywordRetrieverOptions) {
		o.UserID = scope.UserID
		o.KnowledgeBaseID = scope.KnowledgeBaseID
		o.DocumentIDs = scope.DocumentIDs
		o.IndexVersion = scope.IndexVersion
	})
}

// ParadeDBKeywordRetriever implements Exact -> Strong -> Weak retrieval.
// Tokenization and AND/OR matching are delegated exclusively to pg_search.
type ParadeDBKeywordRetriever struct {
	search repository.KeywordSearchRepository
}

type keywordMatch struct {
	hit           *repository.KeywordHit
	level         contracts.KeywordMatchLevel
	matchedTerms  []string
	coverage      *float64
	recallStage   string
	lowConfidence bool
}

func NewParadeDBKeywordRetriever(search repository.KeywordSearchRepository) (*ParadeDBKeywordRetriever, error) {
	if search == nil {
		return nil, fmt.Errorf("关键词检索仓储不能为空")
	}
	return &ParadeDBKeywordRetriever{search: search}, nil
}

func (r *ParadeDBKeywordRetriever) Retrieve(ctx context.Context, rawQuery string, opts ...retriever.Option) ([]*schema.Document, error) {
	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{Query: rawQuery})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
	impl := retriever.GetImplSpecificOptions(&KeywordRetrieverOptions{}, opts...)
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

	query := queryutil.Normalize(rawQuery)
	if query == "" {
		return nil, fmt.Errorf("检索查询不能为空")
	}
	matches, err := r.retrieveLayered(ctx, impl, query, topK, common.ScoreThreshold)
	if err != nil {
		return nil, err
	}

	docs := make([]*schema.Document, 0, len(matches))
	for _, match := range matches {
		hit := match.hit
		rank := len(docs) + 1
		location := make(map[string]any)
		_ = json.Unmarshal(hit.SourceLocation, &location)
		meta := map[string]any{
			einoadapter.MetaUserID:               impl.UserID,
			einoadapter.MetaKnowledgeBase:        impl.KnowledgeBaseID,
			einoadapter.MetaDocumentID:           hit.DocumentID,
			einoadapter.MetaChunkID:              hit.ChunkID,
			einoadapter.MetaIndexVersion:         hit.IndexVersion,
			einoadapter.MetaKeywordRank:          rank,
			einoadapter.MetaKeywordScore:         hit.Score,
			einoadapter.MetaKeywordMatchLevel:    string(match.level),
			einoadapter.MetaKeywordMatchedTerms:  append([]string(nil), match.matchedTerms...),
			einoadapter.MetaKeywordRecallStage:   match.recallStage,
			einoadapter.MetaKeywordLowConfidence: match.lowConfidence,
			einoadapter.MetaDocumentUpdAt:        hit.UpdatedAt,
			einoadapter.MetaDocumentTitle:        hit.DocumentTitle,
			einoadapter.MetaSourceLocation:       location,
		}
		if hit.DirectoryID != nil {
			meta[einoadapter.MetaDirectoryID] = *hit.DirectoryID
		}
		if match.coverage != nil {
			meta[einoadapter.MetaKeywordCoverage] = *match.coverage
		}
		doc := (&schema.Document{ID: hit.ChunkID, Content: hit.Content, MetaData: meta}).WithScore(hit.Score)
		docs = append(docs, doc)
	}
	_ = callbacks.OnEnd(ctx, &retriever.CallbackOutput{
		Docs: docs,
		Extra: map[string]any{
			"top_k": topK, "query": query, "tokenizer": "paradedb_ngram_2_2",
		},
	})
	return docs, nil
}

func (r *ParadeDBKeywordRetriever) retrieveLayered(ctx context.Context, scope *KeywordRetrieverOptions, query string, topK int, scoreThreshold *float64) ([]keywordMatch, error) {
	candidateTopK := topK * keywordCandidateMultiple
	if candidateTopK < topK {
		candidateTopK = topK
	}
	if candidateTopK > MaxKeywordTopK {
		candidateTopK = MaxKeywordTopK
	}

	// ### uses the fixed-position ngram index for exact substring matching.
	hits, searchErr := r.searchLayer(ctx, scope, query, repository.KeywordSearchExact, candidateTopK)
	if searchErr != nil {
		return nil, searchErr
	}
	exact := acceptParadeDBMatches(hits, contracts.KeywordMatchExact, query, scoreThreshold)
	if len(exact) > 0 {
		return limitKeywordMatches(exact, topK), nil
	}

	// &&& requires every token produced by the ParadeDB index tokenizer to match.
	hits, searchErr = r.searchLayer(ctx, scope, query, repository.KeywordSearchAll, candidateTopK)
	if searchErr != nil {
		return nil, searchErr
	}
	strong := acceptParadeDBMatches(hits, contracts.KeywordMatchStrong, query, scoreThreshold)
	if len(strong) > 0 {
		return limitKeywordMatches(strong, topK), nil
	}

	// ||| is only reached when conjunction recall is empty.
	hits, searchErr = r.searchLayer(ctx, scope, query, repository.KeywordSearchAny, candidateTopK)
	if searchErr != nil {
		return nil, searchErr
	}
	weak := acceptParadeDBMatches(hits, contracts.KeywordMatchWeak, query, scoreThreshold)
	limit := topK
	if limit > keywordWeakResultLimit {
		limit = keywordWeakResultLimit
	}
	return limitKeywordMatches(weak, limit), nil
}

func (r *ParadeDBKeywordRetriever) searchLayer(ctx context.Context, scope *KeywordRetrieverOptions, query string, mode repository.KeywordSearchMode, topK int) ([]*repository.KeywordHit, error) {
	return r.search.Search(ctx, repository.KeywordSearchParams{
		UserID: scope.UserID, KnowledgeBaseID: scope.KnowledgeBaseID, DocumentIDs: scope.DocumentIDs,
		Query: query, Mode: mode, TopK: topK, IndexVersion: scope.IndexVersion,
	})
}

func acceptParadeDBMatches(hits []*repository.KeywordHit, level contracts.KeywordMatchLevel, query string, scoreThreshold *float64) []keywordMatch {
	accepted := make([]keywordMatch, 0, len(hits))
	for _, hit := range hits {
		if hit == nil || (scoreThreshold != nil && hit.Score < *scoreThreshold) {
			continue
		}
		match := keywordMatch{hit: hit, level: level, recallStage: "strong"}
		switch level {
		case contracts.KeywordMatchExact:
			coverage := 1.0
			match.coverage = &coverage
			match.matchedTerms = []string{query}
			match.recallStage = "exact"
		case contracts.KeywordMatchStrong:
			coverage := 1.0
			match.coverage = &coverage
		case contracts.KeywordMatchWeak:
			match.recallStage = "weak_fallback"
			match.lowConfidence = true
		}
		accepted = append(accepted, match)
	}
	return accepted
}

func limitKeywordMatches(matches []keywordMatch, limit int) []keywordMatch {
	if limit < 0 {
		limit = 0
	}
	if len(matches) > limit {
		return matches[:limit]
	}
	return matches
}

func (r *ParadeDBKeywordRetriever) GetType() string { return "ParadeDBKeywordRetriever" }

func (r *ParadeDBKeywordRetriever) IsCallbacksEnabled() bool { return true }
