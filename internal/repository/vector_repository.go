package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// VectorHit 是一次向量检索命中的 Chunk 结果。
type VectorHit struct {
	ChunkID      string
	DocumentID   string
	Content      string
	Score        float64
	IndexVersion int
	UpdatedAt    time.Time
}

// VectorSearchParams 是向量检索参数。
type VectorSearchParams struct {
	UserID          string
	KnowledgeBaseID string
	DocumentIDs     []string
	QueryVector     []float32
	TopK            int
	ScoreThreshold  *float64
	// IndexVersion 为空时使用 documents.active_index_version。
	IndexVersion *int
}

// VectorRepository 定义向量批量持久化与检索接口。
type VectorRepository interface {
	// BatchUpsert 批量写入 document_vectors（同 chunk+model+version 冲突时更新为 ready）。
	BatchUpsert(ctx context.Context, vectors []*entity.DocumentVector) (int, error)
	// MarkFailed 将指定 chunk 的向量标记为 failed。
	MarkFailed(ctx context.Context, documentID string, indexVersion int) error
	// SearchCosine 执行 cosine 相似度检索，只返回 ready 且 active 的向量。
	SearchCosine(ctx context.Context, params VectorSearchParams) ([]*VectorHit, error)
	// DeleteByVersion 删除文档指定索引版本的向量。
	DeleteByVersion(ctx context.Context, documentID string, indexVersion int) error
}

// vectorRepository 是 VectorRepository 的 GORM 实现。
type vectorRepository struct{ db *gorm.DB }

// NewVectorRepository 创建向量仓储。
func NewVectorRepository(db *gorm.DB) VectorRepository {
	return &vectorRepository{db: db}
}

// BatchUpsert 批量写入向量，同 chunk+model+version 冲突时覆盖向量与状态。
// 注意：document_vectors 表无 updated_at 列，DoUpdates 只更新存在的列。
func (r *vectorRepository) BatchUpsert(ctx context.Context, vectors []*entity.DocumentVector) (int, error) {
	if len(vectors) == 0 {
		return 0, nil
	}
	var affected int64
	err := dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chunk_id"}, {Name: "embedding_model_id"}, {Name: "index_version"}},
			DoUpdates: clause.AssignmentColumns([]string{"embedding", "embedding_dim", "status"}),
		}).Create(&vectors)
		if result.Error != nil {
			return fmt.Errorf("批量写入向量失败: %w", result.Error)
		}
		affected = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// MarkFailed 将文档指定版本的向量标记为 failed。
func (r *vectorRepository) MarkFailed(ctx context.Context, documentID string, indexVersion int) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.DocumentVector{}).
		Where("document_id = ? AND index_version = ? AND status = 'pending'", documentID, indexVersion).
		Update("status", "failed").Error
	if err != nil {
		return fmt.Errorf("标记向量失败: %w", err)
	}
	return nil
}

// SearchCosine 执行 cosine 相似度检索。
// 只返回 status='ready' 且 index_version = active_index_version 的向量。
func (r *vectorRepository) SearchCosine(ctx context.Context, params VectorSearchParams) ([]*VectorHit, error) {
	if len(params.QueryVector) == 0 || params.TopK <= 0 {
		return nil, nil
	}
	// args 严格按 SQL 占位符出现顺序构造。
	args := []any{params.QueryVector} // SELECT 1 - (embedding <=> ?)
	where := `dv.user_id = ? AND dv.knowledge_base_id = ? AND dv.status = 'ready' AND d.deleted_at IS NULL`
	where += ` AND 1 - (dv.embedding <=> ?) > 0`
	args = append(args, params.UserID, params.KnowledgeBaseID, params.QueryVector) // WHERE user/kb/dist

	if len(params.DocumentIDs) > 0 {
		where += fmt.Sprintf(" AND dv.document_id IN (%s)", placeholders(len(params.DocumentIDs)))
		for _, id := range params.DocumentIDs {
			args = append(args, id)
		}
	}
	if params.IndexVersion != nil {
		where += " AND dv.index_version = ?"
		args = append(args, *params.IndexVersion)
	} else {
		where += " AND dv.index_version = d.active_index_version"
	}
	if params.ScoreThreshold != nil {
		where += " AND 1 - (dv.embedding <=> ?) >= ?"
		args = append(args, params.QueryVector, *params.ScoreThreshold)
	}
	// ORDER BY 的距离占位符。
	sql := fmt.Sprintf(`
		SELECT dv.chunk_id AS chunk_id, dv.document_id AS document_id,
		       dc.content AS content,
		       1 - (dv.embedding <=> ?) AS score,
		       dv.index_version AS index_version, d.updated_at AS updated_at
		FROM document_vectors dv
		JOIN documents d ON d.id = dv.document_id
		JOIN document_chunks dc ON dc.id = dv.chunk_id AND dc.index_version = dv.index_version
		WHERE %s
		ORDER BY dv.embedding <=> ? ASC
		LIMIT ?`, where)
	args = append(args, params.QueryVector, params.TopK) // ORDER BY dist, LIMIT

	var hits []*VectorHit
	if err := dbFromContext(ctx, r.db).WithContext(ctx).Raw(sql, args...).Scan(&hits).Error; err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}
	return hits, nil
}

// DeleteByVersion 删除文档指定索引版本的向量。
func (r *vectorRepository) DeleteByVersion(ctx context.Context, documentID string, indexVersion int) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("document_id = ? AND index_version = ?", documentID, indexVersion).
		Delete(&entity.DocumentVector{}).Error
	if err != nil {
		return fmt.Errorf("删除向量失败: %w", err)
	}
	return nil
}
