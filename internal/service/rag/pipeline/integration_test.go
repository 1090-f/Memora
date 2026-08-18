package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/retrieval"
	"github.com/1090-f/Memora/internal/testutil"
	"github.com/cloudwego/eino/components/embedding"
)

// scriptedEmbedder 按输入文本返回预置向量，未命中时返回 default 向量。
type scriptedEmbedder struct {
	vectors map[string][]float64
}

func (e *scriptedEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, text := range texts {
		if v, ok := e.vectors[text]; ok {
			out[i] = v
		} else if v, ok := e.vectors["default"]; ok {
			out[i] = v
		} else {
			out[i] = []float64{0, 0, 0}
		}
	}
	return out, nil
}

type fakeCitations struct{}

func (fakeCitations) BuildKnowledge(knowledgeBaseID, documentID, documentTitle, chunkID, quotedText string, sourceLocation map[string]any, documentUpdatedAt time.Time) contracts.Citation {
	return contracts.Citation{
		SourceType:      contracts.CitationKnowledge,
		KnowledgeBaseID: contracts.ID(knowledgeBaseID),
		DocumentID:      contracts.ID(documentID),
		DocumentTitle:   documentTitle,
		ChunkID:         contracts.ID(chunkID),
		QuotedText:      quotedText,
	}
}

// TestIntegrationRetrievalPipelineEndToEnd 在真实 ParadeDB + pgvector 上执行完整检索 Graph，
// 覆盖修复 3：低质量向量结果不再被判定为 sufficient。
func TestIntegrationRetrievalPipelineEndToEnd(t *testing.T) {
	db := testutil.OpenRAGTestDB(t)
	ctx := context.Background()

	userID := testutil.NewID()
	kbID := testutil.NewID()
	modelID := testutil.NewID()
	testutil.SeedUser(t, db, userID)
	testutil.SeedKnowledgeBase(t, db, kbID, userID, "测试知识库")
	testutil.SeedSearchConfig(t, db, testutil.NewID(), kbID, 0.3)
	testutil.SeedModelConfig(t, db, modelID, userID, "embedding", 3)

	docID := testutil.NewID()
	chunkID := testutil.NewID()
	testutil.SeedDocument(t, db, docID, userID, kbID, "向量检索文档", "succeeded", 1, &modelID)
	testutil.SeedChunk(t, db, chunkID, userID, kbID, docID, 0, "向量检索阈值校准测试文档", 1)
	testutil.SeedVector(t, db, testutil.NewID(), userID, kbID, docID, chunkID, 1, modelID, []float32{1, 0, 0})

	keyword, err := retrieval.NewParadeDBKeywordRetriever(repository.NewKeywordSearchRepository(db))
	if err != nil {
		t.Fatalf("构造关键词检索器失败: %v", err)
	}
	vector, err := retrieval.NewPgVectorRetriever(repository.NewVectorRepository(db))
	if err != nil {
		t.Fatalf("构造向量检索器失败: %v", err)
	}
	graph, err := NewRetrievalPipeline(keyword, vector, fakeCitations{})
	if err != nil {
		t.Fatalf("编译检索 Graph 失败: %v", err)
	}

	embedder := &scriptedEmbedder{vectors: map[string][]float64{
		"default": {1, 0, 0},         // 与索引向量同向，余弦相似度 = 1
		"far":     {0, 0, 1},         // 与索引向量正交，余弦相似度 = 0
		"mid":     {0.35, 0.9367, 0}, // 余弦相似度 ≈ 0.35，位于 [min_vector_score, ambiguous_score) 区间
	}}
	config := contracts.SearchConfig{
		KeywordTopK: 10, VectorTopK: 10, RRFK: 60, RRFTopK: 10,
		MinimumEffectiveResult: 1, MinVectorScore: 0.3, AmbiguousScore: 0.45,
	}

	// 高相似度查询：关键词与向量均命中，应返回 sufficient 且带引用。
	result, err := graph.Run(ctx, RetrievalInput{
		Request: contracts.RetrievalRequest{
			UserID: contracts.ID(userID), KnowledgeBaseID: contracts.ID(kbID),
			Query: "向量检索", Mode: contracts.RetrievalHybrid, TopK: 5, Config: config,
		},
		Embedder: embedder, EmbeddingModelID: modelID,
	})
	if err != nil {
		t.Fatalf("混合检索失败: %v", err)
	}
	if result.KnowledgeStatus != "sufficient" {
		t.Fatalf("高相似度查询应 sufficient，实际 %s, items=%d", result.KnowledgeStatus, len(result.Items))
	}
	if len(result.Items) == 0 {
		t.Fatal("应返回至少一条检索结果")
	}
	if result.Items[0].ChunkID != contracts.ID(chunkID) {
		t.Fatalf("首条结果应为种子 Chunk，实际 %s", result.Items[0].ChunkID)
	}
	if result.Items[0].Citation.DocumentID != contracts.ID(docID) {
		t.Fatalf("引用缺少文档 ID: %+v", result.Items[0].Citation)
	}

	// 无答案查询：向量相似度 0 被阈值过滤，关键词不命中，应 insufficient（修复 3 核心断言）。
	result, err = graph.Run(ctx, RetrievalInput{
		Request: contracts.RetrievalRequest{
			UserID: contracts.ID(userID), KnowledgeBaseID: contracts.ID(kbID),
			Query: "far", Mode: contracts.RetrievalHybrid, TopK: 5, Config: config,
		},
		Embedder: embedder, EmbeddingModelID: modelID,
	})
	if err != nil {
		t.Fatalf("无答案混合检索失败: %v", err)
	}
	if result.KnowledgeStatus != "insufficient" {
		t.Fatalf("无答案查询应 insufficient，实际 %s, items=%d", result.KnowledgeStatus, len(result.Items))
	}

	// 边界查询（修复 3 扩展）：向量相似度 0.35 通过召回阈值但低于 ambiguous_score，
	// 关键词不命中 → 应 ambiguous（资料存在但无法明确支持结论）。
	result, err = graph.Run(ctx, RetrievalInput{
		Request: contracts.RetrievalRequest{
			UserID: contracts.ID(userID), KnowledgeBaseID: contracts.ID(kbID),
			Query: "mid", Mode: contracts.RetrievalHybrid, TopK: 5, Config: config,
		},
		Embedder: embedder, EmbeddingModelID: modelID,
	})
	if err != nil {
		t.Fatalf("边界混合检索失败: %v", err)
	}
	if result.KnowledgeStatus != "ambiguous" {
		t.Fatalf("边界查询应 ambiguous，实际 %s, items=%d", result.KnowledgeStatus, len(result.Items))
	}
}
