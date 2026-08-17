package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	customretrieval "github.com/1090-f/Memora/internal/service/rag/retrieval"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// RetrievalInput 是已完成业务配置解析后的 Graph 输入。
type RetrievalInput struct {
	Request  contracts.RetrievalRequest
	Embedder embedding.Embedder
	Reranker contracts.Reranker
	// EmbeddingModelID 查询向量使用的模型配置 ID，向量检索只命中同模型生成的索引。
	EmbeddingModelID string
}

type retrievalState struct {
	input       RetrievalInput
	keywordDocs []*schema.Document
	vectorDocs  []*schema.Document
	fusedDocs   []*schema.Document
	finalDocs   []*schema.Document
	status      string
}

// RetrievalPipeline 是应用启动时编译一次的 Eino 混合检索 Graph。
type RetrievalPipeline struct {
	runnable compose.Runnable[RetrievalInput, contracts.RetrievalResult]
}

type citationBuilder interface {
	BuildKnowledge(knowledgeBaseID, documentID, documentTitle, chunkID, quotedText string, sourceLocation map[string]any, documentUpdatedAt time.Time) contracts.Citation
}

// NewRetrievalPipeline 构造 keyword/vector → RRF → reranker → knowledge → citation Graph。
func NewRetrievalPipeline(keyword, vector retriever.Retriever, citations citationBuilder) (*RetrievalPipeline, error) {
	if keyword == nil || vector == nil || citations == nil {
		return nil, fmt.Errorf("检索流水线缺少 Retriever 或 CitationService")
	}
	g := compose.NewGraph[RetrievalInput, contracts.RetrievalResult]()

	validate := compose.InvokableLambda(func(ctx context.Context, input RetrievalInput) (*retrievalState, error) {
		req := input.Request
		if req.UserID == "" || req.KnowledgeBaseID == "" || strings.TrimSpace(req.Query) == "" {
			return nil, fmt.Errorf("检索身份、知识库和 query 不能为空")
		}
		if req.Mode != contracts.RetrievalKeyword && req.Mode != contracts.RetrievalVector && req.Mode != contracts.RetrievalHybrid {
			return nil, fmt.Errorf("不支持的检索模式 %q", req.Mode)
		}
		if req.TopK <= 0 || req.TopK > 20 || len(req.DocumentIDs) > 100 {
			return nil, fmt.Errorf("检索 TopK 或文档范围超过上限")
		}
		if (req.Mode == contracts.RetrievalVector || req.Mode == contracts.RetrievalHybrid) && input.Embedder == nil {
			return nil, fmt.Errorf("向量检索未配置 Embedding 模型")
		}
		return &retrievalState{input: input}, nil
	})
	if err := g.AddLambdaNode("query_validate", validate); err != nil {
		return nil, err
	}

	retrieve := compose.InvokableLambda(func(ctx context.Context, state *retrievalState) (*retrievalState, error) {
		req := state.input.Request
		scopeIDs := make([]string, len(req.DocumentIDs))
		for i, id := range req.DocumentIDs {
			scopeIDs[i] = string(id)
		}
		group, groupCtx := errgroup.WithContext(ctx)
		if req.Mode == contracts.RetrievalKeyword || req.Mode == contracts.RetrievalHybrid {
			group.Go(func() error {
				docs, err := keyword.Retrieve(groupCtx, req.Query,
					retriever.WithTopK(req.Config.KeywordTopK),
					retrievalKeywordScope(string(req.UserID), string(req.KnowledgeBaseID), scopeIDs))
				if err == nil {
					state.keywordDocs = docs
				}
				return err
			})
		}
		if req.Mode == contracts.RetrievalVector || req.Mode == contracts.RetrievalHybrid {
			group.Go(func() error {
				opts := []retriever.Option{
					retriever.WithTopK(req.Config.VectorTopK), retriever.WithEmbedding(state.input.Embedder),
					retrievalVectorScope(string(req.UserID), string(req.KnowledgeBaseID), scopeIDs, state.input.EmbeddingModelID),
				}
				// 配置了向量分数阈值时，在候选召回层过滤低相似度结果，
				// 避免无意义候选进入 RRF/知识充分性判断。
				if req.Config.MinVectorScore > 0 {
					opts = append(opts, retriever.WithScoreThreshold(req.Config.MinVectorScore))
				}
				docs, err := vector.Retrieve(groupCtx, req.Query, opts...)
				if err == nil {
					state.vectorDocs = docs
				}
				return err
			})
		}
		if err := group.Wait(); err != nil {
			return nil, fmt.Errorf("执行底层检索失败: %w", err)
		}
		return state, nil
	})
	if err := g.AddLambdaNode("retrieve_candidates", retrieve); err != nil {
		return nil, err
	}

	fuse := compose.InvokableLambda(func(ctx context.Context, state *retrievalState) (*retrievalState, error) {
		state.fusedDocs = reciprocalRankFusion(state.keywordDocs, state.vectorDocs, state.input.Request.Config.RRFK)
		limit := state.input.Request.Config.RRFTopK
		if limit <= 0 || limit > len(state.fusedDocs) {
			limit = len(state.fusedDocs)
		}
		state.fusedDocs = state.fusedDocs[:limit]
		for _, doc := range state.fusedDocs {
			einoadapter.SetMetaString(doc, einoadapter.MetaQuery, state.input.Request.Query)
		}
		return state, nil
	})
	if err := g.AddLambdaNode("rrf_fusion", fuse); err != nil {
		return nil, err
	}

	rerank := compose.InvokableLambda(func(ctx context.Context, state *retrievalState) (*retrievalState, error) {
		state.finalDocs = state.fusedDocs
		if state.input.Reranker != nil && len(state.fusedDocs) > 0 {
			transformer, err := einoadapter.NewContractsRerankerTransformer(state.input.Reranker)
			if err != nil {
				return nil, err
			}
			docs, err := transformer.Transform(ctx, state.fusedDocs)
			if err == nil {
				state.finalDocs = docs
			} else {
				// 冻结策略：Reranker 失败降级到 RRF，只记录组件和错误分类，不记录正文。
				if log := logger.GetLogger(); log != nil {
					log.Warn("检索 Reranker 失败，降级到 RRF", zap.String("component", "retrieval_reranker"), zap.Error(err))
				}
			}
		}
		threshold := state.input.Request.Config.RerankerThreshold
		if threshold != nil && state.input.Reranker != nil {
			filtered := state.finalDocs[:0]
			for _, doc := range state.finalDocs {
				if einoadapter.GetMetaFloat(doc.MetaData, einoadapter.MetaRerankerScore) >= *threshold {
					filtered = append(filtered, doc)
				}
			}
			state.finalDocs = filtered
		}
		sort.SliceStable(state.finalDocs, func(i, j int) bool {
			left, right := state.finalDocs[i], state.finalDocs[j]
			ls, rs := einoadapter.GetMetaFloat(left.MetaData, einoadapter.MetaRerankerScore), einoadapter.GetMetaFloat(right.MetaData, einoadapter.MetaRerankerScore)
			if ls != rs {
				return ls > rs
			}
			lr, rr := einoadapter.GetMetaFloat(left.MetaData, einoadapter.MetaRRFScore), einoadapter.GetMetaFloat(right.MetaData, einoadapter.MetaRRFScore)
			if lr != rr {
				return lr > rr
			}
			lb, rb := bestRank(left.MetaData), bestRank(right.MetaData)
			if lb != rb {
				return lb < rb
			}
			return left.ID < right.ID
		})
		limit := state.input.Request.TopK
		if configured := state.input.Request.Config.RerankerTopK; configured > 0 && configured < limit {
			limit = configured
		}
		if limit < len(state.finalDocs) {
			state.finalDocs = state.finalDocs[:limit]
		}
		return state, nil
	})
	if err := g.AddLambdaNode("reranker", rerank); err != nil {
		return nil, err
	}

	evaluate := compose.InvokableLambda(func(ctx context.Context, state *retrievalState) (*retrievalState, error) {
		minimum := state.input.Request.Config.MinimumEffectiveResult
		if minimum <= 0 {
			minimum = 1
		}
		state.status = "insufficient"
		if effectiveResultCount(state.finalDocs, state.input.Request.Mode, state.input.Request.Config.MinVectorScore) >= minimum {
			state.status = "sufficient"
		}
		return state, nil
	})
	if err := g.AddLambdaNode("knowledge_evaluate", evaluate); err != nil {
		return nil, err
	}

	build := compose.InvokableLambda(func(ctx context.Context, state *retrievalState) (contracts.RetrievalResult, error) {
		items := make([]contracts.RetrievalItem, 0, len(state.finalDocs))
		for i, doc := range state.finalDocs {
			meta := doc.MetaData
			updatedAt := einoadapter.GetMetaTime(meta, einoadapter.MetaDocumentUpdAt)
			location, _ := einoadapter.GetMetaAny(meta, einoadapter.MetaSourceLocation).(map[string]any)
			item := contracts.RetrievalItem{
				DocumentID:    contracts.ID(einoadapter.GetMetaString(meta, einoadapter.MetaDocumentID)),
				DocumentTitle: einoadapter.GetMetaString(meta, einoadapter.MetaDocumentTitle),
				DirectoryID:   contracts.ID(einoadapter.GetMetaString(meta, einoadapter.MetaDirectoryID)),
				ChunkID:       contracts.ID(einoadapter.GetMetaString(meta, einoadapter.MetaChunkID)), Content: doc.Content,
				SourceLocation: location, IndexVersion: einoadapter.GetMetaInt(meta, einoadapter.MetaIndexVersion),
				DocumentUpdatedAt: timePtr(updatedAt),
			}
			item.KeywordRank = intPtr(einoadapter.GetMetaInt(meta, einoadapter.MetaKeywordRank))
			item.VectorRank = intPtr(einoadapter.GetMetaInt(meta, einoadapter.MetaVectorRank))
			item.RRFRank = intPtr(einoadapter.GetMetaInt(meta, einoadapter.MetaRRFRank))
			item.KeywordScore = floatPtr(meta, einoadapter.MetaKeywordScore)
			item.KeywordMatchLevel = contracts.KeywordMatchLevel(einoadapter.GetMetaString(meta, einoadapter.MetaKeywordMatchLevel))
			item.KeywordMatchedTerms = append([]string(nil), einoadapter.GetMetaStrings(meta, einoadapter.MetaKeywordMatchedTerms)...)
			item.KeywordCoverage = floatPtr(meta, einoadapter.MetaKeywordCoverage)
			item.KeywordRecallStage = einoadapter.GetMetaString(meta, einoadapter.MetaKeywordRecallStage)
			item.LowConfidence = einoadapter.GetMetaBool(meta, einoadapter.MetaKeywordLowConfidence)
			item.VectorScore = floatPtr(meta, einoadapter.MetaVectorScore)
			item.RerankerScore = floatPtr(meta, einoadapter.MetaRerankerScore)
			finalRank := i + 1
			item.FinalRank = &finalRank
			item.Score = einoadapter.GetMetaFloat(meta, einoadapter.MetaRerankerScore)
			if item.RerankerScore == nil {
				item.Score = einoadapter.GetMetaFloat(meta, einoadapter.MetaRRFScore)
			}
			item.Citation = citations.BuildKnowledge(string(state.input.Request.KnowledgeBaseID), string(item.DocumentID),
				item.DocumentTitle, string(item.ChunkID), item.Content, location, updatedAt)
			items = append(items, item)
		}
		return contracts.RetrievalResult{Query: state.input.Request.Query, Mode: state.input.Request.Mode, Items: items, KnowledgeStatus: state.status}, nil
	})
	if err := g.AddLambdaNode("citation_build", build); err != nil {
		return nil, err
	}

	nodes := []string{"query_validate", "retrieve_candidates", "rrf_fusion", "reranker", "knowledge_evaluate", "citation_build"}
	if err := g.AddEdge(compose.START, nodes[0]); err != nil {
		return nil, err
	}
	for i := 0; i+1 < len(nodes); i++ {
		if err := g.AddEdge(nodes[i], nodes[i+1]); err != nil {
			return nil, err
		}
	}
	if err := g.AddEdge(nodes[len(nodes)-1], compose.END); err != nil {
		return nil, err
	}
	runnable, err := g.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("编译检索 Graph 失败: %w", err)
	}
	return &RetrievalPipeline{runnable: runnable}, nil
}

