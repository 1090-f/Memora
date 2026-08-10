package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// KeywordHit 是一次关键词检索命中的 Chunk 结果。
type KeywordHit struct {
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

// KeywordSearchParams 是关键词全文检索参数。
type KeywordSearchParams struct {
	UserID          string
	KnowledgeBaseID string
	DocumentIDs     []string
	QueryTokens     []string
	TopK            int
	// IndexVersion 为空时使用 documents.active_index_version。
	IndexVersion *int
}

// KeywordSearchRepository 定义参数化全文检索接口。
type KeywordSearchRepository interface {
	// Search 执行关键词全文检索，返回按相关性排序的命中结果。
	Search(ctx context.Context, params KeywordSearchParams) ([]*KeywordHit, error)
}

// keywordSearchRepository 是 KeywordSearchRepository 的 GORM 实现。
// SQL 过滤：user_id + knowledge_base_id + 可选 document_ids + 软删除 + active 索引版本。
type keywordSearchRepository struct {
	db *gorm.DB
}

// NewKeywordSearchRepository 创建关键词检索仓储。
func NewKeywordSearchRepository(db *gorm.DB) KeywordSearchRepository {
	return &keywordSearchRepository{db: db}
}

// Search 执行关键词全文检索。
func (r *keywordSearchRepository) Search(ctx context.Context, params KeywordSearchParams) ([]*KeywordHit, error) {
	if len(params.QueryTokens) == 0 || params.TopK <= 0 {
		return nil, nil
	}
	// 用 OR 连接 Token 构建 tsquery，提升召回。
	query := strings.Join(params.QueryTokens, " | ")
	args := []any{query}
	// 归属过滤(user_id + knowledge_base_id) + 文档软删除过滤，防止跨库/跨用户泄漏已删内容。
	where := `dc.user_id = ? AND dc.knowledge_base_id = ? AND d.deleted_at IS NULL AND d.processing_status = 'succeeded'`
	// tsquery 命中 dc.fts_vector（'simple' 配置下分词后的全文索引列）。
	where += ` AND to_tsquery('simple', ?) @@ dc.fts_vector`
	args = append(args, params.UserID, params.KnowledgeBaseID, query)

	if len(params.DocumentIDs) > 0 {
		// 限定检索范围到指定文档集合，动态生成 IN 占位符并追加参数。
		where += fmt.Sprintf(" AND dc.document_id IN (%s)", placeholders(len(params.DocumentIDs)))
		for _, id := range params.DocumentIDs {
			args = append(args, id)
		}
	}
	// 未显式指定索引版本时，只检索文档当前活跃版本，避免命中历史失效分块。
	if params.IndexVersion != nil {
		where += " AND dc.index_version = ?"
		args = append(args, *params.IndexVersion)
	} else {
		where += " AND dc.index_version = d.active_index_version"
	}
	args = append(args, params.TopK)

	sql := fmt.Sprintf(`
		SELECT dc.id AS chunk_id, dc.document_id AS document_id, d.title AS document_title,
		       d.directory_id, dc.content AS content, dc.source_location,
		       ts_rank(dc.fts_vector, to_tsquery('simple', ?)) AS score,
		       dc.index_version AS index_version, d.updated_at AS updated_at
		FROM document_chunks dc
		JOIN documents d ON d.id = dc.document_id
		WHERE %s
		ORDER BY score DESC, dc.id ASC
		LIMIT ?`, where)

	// ts_rank 计算相关度并按分数倒序实现"按相关性排序"，chunk_id 作为稳定排序键。
	var hits []*KeywordHit
	if err := dbFromContext(ctx, r.db).WithContext(ctx).Raw(sql, args...).Scan(&hits).Error; err != nil {
		return nil, fmt.Errorf("关键词检索失败: %w", err)
	}
	return hits, nil
}

// placeholders 生成 n 个 "?," 占位符用于构造动态 IN 子句。
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
