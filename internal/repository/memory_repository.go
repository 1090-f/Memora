package repository

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrMemoryNotFound 表示未找到指定的记忆。
var ErrMemoryNotFound = errors.New("memory not found")

// memoryRepository 是 MemoryRepository 接口的 GORM 实现。
type memoryRepository struct{ db *gorm.DB }

// NewMemoryRepository 创建一个新的记忆仓储实例。
func NewMemoryRepository(db *gorm.DB) MemoryRepository {
	return &memoryRepository{db: db}
}

// Create 创建新的记忆条目。
// 如果遇到唯一约束冲突（并发精确去重），重新查询并返回已存在的记忆。
func (r *memoryRepository) Create(ctx context.Context, memory *entity.Memory) error {
	logger.Info("[记忆存储-Create] 开始存储记忆",
		zap.String("user_id", memory.UserID),
		zap.String("content", memory.Content),
		zap.String("memory_type", memory.MemoryType),
		zap.Int("embedding_dim", memory.EmbeddingDim),
		zap.String("embedding_model_id", memory.EmbeddingModelID),
	)

	err := r.db.WithContext(ctx).Create(memory).Error
	if err != nil {
		// 检查是否是唯一约束冲突（PostgreSQL 错误码 23505）
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") {
			logger.Warn("[记忆存储-Create] 唯一约束冲突，重新查询已存在的记忆",
				zap.String("content_hash", memory.ContentHash),
			)
			// 重新按哈希查询已存在的记忆
			var scopeID *string
			if memory.ScopeID != nil {
				scopeID = memory.ScopeID
			}
			existing, findErr := r.FindByContentHash(ctx, memory.UserID, memory.ContentHash, memory.ScopeType, scopeID)
			if findErr != nil {
				return fmt.Errorf("create memory: %w, and find existing: %w", err, findErr)
			}
			// 将已存在的记忆信息复制到传入的 memory 对象
			*memory = *existing
			return nil
		}
		logger.Error("[记忆存储-Create] 存储记忆失败",
			zap.Error(err),
		)
		return fmt.Errorf("create memory: %w", err)
	}

	logger.Info("[记忆存储-Create] 存储记忆成功",
		zap.String("memory_id", memory.ID),
	)
	return nil
}

// FindByID 根据 ID 和用户 ID 查找记忆。
func (r *memoryRepository) FindByID(ctx context.Context, id, userID string) (*entity.Memory, error) {
	var memory entity.Memory
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		First(&memory).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query memory: %w", err)
	}
	return &memory, nil
}

// Update 更新记忆条目。
func (r *memoryRepository) Update(ctx context.Context, memory *entity.Memory) error {
	if err := r.db.WithContext(ctx).Save(memory).Error; err != nil {
		return fmt.Errorf("update memory: %w", err)
	}
	return nil
}

