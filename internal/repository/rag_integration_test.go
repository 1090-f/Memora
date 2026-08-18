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

// TestIntegrationCleanupInactiveVersions 验证旧索引版本后台清理：
// 保留当前 active + retention 个旧版本，删除更旧版本；已软删除文档全部清理；
// 正在构建的新版本（active 为 NULL 或 active+1）不受影响。
func TestIntegrationCleanupInactiveVersions(t *testing.T) {
	db := testutil.OpenRAGTestDB(t)
	ctx := context.Background()
	vectors := NewVectorRepository(db)
	chunks := NewDocumentChunkRepository(db)

	userID := testutil.NewID()
	kbID := testutil.NewID()
	modelID := testutil.NewID()
	testutil.SeedUser(t, db, userID)
	testutil.SeedKnowledgeBase(t, db, kbID, userID, "测试知识库")
	testutil.SeedModelConfig(t, db, modelID, userID, "embedding", 3)

	// D1：active=3，v1/v2 为旧版本，v3 为当前版本；retention=1 时应删除 v1，保留 v2/v3。
	d1 := testutil.NewID()
	testutil.SeedDocument(t, db, d1, userID, kbID, "多版本文档", "succeeded", 3, &modelID)
	for _, version := range []int{1, 2, 3} {
		chunkID := testutil.NewID()
		testutil.SeedChunk(t, db, chunkID, userID, kbID, d1, version, "版本内容", version)
		testutil.SeedVector(t, db, testutil.NewID(), userID, kbID, d1, chunkID, version, modelID, []float32{1, 0, 0})
	}

	// D2：active 为 NULL（首次导入进行中）→ 其 v1 数据是正在构建的新版本，不得删除。
	d2 := testutil.NewID()
	testutil.SeedDocument(t, db, d2, userID, kbID, "构建中文档", "parsing", 0, nil)
	testutil.SeedChunk(t, db, testutil.NewID(), userID, kbID, d2, 0, "构建中内容", 1)

	// D3：已软删除 → 全部清理。
	d3 := testutil.NewID()
	d3Chunk := testutil.NewID()
	testutil.SeedDocument(t, db, d3, userID, kbID, "已删除文档", "succeeded", 1, &modelID)
	testutil.SeedChunk(t, db, d3Chunk, userID, kbID, d3, 0, "已删除内容", 1)
	testutil.SeedVector(t, db, testutil.NewID(), userID, kbID, d3, d3Chunk, 1, modelID, []float32{1, 0, 0})
	testutil.SoftDeleteDocument(t, db, d3)

	if n, err := vectors.CleanupInactive(ctx, 1); err != nil {
		t.Fatalf("清理向量失败: %v", err)
	} else if n != 2 {
		t.Fatalf("应删除 2 条向量（D1 v1 + D3），实际 %d", n)
	}
	if n, err := chunks.CleanupInactive(ctx, 1); err != nil {
		t.Fatalf("清理 Chunk 失败: %v", err)
	} else if n != 2 {
		t.Fatalf("应删除 2 条 Chunk（D1 v1 + D3），实际 %d", n)
	}

	// 校验剩余：D1 的 v2/v3 保留，D2 构建中内容保留。
	var remainingChunks int64
	if err := db.Table("document_chunks").Where("document_id = ?", d1).Count(&remainingChunks).Error; err != nil {
		t.Fatalf("统计 D1 Chunk 失败: %v", err)
	}
	if remainingChunks != 2 {
		t.Fatalf("D1 应保留 v2/v3 共 2 条 Chunk，实际 %d", remainingChunks)
	}
	var d2Chunks int64
	if err := db.Table("document_chunks").Where("document_id = ?", d2).Count(&d2Chunks).Error; err != nil {
		t.Fatalf("统计 D2 Chunk 失败: %v", err)
	}
	if d2Chunks != 1 {
		t.Fatalf("D2 构建中 Chunk 不应被清理，实际 %d", d2Chunks)
	}
	var d3Chunks int64
	if err := db.Table("document_chunks").Where("document_id = ?", d3).Count(&d3Chunks).Error; err != nil {
		t.Fatalf("统计 D3 Chunk 失败: %v", err)
	}
	if d3Chunks != 0 {
		t.Fatalf("D3 已删除文档 Chunk 应全部清理，实际 %d", d3Chunks)
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
