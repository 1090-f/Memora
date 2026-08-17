package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	queryutil "github.com/1090-f/Memora/internal/service/rag/query"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// 长期记忆检索器的默认配置。
const (
	DefaultVectorTopK        = 30   // 向量检索返回数
	DefaultKeywordTopK       = 30   // 关键词检索返回数
	DefaultRRFK              = 60   // RRF 常数
	DefaultFinalTopK         = 10   // 最终返回数
	DefaultSimilarityWeight  = 0.6  // 相似度权重
	DefaultImportanceWeight  = 0.3  // 重要性权重
	DefaultTimeDecayWeight   = 0.1  // 时间衰减权重
	DefaultTimeDecayHalfLife = 30.0 // 时间衰减半衰期（天）
	DefaultMinImportance     = 0.3  // 最小重要性阈值
)

// RetrieverConfig 记忆检索器配置。
type RetrieverConfig struct {
	VectorTopK        int
	KeywordTopK       int
	RRFK              int
	FinalTopK         int
	SimilarityWeight  float64
	ImportanceWeight  float64
	TimeDecayWeight   float64
	TimeDecayHalfLife float64
	MinImportance     float64
}

// DefaultRetrieverConfig 返回默认配置。
func DefaultRetrieverConfig() *RetrieverConfig {
	return &RetrieverConfig{
		VectorTopK:        DefaultVectorTopK,
		KeywordTopK:       DefaultKeywordTopK,
		RRFK:              DefaultRRFK,
		FinalTopK:         DefaultFinalTopK,
		SimilarityWeight:  DefaultSimilarityWeight,
		ImportanceWeight:  DefaultImportanceWeight,
		TimeDecayWeight:   DefaultTimeDecayWeight,
		TimeDecayHalfLife: DefaultTimeDecayHalfLife,
		MinImportance:     DefaultMinImportance,
	}
}

// memoryRetriever 实现 MemoryRetriever 接口。
type memoryRetriever struct {
	memoryRepo   repository.MemoryRepository
	embeddingSvc contracts.EmbeddingService
	config       *RetrieverConfig
}

// NewMemoryRetriever 创建记忆检索器。
func NewMemoryRetriever(
	memoryRepo repository.MemoryRepository,
	embeddingSvc contracts.EmbeddingService,
) contracts.MemoryRetriever {
	return &memoryRetriever{
		memoryRepo:   memoryRepo,
		embeddingSvc: embeddingSvc,
		config:       DefaultRetrieverConfig(),
	}
}