// 下列两个小适配函数把 pipeline 与具体自定义 Retriever options 隔离。
var retrievalKeywordScope = func(userID, kbID string, documentIDs []string) retriever.Option {
	return customretrieval.WithKeywordScope(customretrieval.KeywordRetrieverOptions{UserID: userID, KnowledgeBaseID: kbID, DocumentIDs: documentIDs})
}
var retrievalVectorScope = func(userID, kbID string, documentIDs []string, embeddingModelID string) retriever.Option {
	return customretrieval.WithVectorScope(customretrieval.VectorRetrieverOptions{UserID: userID, KnowledgeBaseID: kbID, DocumentIDs: documentIDs, EmbeddingModelID: embeddingModelID})
}

// Run 执行已编译检索 Graph。
func (p *RetrievalPipeline) Run(ctx context.Context, input RetrievalInput) (contracts.RetrievalResult, error) {
	started := time.Now()
	result, err := p.runnable.Invoke(ctx, input)
	result.ElapsedMS = time.Since(started).Milliseconds()
	return result, err
}

func reciprocalRankFusion(keywordDocs, vectorDocs []*schema.Document, k int) []*schema.Document {
	if k <= 0 {
		k = 60
	}
	merged := make(map[string]*schema.Document, len(keywordDocs)+len(vectorDocs))
	sources := []struct {
		docs    []*schema.Document
		keyword bool
	}{
		{docs: keywordDocs, keyword: true},
		{docs: vectorDocs},
	}
	for _, input := range sources {
		docs := input.docs
		for rank, source := range docs {
			id := source.ID
			if id == "" {
				id = einoadapter.GetMetaString(source.MetaData, einoadapter.MetaChunkID)
			}
			doc := merged[id]
			if doc == nil {
				doc = &schema.Document{ID: source.ID, Content: source.Content, MetaData: cloneMeta(source.MetaData)}
				merged[id] = doc
			} else {
				for key, value := range source.MetaData {
					doc.MetaData[key] = value
				}
			}
			weight := 1.0
			if input.keyword {
				weight = keywordRRFWeight(source.MetaData)
			}
			score := einoadapter.GetMetaFloat(doc.MetaData, einoadapter.MetaRRFScore) + weight/float64(k+rank+1)
			einoadapter.SetMetaFloat(doc, einoadapter.MetaRRFScore, score)
		}
	}
	result := make([]*schema.Document, 0, len(merged))
	for _, doc := range merged {
		result = append(result, doc)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		ls, rs := einoadapter.GetMetaFloat(left.MetaData, einoadapter.MetaRRFScore), einoadapter.GetMetaFloat(right.MetaData, einoadapter.MetaRRFScore)
		if ls != rs {
			return ls > rs
		}
		lr, rr := bestRank(left.MetaData), bestRank(right.MetaData)
		if lr != rr {
			return lr < rr
		}
		return left.ID < right.ID
	})
	for i, doc := range result {
		einoadapter.SetMetaInt(doc, einoadapter.MetaRRFRank, i+1)
	}
	return result
}

