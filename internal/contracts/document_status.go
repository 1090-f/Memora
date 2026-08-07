package contracts

// DocumentSourceType 表示文档来源类型，与 documents.source_type 枚举一致。
type DocumentSourceType string

const (
	// DocumentSourceManual 手工创建文档。
	DocumentSourceManual DocumentSourceType = "manual"
	// DocumentSourceFile 文件上传导入。
	DocumentSourceFile DocumentSourceType = "file"
	// DocumentSourceURL URL 导入。
	DocumentSourceURL DocumentSourceType = "url"
)

// DocumentProcessingStatus 表示文档处理管线状态，与 documents.processing_status 枚举一致。
type DocumentProcessingStatus string

const (
	// ProcessingPending 等待处理。
	ProcessingPending DocumentProcessingStatus = "pending"
	// ProcessingParsing 正在解析。
	ProcessingParsing DocumentProcessingStatus = "parsing"
	// ProcessingCleaning 正在清洗。
	ProcessingCleaning DocumentProcessingStatus = "cleaning"
	// ProcessingChunking 正在分段。
	ProcessingChunking DocumentProcessingStatus = "chunking"
	// ProcessingEmbedding 正在生成向量。
	ProcessingEmbedding DocumentProcessingStatus = "embedding"
	// ProcessingKeywordIndexing 正在构建关键词索引。
	ProcessingKeywordIndexing DocumentProcessingStatus = "keyword_indexing"
	// ProcessingSucceeded 处理完成。
	ProcessingSucceeded DocumentProcessingStatus = "succeeded"
	// ProcessingFailed 处理失败。
	ProcessingFailed DocumentProcessingStatus = "failed"
)

// ImportTaskStatus 表示导入任务状态，与 import_tasks.status 枚举一致。
type ImportTaskStatus string

const (
	// TaskStatusPending 等待处理。
	TaskStatusPending ImportTaskStatus = "pending"
	// TaskStatusRunning 正在处理。
	TaskStatusRunning ImportTaskStatus = "running"
	// TaskStatusSucceeded 处理成功。
	TaskStatusSucceeded ImportTaskStatus = "succeeded"
	// TaskStatusFailed 处理失败。
	TaskStatusFailed ImportTaskStatus = "failed"
	// TaskStatusSkipped 已跳过（重复策略）。
	TaskStatusSkipped ImportTaskStatus = "skipped"
)
