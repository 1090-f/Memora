package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// documentChunkRepository 是 DocumentChunkRepository 接口的 GORM 实现。
type documentChunkRepository struct{ db *gorm.DB }

// NewDocumentChunkRepository 创建一个新的文档分块仓储实例。
func NewDocumentChunkRepository(db *gorm.DB) DocumentChunkRepository {
	return &documentChunkRepository{db: db}
}

// BatchInsert 在短事务中批量插入 document_chunks。
// 同文档同版本同 chunk_no 冲突时跳过（DoNothing），返回各 Chunk 的实际 ID
// （冲突行返回空字符串，顺序与输入一致）。
func (r *documentChunkRepository) BatchInsert(ctx context.Context, chunks []*entity.DocumentChunk) ([]string, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	ids := make([]string, len(chunks))
	err := dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, chunk := range chunks {
			// 唯一键(document_id, index_version, chunk_no)冲突时跳过插入，保证重复导入幂等。
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "document_id"}, {Name: "index_version"}, {Name: "chunk_no"}},
				DoNothing: true,
			}).Create(chunk)
			if result.Error != nil {
				return fmt.Errorf("批量插入文档分块失败: %w", result.Error)
			}
			// 冲突被跳过时 RowsAffected 为 0，不记录该 ID（返回数组仍保持与输入一致的顺序）。
			if result.RowsAffected > 0 {
				ids[i] = chunk.ID
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// DeleteByVersion 删除文档指定索引版本的全部 Chunk。
func (r *documentChunkRepository) DeleteByVersion(ctx context.Context, documentID string, indexVersion int) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("document_id = ? AND index_version = ?", documentID, indexVersion).
		Delete(&entity.DocumentChunk{}).Error
	if err != nil {
		return fmt.Errorf("删除文档分块失败: %w", err)
	}
	return nil
}

// ReadActive 只读取当前用户、知识库内已发布活动索引版本的 Chunk。
// 可见性由 active_index_version 控制，重建失败不影响旧版本内容的读取。
func (r *documentChunkRepository) ReadActive(ctx context.Context, userID, kbID, documentID, section string, fromChunk, limit int) ([]DocumentReadChunk, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	query := dbFromContext(ctx, r.db).WithContext(ctx).Table("document_chunks dc").
		Select(`d.id AS document_id, d.knowledge_base_id, d.title AS document_title,
			d.updated_at AS document_updated_at, dc.index_version, dc.id AS chunk_id,
			dc.chunk_no, dc.content, COALESCE(dc.context_title, '') AS context_title,
			dc.source_location`).
		Joins("JOIN documents d ON d.id = dc.document_id").
		Where(`d.id = ? AND d.user_id = ? AND d.knowledge_base_id = ?
			AND d.deleted_at IS NULL
			AND d.active_index_version IS NOT NULL
			AND dc.index_version = d.active_index_version AND dc.chunk_no >= ?`, documentID, userID, kbID, fromChunk)
	if value := strings.TrimSpace(section); value != "" {
		query = query.Where("COALESCE(dc.context_title, '') ILIKE ?", "%"+value+"%")
	}
	var chunks []DocumentReadChunk
	if err := query.Order("dc.chunk_no ASC").Limit(limit).Scan(&chunks).Error; err != nil {
		return nil, fmt.Errorf("读取文档 Chunk 失败: %w", err)
	}
	if len(chunks) == 0 {
		var count int64
		err := dbFromContext(ctx, r.db).WithContext(ctx).Table("documents").
			Where("id = ? AND user_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL AND active_index_version IS NOT NULL", documentID, userID, kbID).
			Count(&count).Error
		if err != nil {
			return nil, fmt.Errorf("校验文档读取权限失败: %w", err)
		}
		if count == 0 {
			return nil, ErrDocumentNotFound
		}
	}
	return chunks, nil
}

// ListIndexVersions 从 Chunk/Vector 表聚合版本，不创建额外版本表。
func (r *documentChunkRepository) ListIndexVersions(ctx context.Context, userID, documentID string) ([]DocumentIndexVersion, error) {
	var exists int64
	if err := dbFromContext(ctx, r.db).WithContext(ctx).Table("documents").
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", documentID, userID).Count(&exists).Error; err != nil {
		return nil, fmt.Errorf("校验文档索引版本权限失败: %w", err)
	}
	if exists == 0 {
		return nil, ErrDocumentNotFound
	}
	var versions []DocumentIndexVersion
	err := dbFromContext(ctx, r.db).WithContext(ctx).Raw(`
		SELECT dc.index_version AS version, COUNT(DISTINCT dc.id) AS chunk_count,
		       COUNT(DISTINCT dv.id) AS vector_count,
		       CASE WHEN dc.index_version = d.active_index_version THEN 'active' ELSE 'inactive' END AS status,
		       MIN(dc.created_at) AS created_at
		FROM documents d
		JOIN document_chunks dc ON dc.document_id = d.id
		LEFT JOIN document_vectors dv ON dv.document_id = d.id AND dv.index_version = dc.index_version
		WHERE d.id = ? AND d.user_id = ? AND d.deleted_at IS NULL
		GROUP BY dc.index_version, d.active_index_version
		ORDER BY dc.index_version DESC`, documentID, userID).Scan(&versions).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询文档索引版本失败: %w", err)
	}
	return versions, nil
}
