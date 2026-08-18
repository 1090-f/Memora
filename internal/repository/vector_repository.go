package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// VectorHit 是一次向量检索命中的 Chunk 结果。
type VectorHit struct {
	ChunkID        string
	DocumentID     string
	DocumentTitle  string
	DirectoryID    *string
	Content        string
	SourceLocation []byte
	Score          float64
	IndexVersion   int
	UpdatedAt      time.Time
}

// VectorSearchParams 是向量检索参数。
type VectorSearchParams struct {
	UserID          string
	KnowledgeBaseID string
	DocumentIDs     []string
	QueryVector     []float32
	TopK            int
	ScoreThreshold  *float64
	// EmbeddingModelID 查询向量使用的模型配置 ID，检索只命中同模型生成的向量。
	EmbeddingModelID string
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
	// CleanupInactive 清理已软删除文档的全部向量，以及超出保留版本的旧索引向量。
	// retention 为保留的旧版本数（0 表示只保留当前 active 版本）；返回删除数量。
	CleanupInactive(ctx context.Context, retention int) (int64, error)
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
	err := dbFromContext(ctx, r.db).WithContext(ctx).Table("document_vectors").
		Where("document_id = ? AND index_version = ? AND status = 'pending'", documentID, indexVersion).
		Update("status", "failed").Error
	if err != nil {
		return fmt.Errorf("标记向量失败: %w", err)
	}
	return nil
}

// SearchCosine 执行 cosine 相似度检索。
// 只返回 status='ready'、与查询向量同 embedding 模型的向量，且 index_version = active_index_version。
// 可见性由 active 索引版本控制，不依赖 processing_status，重建失败不影响旧版本。
func (r *vectorRepository) SearchCosine(ctx context.Context, params VectorSearchParams) ([]*VectorHit, error) {
	if len(params.QueryVector) == 0 || params.TopK <= 0 {
		return nil, nil
	}
	queryVector := pgvector.NewVector(params.QueryVector)
	// args 严格按 SQL 占位符出现顺序构造。
	args := []any{queryVector} // SELECT 1 - (embedding <=> ?)
	// 归属过滤(user_id + knowledge_base_id) + 状态过滤 + 软删除过滤。
	// embedding_model_id 必须与查询向量模型一致：避免模型切换后不同维度向量比较报错，
	// 或同维度不同语义空间产生无意义分数；未重建文档在向量分支自然缺席，由关键词分支兜底。
	where := `dv.user_id = ? AND dv.knowledge_base_id = ? AND dv.status = 'ready' AND d.deleted_at IS NULL`
	where += ` AND dv.embedding_model_id = ?`
	where += ` AND 1 - (dv.embedding <=> ?) > 0`
	args = append(args, params.UserID, params.KnowledgeBaseID, params.EmbeddingModelID, queryVector) // WHERE user/kb/model/dist

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
		args = append(args, queryVector, *params.ScoreThreshold)
	}
	// ORDER BY 的距离占位符。
	sql := fmt.Sprintf(`
		SELECT dv.chunk_id AS chunk_id, dv.document_id AS document_id,
		       d.title AS document_title, d.directory_id, dc.content AS content, dc.source_location,
		       1 - (dv.embedding <=> ?) AS score,
		       dv.index_version AS index_version, d.updated_at AS updated_at
		FROM document_vectors dv
		JOIN documents d ON d.id = dv.document_id
		JOIN document_chunks dc ON dc.id = dv.chunk_id AND dc.index_version = dv.index_version
		WHERE %s
		ORDER BY dv.embedding <=> ? ASC
		LIMIT ?`, where)
	args = append(args, queryVector, params.TopK) // ORDER BY dist, LIMIT

	var hits []*VectorHit
	if err := dbFromContext(ctx, r.db).WithContext(ctx).Raw(sql, args...).Scan(&hits).Error; err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}
	return hits, nil
}

// DeleteByVersion 删除文档指定索引版本的向量。
func (r *vectorRepository) DeleteByVersion(ctx context.Context, documentID string, indexVersion int) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Exec("DELETE FROM document_vectors WHERE document_id = ? AND index_version = ?", documentID, indexVersion).Error
	if err != nil {
		return fmt.Errorf("删除向量失败: %w", err)
	}
	return nil
}

// CleanupInactive 清理已软删除文档的全部向量，以及超出保留版本的旧索引向量。
// 可见性只由 active_index_version 控制；仅删除 version < active_index_version - retention 的旧版本，
// 正在构建的新版本（active 为 NULL 或 active+1）不受影响。
func (r *vectorRepository) CleanupInactive(ctx context.Context, retention int) (int64, error) {
	if retention < 0 {
		retention = 0
	}
	result := dbFromContext(ctx, r.db).WithContext(ctx).Exec(`
		DELETE FROM document_vectors dv
		USING documents d
		WHERE dv.document_id = d.id
		  AND (d.deleted_at IS NOT NULL
		       OR (d.active_index_version IS NOT NULL AND dv.index_version < d.active_index_version - ?))`, retention)
	if result.Error != nil {
		return 0, fmt.Errorf("清理旧索引向量失败: %w", result.Error)
	}
	return result.RowsAffected, nil
}
