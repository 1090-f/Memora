package contracts

import "context"

// RetrievalMode 检索方式。
type RetrievalMode string

// 检索模式常量。
const (
	RetrievalKeyword RetrievalMode = "keyword"  // 仅关键词检索
	RetrievalVector  RetrievalMode = "vector"   // 仅向量检索
	RetrievalHybrid  RetrievalMode = "hybrid"   // 混合检索（关键词 + 向量 + RRF 融合）
)

// SearchConfig 是多路检索与融合的调参配置。
type SearchConfig struct {
	KeywordTopK            int      `json:"keyword_top_k"`            // 关键词检索返回条数
	VectorTopK             int      `json:"vector_top_k"`             // 向量检索返回条数
	RRFK                   int      `json:"rrf_k"`                    // RRF 融合常数 k
	RRFTopK                int      `json:"rrf_top_k"`                // RRF 融合后保留条数
	RerankerTopK           int      `json:"reranker_top_k"`           // 重排后保留条数
	RerankerThreshold      *float64 `json:"reranker_threshold,omitempty"` // 可选：重排分数阈值，低于则丢弃
	MinimumEffectiveResult int      `json:"minimum_effective_results"`    // 判定"知识充足"的最小有效结果数
}

// DefaultSearchConfig 返回一套默认的检索调参。
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{KeywordTopK: 30, VectorTopK: 30, RRFK: 60, RRFTopK: 20, RerankerTopK: 8, MinimumEffectiveResult: 1}
}

// RetrievalRequest 是知识库检索请求。
type RetrievalRequest struct {
	UserID          ID            `json:"user_id"`          // 用户标识
	KnowledgeBaseID ID            `json:"knowledge_base_id"` // 知识库标识
	Query           string        `json:"query"`            // 查询文本
	Mode            RetrievalMode `json:"mode"`             // 检索模式
	DocumentIDs     []ID          `json:"document_ids,omitempty"` // 可选：限定检索的文档集合
	TopK            int           `json:"top_k"`            // 最终返回条数
	Config          SearchConfig  `json:"config"`           // 检索参数
}

// RetrievalItem 是检索命中的单个片段（chunk）。
type RetrievalItem struct {
	DocumentID   ID       `json:"document_id"`          // 所属文档 ID
	ChunkID      ID       `json:"chunk_id"`             // 片段 ID
	Content      string   `json:"content"`              // 片段内容
	Score        float64  `json:"score"`                // 融合后的最终得分
	KeywordRank  *int     `json:"keyword_rank,omitempty"` // 可选：关键词检索排名
	VectorRank   *int     `json:"vector_rank,omitempty"`  // 可选：向量检索排名
	IndexVersion int      `json:"index_version"`        // 命中使用的索引版本
	Citation     Citation `json:"citation"`             // 引用信息
}

// RetrievalResult 是检索的整体结果。
type RetrievalResult struct {
	Items           []RetrievalItem `json:"items"`                     // 命中的片段列表
	RewrittenQuery  string          `json:"rewritten_query,omitempty"` // 可选：改写后的查询
	KnowledgeStatus string          `json:"knowledge_status"`          // 知识充足性状态标识
}

// RetrievalService 抽象知识库检索能力。
type RetrievalService interface {
	// Retrieve 执行一次多路检索并返回融合结果。
	Retrieve(ctx context.Context, request RetrievalRequest) (RetrievalResult, error)
}