// Delete 软删除记忆条目。
func (r *memoryRepository) Delete(ctx context.Context, id, userID string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&entity.Memory{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Updates(map[string]interface{}{
			"status":     "deleted",
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("delete memory: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrMemoryNotFound
	}
	return nil
}

// UpdateStatus 更新记忆状态。
func (r *memoryRepository) UpdateStatus(ctx context.Context, id, userID, status string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&entity.Memory{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("update memory status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrMemoryNotFound
	}
	return nil
}

// ListByUser 列出用户的记忆列表。
func (r *memoryRepository) ListByUser(ctx context.Context, userID string, opts ListMemoryOpts) (*ListMemoryResult, error) {
	var memories []entity.Memory
	var total int64

	query := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID)

	if opts.MemoryType != "" {
		query = query.Where("memory_type = ?", opts.MemoryType)
	}
	if opts.ScopeType != "" {
		query = query.Where("scope_type = ?", opts.ScopeType)
	}
	if opts.ScopeID != nil {
		query = query.Where("scope_id = ?", *opts.ScopeID)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	} else {
		query = query.Where("status != ?", "deleted")
	}

	// 统计总数
	if err := query.Model(&entity.Memory{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count memories: %w", err)
	}

	// 分页查询
	page, pageSize := opts.Page, opts.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&memories).Error; err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}

	return &ListMemoryResult{Items: memories, Total: total}, nil
}

// SearchByVector 使用向量相似度搜索记忆。
func (r *memoryRepository) SearchByVector(ctx context.Context, req VectorSearchRequest) ([]VectorSearchResult, error) {
	var results []VectorSearchResult

	// 构建查询，使用 pgvector 的余弦相似度操作符
	// 使用参数绑定而不是字符串拼接，避免 SQL 注入
	query := r.db.WithContext(ctx).
		Table("memories").
		Select("memories.*, 1 - (memories.embedding <=> CAST(? AS vector)) as similarity", req.QueryVector).
		Where("user_id = ? AND status = 'active' AND deleted_at IS NULL AND embedding_dim = ?", req.UserID, req.EmbeddingDim)

	// 如果指定了 EmbeddingModelID，则过滤相同模型生成的向量
	if req.EmbeddingModelID != "" {
		query = query.Where("embedding_model_id = ?", req.EmbeddingModelID)
	}

	if req.KnowledgeBaseID != nil {
		query = query.Where("(scope_type = 'user' OR (scope_type = 'knowledge_base' AND scope_id = ?))", *req.KnowledgeBaseID)
	} else {
		query = query.Where("scope_type = 'user'")
	}

	if req.MinImportance > 0 {
		query = query.Where("importance >= ?", req.MinImportance)
	}

	// 按相似度排序，取前 TopK
	if err := query.Order(gorm.Expr("memories.embedding <=> CAST(? AS vector) ASC", req.QueryVector)).Limit(req.TopK).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("vector search memories: %w", err)
	}

	return results, nil
}

// formatPgVector 将二进制向量转换为 PostgreSQL 向量格式
func formatPgVector(data []byte) string {
	// 将 []byte 转换为 []float64（little-endian IEEE 754 格式）
	vec := make([]float64, len(data)/8)
	for i := range vec {
		bits := binary.LittleEndian.Uint64(data[i*8:])
		vec[i] = math.Float64frombits(bits)
	}

	strs := make([]string, len(vec))
	for i, v := range vec {
		strs[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(strs, ",") + "]"
}

// SearchByKeyword 使用 ParadeDB pg_search 的 BM25 排序搜索记忆。
func (r *memoryRepository) SearchByKeyword(ctx context.Context, req KeywordMemorySearchRequest) ([]KeywordMemorySearchResult, error) {
	if req.Query == "" || req.TopK <= 0 {
		return nil, nil
	}

	// 构建 WHERE 条件
	where := "user_id = ? AND status = 'active' AND deleted_at IS NULL"
	args := []any{req.UserID}

	// scope 过滤
	if req.KnowledgeBaseID != nil {
		where += " AND (scope_type = 'user' OR (scope_type = 'knowledge_base' AND scope_id = ?))"
		args = append(args, *req.KnowledgeBaseID)
	} else {
		where += " AND scope_type = 'user'"
	}

	// ParadeDB 在索引侧使用固定 bigram tokenizer，应用只传规范化后的原始查询。
	where += " AND content ||| CAST(? AS text)"
	args = append(args, req.Query)
	args = append(args, req.TopK)

	sql := fmt.Sprintf(`
		SELECT *, pdb.score(id) AS score
		FROM memories
		WHERE %s
		ORDER BY pdb.score(id) DESC, id ASC
		LIMIT ?`, where)

	var results []KeywordMemorySearchResult
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("keyword search memories: %w", err)
	}
	return results, nil
}

// FindByContentHash 根据内容哈希查找记忆，用于去重。
func (r *memoryRepository) FindByContentHash(ctx context.Context, userID, contentHash, scopeType string, scopeID *string) (*entity.Memory, error) {
	var memory entity.Memory
	query := r.db.WithContext(ctx).
		Where("user_id = ? AND content_hash = ? AND scope_type = ? AND deleted_at IS NULL", userID, contentHash, scopeType)

	if scopeID != nil {
		query = query.Where("scope_id = ?", *scopeID)
	} else {
		query = query.Where("scope_id IS NULL")
	}

	err := query.First(&memory).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 未找到返回 nil，不是错误
	}
	if err != nil {
		return nil, fmt.Errorf("query memory by content hash: %w", err)
	}
	return &memory, nil
}

// UpdateLastAccessedAt 更新记忆的最后访问时间。
func (r *memoryRepository) UpdateLastAccessedAt(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&entity.Memory{}).
		Where("id IN ?", ids).
		Update("last_accessed_at", now).Error; err != nil {
		return fmt.Errorf("update last accessed at: %w", err)
	}
	return nil
}