// Retrieve 搜索与查询匹配的记忆并返回排序后的结果。
// 采用双路召回策略：向量检索 + 关键词检索，通过 RRF 融合后复合排序。
func (r *memoryRetriever) Retrieve(
	ctx context.Context,
	query contracts.MemoryQuery,
) ([]contracts.MemoryQueryResult, error) {
	userID := string(query.UserID)
	var kbID *string
	if query.KnowledgeBaseID != "" {
		id := string(query.KnowledgeBaseID)
		kbID = &id
	}

	// 记忆检索入口日志
	logger.Info("[记忆检索-入口] 开始检索记忆",
		zap.String("user_id", userID),
		zap.String("query", query.Query),
		zap.Int("top_k", query.TopK),
	)

	// 1. 并行执行向量检索和关键词检索
	type vectorResult struct {
		results []repository.VectorSearchResult
		err     error
	}
	type keywordResult struct {
		results []repository.KeywordMemorySearchResult
		err     error
	}

	vectorCh := make(chan vectorResult, 1)
	keywordCh := make(chan keywordResult, 1)

	// goroutine 1: 向量检索
	go func() {
		queryVector, err := r.embeddingSvc.Embed(ctx, userID, query.Query)
		if err != nil {
			vectorCh <- vectorResult{err: fmt.Errorf("embed query: %w", err)}
			return
		}
		queryVectorStr := formatPgVectorFromFloat64(queryVector)
		searchReq := repository.VectorSearchRequest{
			UserID:          userID,
			KnowledgeBaseID: kbID,
			QueryVector:     queryVectorStr,
			EmbeddingDim:    len(queryVector),
			TopK:            r.config.VectorTopK,
			MinImportance:   r.config.MinImportance,
		}
		results, err := r.memoryRepo.SearchByVector(ctx, searchReq)
		vectorCh <- vectorResult{results: results, err: err}
	}()

	// goroutine 2: 关键词检索
	go func() {
		normalized := queryutil.Normalize(query.Query)
		if normalized == "" {
			keywordCh <- keywordResult{results: nil, err: nil}
			return
		}
		searchReq := repository.KeywordMemorySearchRequest{
			UserID:          userID,
			KnowledgeBaseID: kbID,
			Query:           normalized,
			TopK:            r.config.KeywordTopK,
		}
		results, err := r.memoryRepo.SearchByKeyword(ctx, searchReq)
		keywordCh <- keywordResult{results: results, err: err}
	}()

	// 等待两路结果
	vr := <-vectorCh
	kr := <-keywordCh

	// 记忆检索结果日志
	if vr.err != nil {
		logger.Error("[记忆检索-向量检索] 失败", zap.Error(vr.err))
		return nil, fmt.Errorf("vector search: %w", vr.err)
	}
	logger.Info("[记忆检索-向量检索] 完成",
		zap.Int("结果数量", len(vr.results)),
	)

	if kr.err != nil {
		// 关键词检索失败不阻塞，降级为纯向量检索
		logger.Warn("[记忆检索-关键词检索] 失败，降级为纯向量检索", zap.Error(kr.err))
		kr.results = nil
	} else {
		logger.Info("[记忆检索-关键词检索] 完成",
			zap.Int("结果数量", len(kr.results)),
		)
	}

	// 2. RRF 融合
	fused := r.rrfFuse(vr.results, kr.results)

	// 3. 复合排序（相似度 + 重要性 + 时间衰减）
	now := time.Now()
	ranked := r.rankCandidates(fused, now)

	// 4. 截取 TopK
	finalTopK := r.config.FinalTopK
	if query.TopK > 0 && query.TopK < finalTopK {
		finalTopK = query.TopK
	}
	if len(ranked) > finalTopK {
		ranked = ranked[:finalTopK]
	}

	// 5. 更新最后访问时间
	ids := make([]string, len(ranked))
	for i, item := range ranked {
		ids[i] = string(item.MemoryID)
	}
	if err := r.memoryRepo.UpdateLastAccessedAt(ctx, ids); err != nil {
		// 更新失败不影响返回结果
		logger.Warn("update last accessed at failed", zap.Error(err))
	}

	// 记忆检索完成日志
	logger.Info("[记忆检索-完成] 返回最终结果",
		zap.Int("最终结果数量", len(ranked)),
	)

	return ranked, nil
}

// fusedCandidate 融合后的候选记忆。
type fusedCandidate struct {
	memory     repository.VectorSearchResult // 复用结构，Similarity 字段存储 RRF 分数
	rrfScore   float64
	similarity float64 // 原始向量相似度
}

