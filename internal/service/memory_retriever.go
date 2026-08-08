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
func (r *memoryRetriever) Retrieve(
	ctx context.Context,
	query contracts.MemoryQuery,
) ([]contracts.MemoryQueryResult, error) {
	// 1. 生成查询向量
	queryVector, err := r.embeddingSvc.Embed(ctx, query.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// 2. 转换为字节数组（pgvector 格式）
	queryVectorBytes := float64SliceToBytes(queryVector)

	// 3. 向量检索
	searchReq := repository.VectorSearchRequest{
		UserID:          string(query.UserID),
		KnowledgeBaseID: (*string)(&query.KnowledgeBaseID),
		QueryVector:     queryVectorBytes,
		EmbeddingDim:    len(queryVector),
		TopK:            r.config.VectorTopK,
		MinImportance:   r.config.MinImportance,
	}

	candidates, err := r.memoryRepo.SearchByVector(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	// 4. 综合排序
	ranked := r.rankCandidates(candidates, queryVector)

	// 5. 截取 TopK
	finalTopK := r.config.FinalTopK
	if query.TopK > 0 && query.TopK < finalTopK {
		finalTopK = query.TopK
	}
	if len(ranked) > finalTopK {
		ranked = ranked[:finalTopK]
	}

	// 6. 更新最后访问时间
	ids := make([]string, len(ranked))
	for i, item := range ranked {
		ids[i] = string(item.MemoryID)
	}
	if err := r.memoryRepo.UpdateLastAccessedAt(ctx, ids); err != nil {
		// 更新失败不影响返回结果
		logger.Warn("update last accessed at failed", zap.Error(err))
	}

	return ranked, nil
}

// rankCandidates 综合排序候选记忆。
func (r *memoryRetriever) rankCandidates(
	candidates []repository.VectorSearchResult,
	queryVector []float64,
) []contracts.MemoryQueryResult {
	now := time.Now()
	var results []contracts.MemoryQueryResult

	for _, cand := range candidates {
		// 计算综合得分
		_ = r.calculateScore(cand, now)

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
		return r.calculateScoreFromResult(results[i], now) > r.calculateScoreFromResult(results[j], now)
	})

	return results
}

// calculateScore 计算综合得分。
func (r *memoryRetriever) calculateScore(
	cand repository.VectorSearchResult,
	now time.Time,
) float64 {
	// 1. 相似度得分（已归一化到 0-1）
	similarityScore := cand.Similarity

	// 2. 重要性得分（已归一化到 0-1）
	importanceScore := cand.Memory.Importance

	// 3. 时间衰减得分
	timeDecayScore := r.calculateTimeDecay(cand.Memory.UpdatedAt, now)

	// 4. 加权求和
	score := r.config.SimilarityWeight*similarityScore +
		r.config.ImportanceWeight*importanceScore +
		r.config.TimeDecayWeight*timeDecayScore

	return score
}

// calculateScoreFromResult 从 MemoryQueryResult 计算综合得分。
func (r *memoryRetriever) calculateScoreFromResult(
	result contracts.MemoryQueryResult,
	now time.Time,
) float64 {
	// 1. 相似度得分
	similarityScore := result.Similarity

	// 2. 重要性得分
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
