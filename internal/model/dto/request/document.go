package request

// CreateDocumentRequest 表示手工创建只读知识文档的请求。
type CreateDocumentRequest struct {
	Title       string  `json:"title" binding:"required,max=500"`            // 文档标题，必填，最长 500 字符
	Content     *string `json:"content" binding:"omitempty,max=2000000"`     // 文档正文内容，可选，最大 2MB
	DirectoryID *string `json:"directory_id" binding:"omitempty"`            // 所属目录 ID，为空则不归入目录
	SourceType  string  `json:"source_type" binding:"required,oneof=manual"` // 文档来源类型，当前仅支持 manual（手工创建）
	SourceURL   *string `json:"source_url" binding:"omitempty,url,max=2000"` // 原始来源 URL，可选，最长 2000 字符
}

// DocumentListFilter 表示文档列表查询过滤条件。
type DocumentListFilter struct {
	Keyword          string  // 按标题关键词模糊搜索
	DirectoryID      *string // 按目录 ID 过滤
	ProcessingStatus *string // 按处理状态过滤（pending/processing/succeeded/failed）
	SourceType       *string // 按来源类型过滤（manual/file/url）
}

// ImportURLRequest 是静态网页异步导入请求；抓取由 Worker 执行。
type ImportURLRequest struct {
	URL             string  `json:"url" binding:"required,max=4096"`
	DirectoryID     *string `json:"directory_id" binding:"omitempty"`
	DuplicatePolicy string  `json:"duplicate_policy" binding:"omitempty,oneof=skip create_new"`
}
