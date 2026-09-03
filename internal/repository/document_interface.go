package repository

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
)

// DocumentReadChunk 是受所有权和活动索引约束的只读 Chunk 投影。
type DocumentReadChunk struct {
	DocumentID        string
	KnowledgeBaseID   string
	DocumentTitle     string
	DocumentUpdatedAt time.Time
	IndexVersion      int
	ChunkID           string
	ChunkNo           int
	Content           string
	ContextTitle      string
	SourceLocation    []byte
}

// DocumentIndexVersion 是从 Chunk/Vector 事实表聚合出的索引版本视图。
type DocumentIndexVersion struct {
	Version     int
	ChunkCount  int64
	VectorCount int64
	Status      string
	CreatedAt   time.Time
}

// DocumentFilter 定义文档列表查询条件，空值表示不过滤。
type DocumentFilter struct {
	Keyword          string
	DirectoryID      *string
	ProcessingStatus *string
	IndexMode        *string
	SourceType       *string
}

// DocumentRepository 定义文档数据访问接口。
// 所有查询必须组合 user_id + knowledge_base_id（适用时）+ 软删除过滤。
type DocumentRepository interface {
	// Create 创建文档。
	Create(ctx context.Context, doc *entity.Document) error
	// FindByID 按文档 ID 与用户查询文档（跨知识库详情）。
	FindByID(ctx context.Context, userID, documentID string) (*entity.Document, error)
	// FindByIDInternal 按文档 ID 查询文档（资产签名 URL 校验后使用）。
	FindByIDInternal(ctx context.Context, documentID string) (*entity.Document, error)
	// FindByIDInKB 按文档 ID、用户与知识库查询文档。
	FindByIDInKB(ctx context.Context, userID, kbID, documentID string) (*entity.Document, error)
	// ListByKB 分页查询知识库文档列表。
	ListByKB(ctx context.Context, userID, kbID string, page, pageSize int, filter DocumentFilter) ([]*entity.Document, int64, error)
	// SoftDelete 软删除文档。
	SoftDelete(ctx context.Context, userID, documentID string) error
	// Move 更新文档所属目录；directoryID 为空时移出目录。
	Move(ctx context.Context, userID, documentID string, directoryID *string) error
	// FindBySourceHash 按用户与源哈希查询未删除文档（duplicate_policy=skip 去重）。
	FindBySourceHash(ctx context.Context, userID, kbID, sourceHash string) (*entity.Document, error)
	// UpdateProcessing 更新文档处理状态、失败信息与活动索引版本。
	UpdateProcessing(ctx context.Context, docID string, updates map[string]any) error
	// BeginIndexBuild 原子领取候选索引构建权；同一 owner 重试时复用版本，过期 owner 被接管时分配更高版本。
	BeginIndexBuild(ctx context.Context, docID, owner string, staleBefore time.Time) (int, error)
	// PublishIndexBuild 仅允许当前 owner 发布其持有的候选版本，并释放构建权。
	PublishIndexBuild(ctx context.Context, docID, owner string, indexVersion int, updates map[string]any) error
	// FailIndexBuild 仅在 owner 仍持有构建权时标记失败并释放；失去所有权的旧 Worker 不得覆盖新状态。
	FailIndexBuild(ctx context.Context, docID, owner, failureStep, failureReason string) error
	// FindByImportTask 查询导入任务关联的文档（任务包 03 创建文档后回填）。
	FindByImportTask(ctx context.Context, taskID string) (*entity.Document, error)
}

// IndexBuildCompletionRepository 是 DocumentRepository 的可选事务能力。
// 生产仓储实现它，用一个数据库事务同时发布 active index 与完成导入任务。
type IndexBuildCompletionRepository interface {
	PublishIndexBuildAndCompleteTask(ctx context.Context, docID, owner string, indexVersion int, updates map[string]any) error
}

// DocumentChunkRepository 定义文档分块数据访问接口。
type DocumentChunkRepository interface {
	// BatchInsert 在短事务中批量插入 document_chunks（同文档同版本）。
	// 返回各 Chunk 的实际 ID（冲突跳过时为空字符串，顺序与输入一致）。
	BatchInsert(ctx context.Context, chunks []*entity.DocumentChunk) ([]string, error)
	// DeleteByVersion 删除文档指定索引版本的全部 Chunk。
	DeleteByVersion(ctx context.Context, documentID string, indexVersion int) error
	// ReadActive 按用户、知识库、文档和活动索引版本连续读取 Chunk。
	ReadActive(ctx context.Context, userID, kbID, documentID, section string, fromChunk, limit int) ([]DocumentReadChunk, error)
	// ListIndexVersions 返回指定文档的版本聚合信息并强制校验所有权。
	ListIndexVersions(ctx context.Context, userID, documentID string) ([]DocumentIndexVersion, error)
	// CleanupInactive 清理已软删除文档的全部 Chunk，以及超出保留版本的旧索引 Chunk。
	// retention 为保留的旧版本数（0 表示只保留当前 active 版本）；返回删除数量。
	CleanupInactive(ctx context.Context, retention int) (int64, error)
}

