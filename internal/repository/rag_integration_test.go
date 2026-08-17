package repository

import (
	"context"
	"testing"

	"github.com/1090-f/Memora/internal/testutil"
)

// TestIntegrationVectorSearchVisibilityAndModelFilter 验证修复 1（重建失败不影响旧索引可见）
// 与修复 2（向量检索按 embedding_model_id 过滤）在真实 pgvector 上的行为。
func TestIntegrationVectorSearchVisibilityAndModelFilter(t *testing.T) {
	db := testutil.OpenRAGTestDB(t)
	ctx := context.Background()
	vectors := NewVectorRepository(db)

	userID := testutil.NewID()
	otherUserID := testutil.NewID()
	kbID := testutil.NewID()
	modelA := testutil.NewID()
	modelB := testutil.NewID()
	testutil.SeedUser(t, db, userID)
	testutil.SeedUser(t, db, otherUserID)
	testutil.SeedKnowledgeBase(t, db, kbID, userID, "测试知识库")
	testutil.SeedModelConfig(t, db, modelA, userID, "embedding", 3)
	testutil.SeedModelConfig(t, db, modelB, userID, "embedding", 2)

	// D1：重建失败（processing_status=failed），但 v1 索引已发布（active=1）。
	// 修复前该文档因状态过滤无法被检索；修复后旧版本必须仍然可见。
	d1 := testutil.NewID()
	c1 := testutil.NewID()
	testutil.SeedDocument(t, db, d1, userID, kbID, "重建失败文档", "failed", 1, &modelA)
	testutil.SeedChunk(t, db, c1, userID, kbID, d1, 0, "向量检索阈值校准测试", 1)
	testutil.SeedVector(t, db, testutil.NewID(), userID, kbID, d1, c1, 1, modelA, []float32{1, 0, 0})

	// D2：succeeded 且 active=1，向量属于模型 B（2 维）；另有 v2 chunk+向量（未激活，不可见）。
	d2 := testutil.NewID()
	c2 := testutil.NewID()
	testutil.SeedDocument(t, db, d2, userID, kbID, "模型B文档", "succeeded", 1, &modelB)
	testutil.SeedChunk(t, db, c2, userID, kbID, d2, 0, "模型B的内容", 1)
	testutil.SeedVector(t, db, testutil.NewID(), userID, kbID, d2, c2, 1, modelB, []float32{0, 1})
	c3 := testutil.NewID()
	testutil.SeedChunk(t, db, c3, userID, kbID, d2, 1, "模型B的新版本内容", 2)
	testutil.SeedVector(t, db, testutil.NewID(), userID, kbID, d2, c3, 2, modelB, []float32{0, 1})

	// 模型 A 查询（3 维）：只命中 D1 的旧版本——重建失败不影响旧索引，模型过滤同时生效。
	hits, err := vectors.SearchCosine(ctx, VectorSearchParams{
		UserID: userID, KnowledgeBaseID: kbID,
		QueryVector: []float32{1, 0, 0}, TopK: 10, EmbeddingModelID: modelA,
	})
	if err != nil {
		t.Fatalf("模型 A 检索失败: %v", err)
	}
	if len(hits) != 1 || hits[0].ChunkID != c1 {
		t.Fatalf("模型 A 检索应只命中重建失败文档的旧版本，实际: %+v", hits)
	}

	// 模型 B 查询（2 维）：只命中 D2 的 active v1；未激活 v2 不可见；不同维度向量共存不报错。
	hits, err = vectors.SearchCosine(ctx, VectorSearchParams{
		UserID: userID, KnowledgeBaseID: kbID,
		QueryVector: []float32{0, 1}, TopK: 10, EmbeddingModelID: modelB,
	})
	if err != nil {
		t.Fatalf("模型 B 检索失败: %v", err)
	}
	if len(hits) != 1 || hits[0].ChunkID != c2 {
		t.Fatalf("模型 B 检索应只命中 active v1，实际: %+v", hits)
	}

	// 分数阈值：查询向量与索引向量正交时相似度为 0，应被阈值过滤。
	threshold := 0.5
	hits, err = vectors.SearchCosine(ctx, VectorSearchParams{
		UserID: userID, KnowledgeBaseID: kbID,
		QueryVector: []float32{0, 0, 1}, TopK: 10, EmbeddingModelID: modelA, ScoreThreshold: &threshold,
	})
	if err != nil {
		t.Fatalf("阈值检索失败: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("低于阈值的向量不应返回，实际: %+v", hits)
	}

	// 其他用户不能检索到任何结果。
	hits, err = vectors.SearchCosine(ctx, VectorSearchParams{
		UserID: otherUserID, KnowledgeBaseID: kbID,
		QueryVector: []float32{1, 0, 0}, TopK: 10, EmbeddingModelID: modelA,
	})
	if err != nil {
		t.Fatalf("其他用户检索失败: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("其他用户不应检索到结果，实际: %+v", hits)
	}
}

// TestIntegrationKeywordSearchActiveVersionVisibility 验证修复 1 在真实 ParadeDB 上的行为：
// 可检索性完全由 active 索引版本 + 软删除 + 归属决定，与 processing_status 无关。
func TestIntegrationKeywordSearchActiveVersionVisibility(t *testing.T) {
	db := testutil.OpenRAGTestDB(t)
	ctx := context.Background()
	search := NewKeywordSearchRepository(db)

	userID := testutil.NewID()
	kbID := testutil.NewID()
	otherKB := testutil.NewID()
	testutil.SeedUser(t, db, userID)
	testutil.SeedKnowledgeBase(t, db, kbID, userID, "测试知识库")
	testutil.SeedKnowledgeBase(t, db, otherKB, userID, "另一个知识库")

	// D1：重建失败但 v1 已发布 → 必须可检索（修复 1 核心断言）。
	d1 := testutil.NewID()
	testutil.SeedDocument(t, db, d1, userID, kbID, "重建失败文档", "failed", 1, nil)
	testutil.SeedChunk(t, db, testutil.NewID(), userID, kbID, d1, 0, "知识库检索文档", 1)

	// D2：首次导入从未完成（active=NULL）→ 半成品不可检索。
	d2 := testutil.NewID()
	testutil.SeedDocument(t, db, d2, userID, kbID, "未完成文档", "parsing", 0, nil)
	testutil.SeedChunk(t, db, testutil.NewID(), userID, kbID, d2, 0, "知识库检索文档半成品", 1)

	// D3：已软删除 → 不可检索。
	d3 := testutil.NewID()
	testutil.SeedDocument(t, db, d3, userID, kbID, "已删除文档", "succeeded", 1, nil)
	testutil.SeedChunk(t, db, testutil.NewID(), userID, kbID, d3, 0, "知识库检索文档已删除", 1)
	testutil.SoftDeleteDocument(t, db, d3)

	// D4：其他知识库 → 不可检索。
	d4 := testutil.NewID()
	testutil.SeedDocument(t, db, d4, userID, otherKB, "其他知识库文档", "succeeded", 1, nil)
	testutil.SeedChunk(t, db, testutil.NewID(), userID, otherKB, d4, 0, "知识库检索文档", 1)

	hits, err := search.Search(ctx, KeywordSearchParams{
		UserID: userID, KnowledgeBaseID: kbID, Query: "知识库", Mode: KeywordSearchAny, TopK: 10,
	})
	if err != nil {
		t.Fatalf("关键词检索失败: %v", err)
	}
	if len(hits) != 1 || hits[0].DocumentID != d1 {
		t.Fatalf("关键词检索应只命中重建失败文档的旧版本，实际: %+v", hits)
	}
}
