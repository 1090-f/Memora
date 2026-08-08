package repository

import (
	"context"
	"fmt"

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