// ImportTaskRepository 定义导入任务数据访问接口。
// Worker 领取必须使用 PostgreSQL 行锁（SKIP LOCKED），避免多 Worker 重复领取。
type ImportTaskRepository interface {
	// Create 创建导入任务。
	Create(ctx context.Context, task *entity.ImportTask) error
	// FindByID 按任务 ID 与用户查询任务。
	FindByID(ctx context.Context, userID, taskID string) (*entity.ImportTask, error)
	// FindByIDInternal 按任务 ID 查询任务，仅 Worker 内部使用（Worker 无用户上下文）。
	FindByIDInternal(ctx context.Context, taskID string) (*entity.ImportTask, error)
	// FindLatestByDocument 查询文档最近一次处理任务，用于关联页面状态、日志与重试。
	FindLatestByDocument(ctx context.Context, userID, documentID string) (*entity.ImportTask, error)
	// HealthSnapshot 返回低基数队列健康摘要，用于指标与健康检查。
	HealthSnapshot(ctx context.Context, stalledBefore time.Time) (ImportTaskHealthSnapshot, error)
	// ListByKB 分页查询知识库导入任务。
	ListByKB(ctx context.Context, userID, kbID string, page, pageSize int) ([]*entity.ImportTask, int64, error)
	// UpdateObjectInfo 更新任务的 MinIO 对象信息与源哈希。
	UpdateObjectInfo(ctx context.Context, userID, taskID string, bucket, objectKey string, sourceHash *string) error
	// UpdateObjectInfoNoEnqueue 更新对象信息但不入队（Markdown/ZIP 待用户确认补传图片）。
	UpdateObjectInfoNoEnqueue(ctx context.Context, userID, taskID string, bucket, objectKey string, sourceHash *string) error
	// StartPendingTask 将 pending 任务入队（显式触发 Worker 处理）。
	StartPendingTask(ctx context.Context, userID, taskID string) error
	// UpdateURLResult 保存 Worker 安全抓取后的最终 URL 与正文哈希。
	UpdateURLResult(ctx context.Context, taskID, finalURL, sourceHash string) error
	// UpdateAttachments 保存 zip 导入的附件映射（相对路径 → MinIO object key）。
	UpdateAttachments(ctx context.Context, userID, taskID string, attachments map[string]string) error
	// AttachDocument 在长处理开始前持久化任务与文档关联，保证失败重试复用同一文档。
	AttachDocument(ctx context.Context, taskID, documentID string) error
	// ReservePending 使用 FOR UPDATE SKIP LOCKED 领取一个 pending 任务并置为 running。
	// 无可用任务时返回 nil, nil。
	ReservePending(ctx context.Context) (*entity.ImportTask, error)
	// ClaimPendingByID 原子认领 Redis Stream 指定的 pending 任务。
	ClaimPendingByID(ctx context.Context, taskID string) (*entity.ImportTask, error)
	// RecoverStale 恢复卡在 running 且超过租约时间的任务为 pending。
	// 返回恢复数量。
	RecoverStale(ctx context.Context, staleBefore int64) (int64, error)
	// CompleteSucceeded 将任务标记为 succeeded 并记录完成时间。
	CompleteSucceeded(ctx context.Context, taskID string, documentID *string) error
	// FailTask 将任务标记为 failed 并记录失败原因。
	FailTask(ctx context.Context, taskID string, failureReason string) error
	// RetryTask 将 failed 任务重置为 pending（显式重试），清空失败原因。
	RetryTask(ctx context.Context, taskID string) error
	// RequeueTask 将运行失败但仍可重试的任务重新排队，并生成 Outbox 事件。
	RequeueTask(ctx context.Context, taskID, failureReason string) error
	// SetRunningStep 更新 running 任务的当前步骤（心跳式进度更新）。
	SetRunningStep(ctx context.Context, taskID, step string) error
	// SkipTask 将任务标记为 skipped（去重策略命中）。
	SkipTask(ctx context.Context, taskID string) error
	// Delete 物理删除任务记录（仅用于上传失败回滚）。
	Delete(ctx context.Context, userID, taskID string) error
	// DeleteCompletedByKB 清理知识库内已结束（succeeded/skipped/failed）的任务，
	// 保留 pending/running 的进行中任务；返回删除数量。
	DeleteCompletedByKB(ctx context.Context, userID, kbID string) (int64, error)
}

type ImportTaskHealthSnapshot struct {
	Pending                 int64
	Running                 int64
	Failed                  int64
	Retried                 int64
	Stalled                 int64
	OldestPendingAgeSeconds int64
}

// TaskOutboxRepository 管理可靠 Redis Stream 发布事件。
type TaskOutboxRepository interface {
	ListUnpublished(ctx context.Context, limit int) ([]*entity.TaskOutbox, error)
	CountUnpublished(ctx context.Context) (int64, error)
	MarkPublished(ctx context.Context, eventID string) error
}
