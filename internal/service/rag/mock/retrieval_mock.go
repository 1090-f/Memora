// Package mock 提供成员一对外可注入的确定性 Mock，供成员三在真实服务就绪前联调。
// Mock 不访问数据库和模型，只用于联调，禁止在生产代码路径引用。
package mock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// RetrievalResultOption 控制 Mock 返回行为。
type RetrievalResultOption struct {
	// AlwaysInsufficient 为 true 时无论查询内容一律返回知识不足。
	AlwaysInsufficient bool
	// ItemCount 覆盖默认结果数量，0 表示使用默认 3 条。
	ItemCount int
	// FixedScore 覆盖每条结果的分数，0 表示使用默认分数。
	FixedScore float64
}

// RetrievalMock 是 contracts.RetrievalService 的确定性 Mock 实现。
// 命中关键字 "insufficient" 或启用 AlwaysInsufficient 时返回 knowledge_status=insufficient，
// 否则返回固定 3 条可预测结果，便于成员三编写联调断言。
type RetrievalMock struct {
	options RetrievalResultOption
	now     func() time.Time
}

// NewRetrievalMock 构造 Mock。now 用于注入时间，nil 时使用 time.Now。
func NewRetrievalMock(opts RetrievalResultOption) *RetrievalMock {
	return &RetrievalMock{options: opts, now: time.Now}
}

// Retrieve 实现 contracts.RetrievalService.Retrieve。
func (m *RetrievalMock) Retrieve(ctx context.Context, request contracts.RetrievalRequest) (contracts.RetrievalResult, error) {
	if request.Query == "" {
		return contracts.RetrievalResult{}, fmt.Errorf("query 不能为空")
	}
	if request.UserID == "" || request.KnowledgeBaseID == "" {
		return contracts.RetrievalResult{}, fmt.Errorf("user_id 和 knowledge_base_id 不能为空")
	}

	now := m.now()
	result := contracts.RetrievalResult{
		Query:           request.Query,
		Mode:            request.Mode,
		RewrittenQuery:  request.Query,
		KnowledgeStatus: "sufficient",
		ElapsedMS:       3,
	}

	// 命中 insufficient 关键词或强制开关时，返回知识不足状态以便联调该分支。
	if m.options.AlwaysInsufficient || containsInsufficient(request.Query) {
		result.KnowledgeStatus = "insufficient"
		return result, nil
	}

	// 数量与分数支持覆盖；默认固定为 3 条、0.91 分，保证结果可预测。
	count := m.options.ItemCount
	if count <= 0 {
		count = 3
	}
	score := m.options.FixedScore
	if score <= 0 {
		score = 0.91
	}
	result.Items = make([]contracts.RetrievalItem, 0, count)
	// 逐条生成可预测结果：三种分数各自递减，模拟真实检索的分层排序。
	for i := 0; i < count; i++ {
		rank := i + 1
		keywordRank := rank
		vectorRank := rank
		rrfRank := rank
		finalRank := rank
		keywordScore := score - float64(i)*0.01
		vectorScore := score - float64(i)*0.015
		rerankerScore := score - float64(i)*0.02
		content := fmt.Sprintf("Mock 检索到的第 %d 条内容，与查询「%s」相关。", i+1, request.Query)
		item := contracts.RetrievalItem{
			DocumentID:        contracts.ID(fmt.Sprintf("doc-%d", i+1)),
			DocumentTitle:     fmt.Sprintf("文档 %d", i+1),
			ChunkID:           contracts.ID(fmt.Sprintf("chunk-%d", i+1)),
			Content:           content,
			Score:             rerankerScore,
			KeywordScore:      &keywordScore,
			VectorScore:       &vectorScore,
			KeywordRank:       &keywordRank,
			VectorRank:        &vectorRank,
			RRFRank:           &rrfRank,
			RerankerScore:     &rerankerScore,
			FinalRank:         &finalRank,
			IndexVersion:      1,
			DocumentUpdatedAt: &now,
			Citation: contracts.Citation{
				SourceType:        contracts.CitationKnowledge,
				KnowledgeBaseID:   request.KnowledgeBaseID,
				DocumentID:        contracts.ID(fmt.Sprintf("doc-%d", i+1)),
				DocumentTitle:     fmt.Sprintf("文档 %d", i+1),
				ChunkID:           contracts.ID(fmt.Sprintf("chunk-%d", i+1)),
				QuotedText:        content,
				SourceLocation:    map[string]any{"section": "mock"},
				DocumentUpdatedAt: &now,
			},
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// containsInsufficient 大小写不敏感地判断查询是否包含 insufficient 关键词。
func containsInsufficient(query string) bool {
	return strings.Contains(strings.ToLower(query), "insufficient")
}

// contains 判断字符串 s 是否包含子串 substr。
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
