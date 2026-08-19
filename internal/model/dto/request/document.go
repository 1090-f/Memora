package request

// DocumentListFilter 表示文档列表查询过滤条件。
type DocumentListFilter struct {
	Keyword          string  // 按标题关键词模糊搜索
	DirectoryID      *string // 按目录 ID 过滤
	ProcessingStatus *string // 按处理状态过滤（pending/processing/succeeded/failed）
	IndexMode        *string // 按活动索引能力过滤（none/keyword/hybrid）
	SourceType       *string // 按来源类型过滤（manual/file/url）
}

// MoveDocumentRequest 表示变更文档所属目录的请求；空目录表示移出目录。
type MoveDocumentRequest struct {
	DirectoryID *string `json:"directory_id"`
}

// ImportURLRequest 是静态网页异步导入请求；抓取由 Worker 执行。
// 重复处理策略读取知识库级配置，请求不再携带。
type ImportURLRequest struct {
	URL         string  `json:"url" binding:"required,max=4096"`
	DirectoryID *string `json:"directory_id" binding:"omitempty"`
}
