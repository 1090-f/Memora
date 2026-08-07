package transformer

// DocMeta 是文档加工所需的稳定业务元数据。
// 元数据注入由 pipeline 的 load Lambda 节点完成（injectMeta），
// 本结构作为跨层传递载体。
type DocMeta struct {
	// UserID 租户用户 ID。
	UserID string
	// KnowledgeBaseID 知识库 ID。
	KnowledgeBaseID string
	// DocumentID 文档 ID。
	DocumentID string
	// IndexVersion 索引版本号。
	IndexVersion int
	// ContentVersion 内容版本号。
	ContentVersion int
	// ChunkVersion 分段版本号。
	ChunkVersion int
	// DocumentTitle 文档标题。
	DocumentTitle string
	// SourceLocation 文档来源位置信息。
	SourceLocation map[string]any
}
