package response

import "time"

// DocumentResponse 表示文档详情的响应。
// ProcessingStatus 取值：pending/parsing/cleaning/chunking/embedding/keyword_indexing/succeeded/failed。
type DocumentResponse struct {
	ID                 string    `json:"id"`                             // ID 文档 ID
	KnowledgeBaseID    string    `json:"knowledge_base_id"`              // KnowledgeBaseID 所属知识库 ID
	DirectoryID        *string   `json:"directory_id,omitempty"`         // DirectoryID 所属目录 ID，为空表示未归入目录
	Title              string    `json:"title"`                          // Title 文档标题
	Content            *string   `json:"content,omitempty"`              // Content 文档正文内容，可选
	ContentFormat      string    `json:"content_format"`                 // ContentFormat 手工文档正文格式（txt/markdown）
	SourceType         string    `json:"source_type"`                    // SourceType 文档来源类型（manual/file/url）
	SourceURL          *string   `json:"source_url,omitempty"`           // SourceURL 原始来源 URL，可选
	OriginalFileName   *string   `json:"original_file_name,omitempty"`   // OriginalFileName 原始文件名，可选
	FileSize           *int64    `json:"file_size,omitempty"`            // FileSize 文件大小（字节），可选
	MIMEType           *string   `json:"mime_type,omitempty"`            // MIMEType 文件 MIME 类型，可选
	ProcessingStatus   string    `json:"processing_status"`              // ProcessingStatus 文档处理状态
	IndexMode          string    `json:"index_mode"`                     // IndexMode 当前活动索引能力（none/keyword/hybrid）
	FailureStep        *string   `json:"failure_step,omitempty"`         // FailureStep 处理失败的步骤，可选
	FailureReason      *string   `json:"failure_reason,omitempty"`       // FailureReason 处理失败原因，可选
	ParseWarnings      []string  `json:"parse_warnings,omitempty"`       // ParseWarnings 解析阶段非致命提示，可选
	ContentVersion     int       `json:"content_version"`                // ContentVersion 内容版本号
	ChunkVersion       int       `json:"chunk_version"`                  // ChunkVersion 分块版本号
	ActiveIndexVersion *int      `json:"active_index_version,omitempty"` // ActiveIndexVersion 当前生效的索引版本，可选
	ChunkStrategy      *string   `json:"chunk_strategy,omitempty"`       // ChunkStrategy 文档级分块策略覆盖
	CreatedAt          time.Time `json:"created_at"`                     // CreatedAt 创建时间
	UpdatedAt          time.Time `json:"updated_at"`                     // UpdatedAt 更新时间
}

// DocumentListItem 表示文档列表项（不返回正文）。
type DocumentListItem struct {
	ID               string    `json:"id"`                     // ID 文档 ID
	Title            string    `json:"title"`                  // Title 文档标题
	DirectoryID      *string   `json:"directory_id,omitempty"` // DirectoryID 所属目录 ID，为空表示未归入目录
	SourceType       string    `json:"source_type"`            // SourceType 文档来源类型（manual/file/url）
	ProcessingStatus string    `json:"processing_status"`      // ProcessingStatus 文档处理状态
	FailureStep      *string   `json:"failure_step,omitempty"`
	FailureReason    *string   `json:"failure_reason,omitempty"`
	IndexMode        string    `json:"index_mode"`          // IndexMode 当前活动索引能力（none/keyword/hybrid）
	FileSize         *int64    `json:"file_size,omitempty"` // FileSize 文件大小（字节），可选
	CreatedAt        time.Time `json:"created_at"`          // CreatedAt 创建时间
	UpdatedAt        time.Time `json:"updated_at"`          // UpdatedAt 更新时间
}

// DocumentList 表示文档分页列表响应。
type DocumentList struct {
	Items    []*DocumentListItem `json:"items"`     // Items 文档列表项
	Page     int                 `json:"page"`      // Page 当前页码（从 1 开始）
	PageSize int                 `json:"page_size"` // PageSize 每页条数
	Total    int64               `json:"total"`     // Total 文档总数
}

// DocumentPreviewResponse 表示从完整解析产物读取的阅读版正文。
type DocumentPreviewResponse struct {
	Content string `json:"content"` // Content 完整解析正文（Markdown 或纯文本）
	Format  string `json:"format"`  // Format 解析后的来源格式（markdown/txt/pdf/docx/xlsx/pptx/图片格式）
}

// UploadTaskItem 表示文件上传后创建的任务项。
type UploadTaskItem struct {
	TaskID               string  `json:"task_id"`               // TaskID 导入任务 ID
	FileName             string  `json:"file_name"`             // FileName 上传的文件名
	Status               string  `json:"status"`                // Status 任务状态（pending/running/succeeded/failed/skipped）
	BatchID              *string `json:"batch_id,omitempty"`    // BatchID 同一次批量上传的批次 ID
	SourcePath           *string `json:"source_path,omitempty"` // SourcePath 文件夹归档内的相对路径
	RequiresConfirmation bool    `json:"requires_confirmation"` // RequiresConfirmation 是否需要前端确认 Markdown 图片
}

// UploadSummary 汇总一次批量上传的处理结果。
type UploadSummary struct {
	Total    int `json:"total"`
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}

// UploadRejectedItem 表示文件夹归档中未创建任务的条目。
type UploadRejectedItem struct {
	SourcePath string `json:"source_path"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// UploadFilesResponse 表示文件上传响应。
type UploadFilesResponse struct {
	BatchID  string               `json:"batch_id"`
	Summary  UploadSummary        `json:"summary"`
	Tasks    []UploadTaskItem     `json:"tasks"` // Tasks 上传创建的任务列表
	Rejected []UploadRejectedItem `json:"rejected"`
}
