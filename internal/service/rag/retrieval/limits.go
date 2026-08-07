package retrieval

import "time"

// 检索共享上限（集中定义，防止恶意超大参数导致资源耗尽）。
const (
	// MaxScopeDocumentIDs 文档范围过滤最大数量（关键词/向量检索共用）。
	MaxScopeDocumentIDs = 200
	// MaxKeywordTopK 关键词检索最大返回数。
	MaxKeywordTopK = 100
	// MaxVectorTopK 向量检索最大返回数。
	MaxVectorTopK = 100
	// vectorEmbedTimeout 查询向量化超时。
	vectorEmbedTimeout = 30 * time.Second
)