// rrfFuse 使用 Reciprocal Rank Fusion 融合向量检索和关键词检索结果。
func (r *memoryRetriever) rrfFuse(
	vectorResults []repository.VectorSearchResult,
	keywordResults []repository.KeywordMemorySearchResult,
) []repository.VectorSearchResult {
	k := float64(r.config.RRFK)

	// 用 memory ID 作为 key，累积 RRF 分数
	rrfScores := make(map[string]float64)
	// 保留原始数据用于后续排序
	vectorMap := make(map[string]repository.VectorSearchResult)

	// 处理向量检索结果
	for rank, vr := range vectorResults {
		id := vr.Memory.ID
		rrfScores[id] += 1.0 / (k + float64(rank+1))
		vectorMap[id] = vr
	}

	// 处理关键词检索结果
	for rank, kr := range keywordResults {
		id := kr.Memory.ID
		rrfScores[id] += 1.0 / (k + float64(rank+1))
		// 如果向量检索没找到这条记忆，补充到 vectorMap 中
		if _, exists := vectorMap[id]; !exists {
			vectorMap[id] = repository.VectorSearchResult{
				Memory:     kr.Memory,
				Similarity: 0, // 向量检索未命中，相似度为 0
			}
		}
	}

	// 按 RRF 分数排序，构造返回结果
	type idScore struct {
		id    string
		score float64
	}
	var sorted []idScore
	for id, score := range rrfScores {
		sorted = append(sorted, idScore{id: id, score: score})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	// 取 Top FinalTopK * 2 作为候选（后续还会复合排序截断）
	limit := r.config.FinalTopK * 2
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	var fused []repository.VectorSearchResult
	for _, item := range sorted {
		vr := vectorMap[item.id]
		// 将 RRF 分数写入 Similarity 字段，供后续复合排序使用
		vr.Similarity = item.score
		fused = append(fused, vr)
	}

	return fused
}

// rankCandidates 复合排序候选记忆。
// 对于向量检索命中的记忆，使用原始相似度；对于仅关键词命中的记忆，使用 RRF 分数作为相似度代理。
func (r *memoryRetriever) rankCandidates(
	candidates []repository.VectorSearchResult,
	now time.Time,
) []contracts.MemoryQueryResult {
	var results []contracts.MemoryQueryResult

	for _, cand := range candidates {
		var scopeID *contracts.ID
		if cand.Memory.ScopeID != nil {
			id := contracts.ID(*cand.Memory.ScopeID)
			scopeID = &id
		}

		results = append(results, contracts.MemoryQueryResult{
			MemoryID:   contracts.ID(cand.Memory.ID),
			MemoryType: contracts.MemoryType(cand.Memory.MemoryType),
			ScopeType:  contracts.MemoryScope(cand.Memory.ScopeType),
			ScopeID:    scopeID,
			Content:    cand.Memory.Content,
			Similarity: cand.Similarity,
			Importance: cand.Memory.Importance,
			UpdatedAt:  cand.Memory.UpdatedAt,
		})
	}

	// 按综合得分排序
	sort.Slice(results, func(i, j int) bool {
		return r.calculateScore(results[i], now) > r.calculateScore(results[j], now)
	})

	return results
}

// calculateScore 计算综合得分。
func (r *memoryRetriever) calculateScore(
	result contracts.MemoryQueryResult,
	now time.Time,
) float64 {
	// 1. 相似度得分（向量检索命中的使用原始相似度，仅关键词命中的使用 RRF 分数）
	similarityScore := result.Similarity

	// 2. 重要性得分（已归一化到 0-1）
	importanceScore := result.Importance

	// 3. 时间衰减得分
	timeDecayScore := r.calculateTimeDecay(result.UpdatedAt, now)

	// 4. 加权求和
	score := r.config.SimilarityWeight*similarityScore +
		r.config.ImportanceWeight*importanceScore +
		r.config.TimeDecayWeight*timeDecayScore

	return score
}

// calculateTimeDecay 计算时间衰减得分。
func (r *memoryRetriever) calculateTimeDecay(updatedAt, now time.Time) float64 {
	// 使用指数衰减公式：score = e^(-λ * t)
	// λ = ln(2) / halfLife
	daysSinceUpdate := now.Sub(updatedAt).Hours() / 24
	lambda := math.Log(2) / r.config.TimeDecayHalfLife
	decay := math.Exp(-lambda * daysSinceUpdate)

	return decay
}

// float64SliceToBytes 将 float64 切片转换为字节数组。
func float64SliceToBytes(v []float64) []byte {
	buf := new(bytes.Buffer)
	for _, f := range v {
		binary.Write(buf, binary.LittleEndian, f)
	}
	return buf.Bytes()
}

// bytesToFloat64Slice 将字节数组转换为 float64 切片。
func bytesToFloat64Slice(b []byte) []float64 {
	var result []float64
	buf := bytes.NewReader(b)
	for buf.Len() > 0 {
		var f float64
		binary.Read(buf, binary.LittleEndian, &f)
		result = append(result, f)
	}
	return result
}