func keywordRRFWeight(meta map[string]any) float64 {
	switch contracts.KeywordMatchLevel(einoadapter.GetMetaString(meta, einoadapter.MetaKeywordMatchLevel)) {
	case contracts.KeywordMatchExact:
		return 2
	case contracts.KeywordMatchWeak:
		return 0.35
	default:
		return 1
	}
}

func effectiveResultCount(docs []*schema.Document, mode contracts.RetrievalMode, minVectorScore float64) int {
	if mode == contracts.RetrievalKeyword {
		count := 0
		for _, doc := range docs {
			level := contracts.KeywordMatchLevel(einoadapter.GetMetaString(doc.MetaData, einoadapter.MetaKeywordMatchLevel))
			if level == "" || level == contracts.KeywordMatchExact || level == contracts.KeywordMatchStrong {
				count++
			}
		}
		return count
	}
	// vector：候选已在召回层按 MinVectorScore 过滤，数量即有效数量。
	if mode == contracts.RetrievalVector {
		return len(docs)
	}
	// hybrid：未配置向量阈值时保持纯计数；配置后只统计“带合格向量分数”或“强关键词匹配”的结果，
	// 弱关键词召回且无向量分数不构成知识充分性，防止低质量结果被判为 sufficient。
	if minVectorScore <= 0 {
		return len(docs)
	}
	count := 0
	for _, doc := range docs {
		meta := doc.MetaData
		if _, ok := meta[einoadapter.MetaVectorScore]; ok {
			count++
			continue
		}
		level := contracts.KeywordMatchLevel(einoadapter.GetMetaString(meta, einoadapter.MetaKeywordMatchLevel))
		if level == "" || level == contracts.KeywordMatchExact || level == contracts.KeywordMatchStrong {
			count++
		}
	}
	return count
}

func cloneMeta(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
func bestRank(meta map[string]any) int {
	a, b := einoadapter.GetMetaInt(meta, einoadapter.MetaKeywordRank), einoadapter.GetMetaInt(meta, einoadapter.MetaVectorRank)
	if a == 0 {
		return b
	}
	if b == 0 || a < b {
		return a
	}
	return b
}
func intPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}
func floatPtr(meta map[string]any, key string) *float64 {
	if _, ok := meta[key]; !ok {
		return nil
	}
	value := einoadapter.GetMetaFloat(meta, key)
	return &value
}
func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}
