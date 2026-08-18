package service

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// DocumentProcessService 是成员一内部的文档处理服务接口，
// 用于文档导入任务、重试、重新索引和处理状态查询。
// 长耗时解析、分段、Embedding 与索引由 Worker 执行，Service 只编排状态与校验。
type DocumentProcessService interface {
	// CreateImportTask 创建导入任务并返回任务 ID。文件已落 MinIO 时传入对象信息。
	CreateImportTask(ctx context.Context, userID, knowledgeBaseID contracts.ID, task DocumentImportTask) (contracts.ID, error)
	// Retry 显式重试失败的任务。
	Retry(ctx context.Context, userID, taskID contracts.ID) error
	// Reindex 触发文档重新索引，生成新的 index_version。
	Reindex(ctx context.Context, userID, knowledgeBaseID, documentID contracts.ID) error
	// GetProcessingStatus 查询文档处理状态（按文档 ID 与用户，无需知识库 ID）。
	GetProcessingStatus(ctx context.Context, userID, documentID contracts.ID) (DocumentProcessingStatus, error)
	// ListIndexVersions 查询文档索引版本聚合视图。
	ListIndexVersions(ctx context.Context, userID, documentID contracts.ID) ([]DocumentIndexVersionView, error)
	// ListImportTasks 分页查询知识库导入任务。
	ListImportTasks(ctx context.Context, userID, knowledgeBaseID contracts.ID, page, pageSize int) ([]ImportTaskView, int64, error)
	// GetImportTask 查询导入任务详情。
	GetImportTask(ctx context.Context, userID, taskID contracts.ID) (ImportTaskView, error)
	// ProcessImportTask 由 Worker 调用：领取后的任务编排（创建文档行并执行文档加工流水线）。
	ProcessImportTask(ctx context.Context, taskID contracts.ID) error
	// StartImportTask 显式触发 pending 任务进入解析（Markdown/ZIP 上传后默认不自动入队）。
	StartImportTask(ctx context.Context, userID, taskID contracts.ID) error
	// ScanImportTask 扫描 Markdown 任务的图片引用并分类（内联/网络/已匹配/待补传）。
	ScanImportTask(ctx context.Context, userID, taskID contracts.ID) (*ImageScanResult, error)
	// UploadTaskAttachments 向任务补传图片附件（存 MinIO 并更新引用映射）。
	UploadTaskAttachments(ctx context.Context, userID, taskID contracts.ID, files []UploadFileInput) error
	// RecoverStaleTasks 恢复卡在 running 且超过租约的任务，返回恢复数量。
	RecoverStaleTasks(ctx context.Context) (int64, error)
	// CleanupInactiveIndexes 清理已软删除文档与超出保留版本的旧索引 Chunk/向量。
	// retention 为保留的旧版本数（0 表示只保留当前 active 版本）；返回删除的向量+Chunk 数量。
	CleanupInactiveIndexes(ctx context.Context, retention int) (int64, error)
	// CleanupImportTasks 清理知识库内已结束（succeeded/skipped/failed）的导入任务，
	// 保留 pending/running；返回删除数量。
	CleanupImportTasks(ctx context.Context, userID, knowledgeBaseID contracts.ID) (int64, error)
}

// ImageRefStatus 是图片引用的扫描分类。
type ImageRefStatus string

const (
	// ImageRefInline 是 data URI 内联图片（自动接收）。
	ImageRefInline ImageRefStatus = "inline"
	// ImageRefNetwork 是网络图片（后端自动下载）。
	ImageRefNetwork ImageRefStatus = "network"
	// ImageRefMatched 是已与附件匹配的本地图片。
	ImageRefMatched ImageRefStatus = "matched"
	// ImageRefPending 是待补传的本地图片。
	ImageRefPending ImageRefStatus = "pending"
)

// ImageScanItem 是单个图片引用的扫描结果。
type ImageScanItem struct {
	Alt    string         `json:"alt"`
	Ref    string         `json:"ref"`
	Status ImageRefStatus `json:"status"`
}

// ImageScanResult 是 Markdown 图片引用扫描结果。
type ImageScanResult struct {
	Refs []ImageScanItem `json:"refs"`
}

// DocumentIndexVersionView 是文档索引版本的服务层只读视图。
type DocumentIndexVersionView struct {
	Version     int       `json:"version"`
	ChunkCount  int64     `json:"chunk_count"`
	VectorCount int64     `json:"vector_count"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// DocumentImportTask 描述一次导入任务所需的最小信息。
// 业务字段随任务包 02/03 落库后扩展，此处只冻结稳定身份与来源标识。
type DocumentImportTask struct {
	KnowledgeBaseID contracts.ID
	SourceType      contracts.DocumentSourceType
	FileName        string
	SourceURL       string
	MinIOBucket     string
	MinIOObjectKey  string
	FileSize        int64
	MIMEType        string
}

// DocumentProcessingStatus 描述文档处理当前状态。
type DocumentProcessingStatus struct {
	DocumentID      contracts.ID
	KnowledgeBaseID contracts.ID
	Status          contracts.DocumentProcessingStatus
	CurrentStep     string
	FailureReason   string
	IndexVersion    int
	ActiveVersion   int
}

// ImportTaskView 描述导入任务的对外视图。
type ImportTaskView struct {
	ID            contracts.ID
	BatchID       *string
	SourcePath    *string
	SourceType    contracts.DocumentSourceType
	FileName      *string
	FileSize      *int64
	MIMEType      *string
	SourceURL     *string
	Status        contracts.ImportTaskStatus
	CurrentStep   *string
	FailureReason *string
	DocumentID    *contracts.ID
	CreatedAt     time.Time
	CompletedAt   *time.Time
}
