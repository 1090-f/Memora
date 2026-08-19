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
	Query           string
	Mode            KeywordSearchMode
	TopK            int
	// IndexVersion 为空时使用 documents.active_index_version。
	IndexVersion *int
}

// KeywordSearchMode 映射 ParadeDB 的短语、合取和析取检索操作符。
type KeywordSearchMode string

const (
	KeywordSearchExact KeywordSearchMode = "exact"
	KeywordSearchAll   KeywordSearchMode = "all"
	KeywordSearchAny   KeywordSearchMode = "any"
)

// KeywordSearchRepository 定义参数化全文检索接口。
type KeywordSearchRepository interface {
	// Search 执行关键词全文检索，返回按相关性排序的命中结果。
	Search(ctx context.Context, params KeywordSearchParams) ([]*KeywordHit, error)
}

// keywordSearchRepository 是基于 ParadeDB pg_search 的 GORM 实现。
// SQL 过滤：user_id + knowledge_base_id + 可选 document_ids + 软删除 + active 索引版本。
// 可见性完全由 active 索引版本控制，不依赖 processing_status，重建失败不影响旧版本。
type keywordSearchRepository struct {
	db *gorm.DB
}

// NewKeywordSearchRepository 创建关键词检索仓储。
func NewKeywordSearchRepository(db *gorm.DB) KeywordSearchRepository {
	return &keywordSearchRepository{db: db}
}

// Search 执行关键词全文检索。
func (r *keywordSearchRepository) Search(ctx context.Context, params KeywordSearchParams) ([]*KeywordHit, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" || params.TopK <= 0 {
		return nil, nil
	}
	operator := paradeDBKeywordOperator(params.Mode)
	args := []any{params.UserID, params.KnowledgeBaseID, query}
	// 归属过滤(user_id + knowledge_base_id) + 文档软删除过滤，防止跨库/跨用户泄漏已删内容。
	// 可检索性由索引版本控制：dc.index_version = d.active_index_version 决定发布内容；
	// active_index_version 仅在加工全部成功后原子写入，为 NULL 时自然无结果，
	// 重建失败保留旧 active 版本，旧索引继续对外可用。
	where := `dc.user_id = ? AND dc.knowledge_base_id = ? AND d.deleted_at IS NULL`
	// 左侧 content 使用迁移中配置的 ParadeDB bigram tokenizer；数据库统一完成分词。
	where += fmt.Sprintf(` AND dc.content %s CAST(? AS text)`, operator)

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
		       pdb.score(dc.id) AS score,
		       dc.index_version AS index_version, d.updated_at AS updated_at
		FROM document_chunks dc
		JOIN documents d ON d.id = dc.document_id
		WHERE %s
		ORDER BY pdb.score(dc.id) DESC, dc.id ASC
		LIMIT ?`, where)

	// BM25 分数倒序，chunk_id 作为稳定排序键。
	var hits []*KeywordHit
	if err := dbFromContext(ctx, r.db).WithContext(ctx).Raw(sql, args...).Scan(&hits).Error; err != nil {
		return nil, fmt.Errorf("关键词检索失败: %w", err)
	}
	if len(hits) > 0 {
		return hits, nil
	}

	// 标题/文件名回退单独执行，避免把普通 SQL 条件与 ParadeDB 的 pdb.score
	// 放在同一条查询中。解析器可能把姓名只保留在文档标题中，而不写入 Chunk 正文。
	titleArgs := []any{params.UserID, params.KnowledgeBaseID, strings.ToLower(query)}
	titleWhere := `dc.user_id = ? AND dc.knowledge_base_id = ? AND d.deleted_at IS NULL`
	titleWhere += ` AND position(CAST(? AS text) in lower(d.title)) > 0`
	if len(params.DocumentIDs) > 0 {
		titleWhere += fmt.Sprintf(" AND dc.document_id IN (%s)", placeholders(len(params.DocumentIDs)))
		for _, id := range params.DocumentIDs {
			titleArgs = append(titleArgs, id)
		}
	}
	if params.IndexVersion != nil {
		titleWhere += " AND dc.index_version = ?"
		titleArgs = append(titleArgs, *params.IndexVersion)
	} else {
		titleWhere += " AND dc.index_version = d.active_index_version"
	}
	titleArgs = append(titleArgs, params.TopK)
	titleSQL := fmt.Sprintf(`
		SELECT dc.id AS chunk_id, dc.document_id AS document_id, d.title AS document_title,
		       d.directory_id, dc.content AS content, dc.source_location,
		       1.0 AS score,
		       dc.index_version AS index_version, d.updated_at AS updated_at
		FROM document_chunks dc
		JOIN documents d ON d.id = dc.document_id
		WHERE %s
		ORDER BY dc.id ASC
		LIMIT ?`, titleWhere)
	if err := dbFromContext(ctx, r.db).WithContext(ctx).Raw(titleSQL, titleArgs...).Scan(&hits).Error; err != nil {
		return nil, fmt.Errorf("文档标题检索失败: %w", err)
	}
	return hits, nil
}

func paradeDBKeywordOperator(mode KeywordSearchMode) string {
	switch mode {
	case KeywordSearchExact:
		return "###"
	case KeywordSearchAll:
		return "&&&"
	default:
		return "|||"
	}
}

// placeholders 生成 n 个 "?," 占位符用于构造动态 IN 子句。
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
