package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	previewservice "github.com/1090-f/Memora/internal/service/preview"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/internal/service/rag/pipeline"
	"github.com/1090-f/Memora/internal/service/rag/transformer"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"github.com/cloudwego/eino/components/embedding"
	"go.uber.org/zap"
)

// documentProcessService 是 DocumentProcessService 接口的实现。
// 任务包 04 起：创建文档行后调用 Eino 文档加工 Graph（解析/清洗/分段/落库/向量）。
type documentProcessService struct {
	tasks            repository.ImportTaskRepository
	docs             repository.DocumentRepository
	kbs              repository.KnowledgeBaseRepository
	chunks           repository.DocumentChunkRepository
	vectors          repository.VectorRepository
	processor        DocumentProcessor
	embeddings       DocumentEmbeddingResolver
	store            ObjectStore
	previewScheduler previewservice.Scheduler
}

// NewDocumentProcessService 创建一个新的文档处理服务实例。
// processor 为 nil 时退回任务包 03 行为（仅创建文档行，不执行加工）。
func NewDocumentProcessService(
	tasks repository.ImportTaskRepository,
	docs repository.DocumentRepository,
	kbs repository.KnowledgeBaseRepository,
	chunks repository.DocumentChunkRepository,
	vectors repository.VectorRepository,
	processor DocumentProcessor,
	embeddings DocumentEmbeddingResolver,
	store ObjectStore,
	previewScheduler previewservice.Scheduler,
) DocumentProcessService {
	return &documentProcessService{tasks: tasks, docs: docs, kbs: kbs, chunks: chunks, vectors: vectors, processor: processor, embeddings: embeddings, store: store, previewScheduler: previewScheduler}
}

// CreateImportTask 创建导入任务。
func (s *documentProcessService) CreateImportTask(ctx context.Context, userID, kbID contracts.ID, task DocumentImportTask) (contracts.ID, error) {
	if userID == "" || kbID == "" {
		return "", apperrors.ErrInvalidArgument
	}
	entityTask := &entity.ImportTask{
		UserID: string(userID), KnowledgeBaseID: string(kbID),
		SourceType:      string(task.SourceType),
		DuplicatePolicy: "skip",
		Status:          string(contracts.TaskStatusPending),
	}
	// 去重策略缺省为 skip：同名/同内容文件默认跳过，避免重复导入。
	// 状态初始为 pending，由 Worker 领取后置为 running 再处理。
	if task.FileName != "" {
		entityTask.FileName = &task.FileName
	}
	if task.FileSize > 0 {
		entityTask.FileSize = &task.FileSize
	}
	if task.MIMEType != "" {
		entityTask.MIMEType = &task.MIMEType
	}
	if task.SourceURL != "" {
		entityTask.SourceURL = &task.SourceURL
	}
	if task.MinIOBucket != "" {
		entityTask.MinIOBucket = &task.MinIOBucket
	}
	if task.MinIOObjectKey != "" {
		entityTask.MinIOObjectKey = &task.MinIOObjectKey
	}
	if err := s.tasks.Create(ctx, entityTask); err != nil {
		return "", apperrors.New(contracts.ErrInternal, err)
	}
	return contracts.ID(entityTask.ID), nil
}

// Retry 显式重试失败的任务（failed → pending）。
func (s *documentProcessService) Retry(ctx context.Context, userID, taskID contracts.ID) error {
	task, err := s.tasks.FindByID(ctx, string(userID), string(taskID))
	if errors.Is(err, repository.ErrImportTaskNotFound) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	if task.Status != string(contracts.TaskStatusFailed) {
		return apperrors.New(contracts.ErrInvalidState, fmt.Errorf("任务状态 %q 不允许重试", task.Status))
	}
	if err := s.tasks.RetryTask(ctx, string(taskID)); err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("导入任务已重试", zap.String("user_id", string(userID)), zap.String("task_id", string(taskID)))
	return nil
}

// StartImportTask 显式触发 pending 任务进入解析（Markdown/ZIP 上传后默认不自动入队）。
func (s *documentProcessService) StartImportTask(ctx context.Context, userID, taskID contracts.ID) error {
	task, err := s.tasks.FindByID(ctx, string(userID), string(taskID))
	if errors.Is(err, repository.ErrImportTaskNotFound) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	if task.Status != string(contracts.TaskStatusPending) {
		return apperrors.New(contracts.ErrInvalidState, fmt.Errorf("任务状态 %q 不允许开始", task.Status))
	}
	if task.MinIOObjectKey == nil || *task.MinIOObjectKey == "" {
		return apperrors.New(contracts.ErrInvalidState, fmt.Errorf("任务尚未完成上传"))
	}
	if err := s.tasks.StartPendingTask(ctx, string(userID), string(taskID)); err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("导入任务已开始解析", zap.String("user_id", string(userID)), zap.String("task_id", string(taskID)))
	return nil
}

// ScanImportTask 读取任务原始 Markdown（或 ZIP 内主文档），扫描图片引用并分类：
// data URI → inline；http(s) → network；附件映射命中（精确或 basename）→ matched；其余 → pending。
func (s *documentProcessService) ScanImportTask(ctx context.Context, userID, taskID contracts.ID) (*ImageScanResult, error) {
	task, err := s.tasks.FindByID(ctx, string(userID), string(taskID))
	if errors.Is(err, repository.ErrImportTaskNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if task.MinIOObjectKey == nil || *task.MinIOObjectKey == "" {
		return nil, apperrors.New(contracts.ErrInvalidState, fmt.Errorf("任务尚未完成上传"))
	}
	if s.store == nil {
		return nil, apperrors.New(contracts.ErrInternal, fmt.Errorf("对象存储未配置"))
	}
	content, err := s.readTaskMarkdown(ctx, task)
	if err != nil {
		return nil, err
	}
	result := &ImageScanResult{Refs: make([]ImageScanItem, 0, 8)}
	seen := make(map[string]bool)
	for _, ref := range parser.ScanMarkdownImageRefs(content) {
		if seen[ref.Ref] {
			continue
		}
		seen[ref.Ref] = true
		result.Refs = append(result.Refs, ImageScanItem{Alt: ref.Alt, Ref: ref.Ref, Status: classifyImageRef(ref.Ref, task.Attachments)})
	}
	return result, nil
}

// readTaskMarkdown 读取任务的 Markdown 正文：普通文件直接读；ZIP 解包取第一个 Markdown 主文档。
func (s *documentProcessService) readTaskMarkdown(ctx context.Context, task *entity.ImportTask) (string, error) {
	reader, err := s.store.OpenObject(ctx, *task.MinIOObjectKey)
	if err != nil {
		return "", apperrors.New(contracts.ErrInternal, fmt.Errorf("读取原始文件失败: %w", err))
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, 64*1024*1024+1))
	if err != nil {
		return "", apperrors.New(contracts.ErrInternal, fmt.Errorf("读取原始文件失败: %w", err))
	}
	fileName := valueOrEmpty(task.FileName)
	if strings.EqualFold(path.Ext(fileName), ".zip") {
		zipReader, zipErr := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if zipErr != nil {
			return "", apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("ZIP 解包失败: %w", zipErr))
		}
		for _, entry := range zipReader.File {
			name := safeZipPath(entry.Name)
			if name == "" || !isMainDocumentExt(strings.ToLower(path.Ext(name))) {
				continue
			}
			entryReader, openErr := entry.Open()
			if openErr != nil {
				return "", apperrors.New(contracts.ErrInternal, fmt.Errorf("读取 ZIP 条目失败: %w", openErr))
			}
			entryData, readErr := io.ReadAll(io.LimitReader(entryReader, 64*1024*1024+1))
			_ = entryReader.Close()
			if readErr != nil {
				return "", apperrors.New(contracts.ErrInternal, fmt.Errorf("读取 ZIP 条目失败: %w", readErr))
			}
			return string(entryData), nil
		}
		return "", apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("ZIP 内没有 Markdown 主文档"))
	}
	return string(data), nil
}

// classifyImageRef 分类单个图片引用。
func classifyImageRef(ref string, attachments map[string]string) ImageRefStatus {
	lower := strings.ToLower(ref)
	switch {
	case strings.HasPrefix(lower, "data:image/"):
		return ImageRefInline
	case strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://"):
		return ImageRefNetwork
	}
	if attachments != nil {
		if _, ok := attachments[path.Clean(ref)]; ok {
			return ImageRefMatched
		}
		base := attachmentBaseName(ref)
		for attachmentPath := range attachments {
			if attachmentBaseName(attachmentPath) == base {
				return ImageRefMatched
			}
		}
	}
	return ImageRefPending
}

// attachmentBaseName 返回路径最后一段，兼容 / 与 \ 分隔符。
func attachmentBaseName(p string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		return normalized[idx+1:]
	}
	return normalized
}

// UploadTaskAttachments 向任务补传图片附件：上传 MinIO（key 含附件前缀），
// 按文件名（basename）合并进引用映射，供 Markdown 图片引用匹配。
func (s *documentProcessService) UploadTaskAttachments(ctx context.Context, userID, taskID contracts.ID, files []UploadFileInput) error {
	if len(files) == 0 {
		return apperrors.ErrInvalidArgument
	}
	task, err := s.tasks.FindByID(ctx, string(userID), string(taskID))
	if errors.Is(err, repository.ErrImportTaskNotFound) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	if task.Status == string(contracts.TaskStatusSucceeded) || task.Status == string(contracts.TaskStatusFailed) {
		return apperrors.New(contracts.ErrInvalidState, fmt.Errorf("任务已结束，无法补传附件"))
	}
	if s.store == nil {
		return apperrors.New(contracts.ErrInternal, fmt.Errorf("对象存储未配置"))
	}
	attachments := make(map[string]string, len(files))
	uploadedKeys := make([]string, 0, len(files))
	for _, file := range files {
		ext := strings.ToLower(path.Ext(file.FileName))
		if !isImageExt(ext) {
			return apperrors.New(contracts.ErrUnsupportedFileType, fmt.Errorf("仅支持图片附件: %q", file.FileName))
		}
		if file.Size <= 0 || file.Size > 32*1024*1024 {
			return apperrors.New(contracts.ErrPayloadTooLarge, nil)
		}
		key := objectstore.BuildObjectKey(string(userID), task.KnowledgeBaseID, string(taskID), "attachments/"+file.FileName)
		if err := s.store.PutObject(ctx, key, file.Reader, file.Size, mimeTypeOf(ext)); err != nil {
			s.removeUploadedObjects(ctx, uploadedKeys)
			return apperrors.New(contracts.ErrServiceUnavailable, err)
		}
		uploadedKeys = append(uploadedKeys, key)
		attachments[file.FileName] = key
	}
	merged := make(map[string]string, len(task.Attachments)+len(attachments))
	for k, v := range task.Attachments {
		merged[k] = v
	}
	for k, v := range attachments {
		merged[k] = v
	}
	if err := s.tasks.UpdateAttachments(ctx, string(userID), string(taskID), merged); err != nil {
		s.removeUploadedObjects(ctx, uploadedKeys)
		return apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("任务附件补传完成", zap.String("user_id", string(userID)), zap.String("task_id", string(taskID)), zap.Int("count", len(files)))
	return nil
}

// removeUploadedObjects 批量删除已上传附件对象（补偿用）。
func (s *documentProcessService) removeUploadedObjects(ctx context.Context, keys []string) {
	for _, key := range keys {
		if err := s.store.RemoveObject(ctx, key); err != nil {
			logger.Error("补偿删除附件对象失败", zap.String("object_key", key), zap.Error(err))
		}
	}
}

// Reindex 为现有文档创建指向原文来源的新导入任务；Worker 构建新版本后原子切换活动索引。
func (s *documentProcessService) Reindex(ctx context.Context, userID, kbID, documentID contracts.ID) error {
	return s.ReindexWithStrategy(ctx, userID, kbID, documentID, nil)
}

// ReindexWithStrategy 可原子地更新文档级策略选择，并为该文档创建新索引任务。
func (s *documentProcessService) ReindexWithStrategy(ctx context.Context, userID, kbID, documentID contracts.ID, chunkStrategy *string) error {
	doc, err := s.docs.FindByIDInKB(ctx, string(userID), string(kbID), string(documentID))
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	if (doc.MinIOObjectKey == nil || *doc.MinIOObjectKey == "") && (doc.SourceURL == nil || *doc.SourceURL == "") && doc.SourceType != string(contracts.DocumentSourceManual) {
		return apperrors.New(contracts.ErrInvalidState, fmt.Errorf("文档没有可重建的原始来源"))
	}
	if chunkStrategy != nil {
		value := strings.TrimSpace(*chunkStrategy)
		var persisted any = value
		if value == "" || value == "inherit" {
			persisted = nil
			doc.ChunkStrategy = nil
		} else {
			doc.ChunkStrategy = &value
		}
		if err := s.docs.UpdateProcessing(ctx, doc.ID, map[string]any{"chunk_strategy": persisted}); err != nil {
			return apperrors.New(contracts.ErrInternal, fmt.Errorf("更新文档分块策略失败: %w", err))
		}
	}
	task := &entity.ImportTask{
		UserID: doc.UserID, KnowledgeBaseID: doc.KnowledgeBaseID, TargetDirectoryID: doc.DirectoryID,
		SourceType: doc.SourceType, FileName: doc.OriginalFileName, FileSize: doc.FileSize, MIMEType: doc.MIMEType,
		SourceURL: doc.SourceURL, SourceHash: doc.FileHash, MinIOBucket: doc.MinIOBucket, MinIOObjectKey: doc.MinIOObjectKey,
		DuplicatePolicy: "create_new", Status: string(contracts.TaskStatusPending), DocumentID: &doc.ID,
	}
	// 手工文档没有 MinIO 对象与 URL：正文存于 documents.content，任务按来源类型
	// 自动入队，Worker 读取正文后执行分块与索引。
	if doc.SourceType == string(contracts.DocumentSourceManual) {
		fileName := manualTaskFileName(doc)
		task.FileName = &fileName
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("文档重新索引任务已创建", zap.String("user_id", string(userID)), zap.String("document_id", string(documentID)), zap.String("task_id", task.ID))
	return nil
}

// GetProcessingStatus 查询文档处理状态。
func (s *documentProcessService) GetProcessingStatus(ctx context.Context, userID, documentID contracts.ID) (DocumentProcessingStatus, error) {
	doc, err := s.docs.FindByID(ctx, string(userID), string(documentID))
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return DocumentProcessingStatus{}, apperrors.ErrNotFound
	}
	if err != nil {
		return DocumentProcessingStatus{}, apperrors.New(contracts.ErrInternal, err)
	}
	result := DocumentProcessingStatus{
		DocumentID:      contracts.ID(doc.ID),
		KnowledgeBaseID: contracts.ID(doc.KnowledgeBaseID),
		Status:          contracts.DocumentProcessingStatus(doc.ProcessingStatus),
		FailureReason:   valueOrEmpty(doc.FailureReason),
		ActiveVersion:   valueOrZero(doc.ActiveIndexVersion),
	}
	if doc.FailureStep != nil {
		result.CurrentStep = *doc.FailureStep
	}
	return result, nil
}

func (s *documentProcessService) ListIndexVersions(ctx context.Context, userID, documentID contracts.ID) ([]DocumentIndexVersionView, error) {
	versions, err := s.chunks.ListIndexVersions(ctx, string(userID), string(documentID))
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	views := make([]DocumentIndexVersionView, 0, len(versions))
	for _, version := range versions {
		views = append(views, DocumentIndexVersionView{Version: version.Version, ChunkCount: version.ChunkCount, VectorCount: version.VectorCount, Status: version.Status, CreatedAt: version.CreatedAt})
	}
	return views, nil
}

// ListImportTasks 分页查询知识库导入任务。
func (s *documentProcessService) ListImportTasks(ctx context.Context, userID, kbID contracts.ID, page, pageSize int) ([]ImportTaskView, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := s.tasks.ListByKB(ctx, string(userID), string(kbID), page, pageSize)
	if err != nil {
		return nil, 0, apperrors.New(contracts.ErrInternal, err)
	}
	views := make([]ImportTaskView, 0, len(items))
	for _, task := range items {
		views = append(views, importTaskView(task))
	}
	return views, total, nil
}

// GetImportTask 查询导入任务详情。
func (s *documentProcessService) GetImportTask(ctx context.Context, userID, taskID contracts.ID) (ImportTaskView, error) {
	task, err := s.tasks.FindByID(ctx, string(userID), string(taskID))
	if errors.Is(err, repository.ErrImportTaskNotFound) {
		return ImportTaskView{}, apperrors.ErrNotFound
	}
	if err != nil {
		return ImportTaskView{}, apperrors.New(contracts.ErrInternal, err)
	}
	return importTaskView(task), nil
}

// ProcessImportTask 由 Worker 调用：领取后的任务编排。
// 流程：校验状态 → 去重 → 幂等检查 → 创建文档行（若需）→ 运行 Eino 文档加工 Graph → 标记任务 succeeded。
// 加工失败：任务标记 failed 且文档 processing_status=failed，失败原因安全落库。
func (s *documentProcessService) ProcessImportTask(ctx context.Context, taskID contracts.ID) error {
	task, err := s.tasks.FindByIDInternal(ctx, string(taskID))
	if err != nil {
		return err
	}
	// 状态机：pending → running（Worker 领取）→ succeeded / failed / skipped；
	// 仅 running 允许继续编排，防止绕过领取流程直接处理任务。
	if task.Status != string(contracts.TaskStatusRunning) {
		return fmt.Errorf("任务 %q 状态 %q 不允许处理", taskID, task.Status)
	}

	// 去重策略：duplicate_policy=skip 且存在相同源哈希的文档时跳过。
	if task.DuplicatePolicy == "skip" && task.SourceHash != nil && *task.SourceHash != "" {
		existing, findErr := s.docs.FindBySourceHash(ctx, task.UserID, task.KnowledgeBaseID, *task.SourceHash)
		if findErr != nil && !errors.Is(findErr, repository.ErrDocumentNotFound) {
			return fmt.Errorf("去重检查失败: %w", findErr)
		}
		if existing != nil {
			if skipErr := s.tasks.SkipTask(ctx, task.ID); skipErr != nil {
				return fmt.Errorf("跳过重复任务失败: %w", skipErr)
			}
			logger.Info("导入任务按去重策略跳过", zap.String("task_id", task.ID), zap.String("document_id", existing.ID))
			return nil
		}
	}

	// 幂等：任务已关联文档时直接复用，不重复创建。
	doc, err := s.docs.FindByImportTask(ctx, task.ID)
	if err != nil && !errors.Is(err, repository.ErrDocumentNotFound) {
		// 数据库故障等非"未找到"错误必须向上返回，避免重复创建文档。
		return fmt.Errorf("查询任务关联文档失败: %w", err)
	}
	// 文档行仅在首次处理时创建（pending）；重试或幂等命中时复用已有关联文档，避免重复创建。
	if doc == nil {
		doc = &entity.Document{
			UserID:           task.UserID,
			KnowledgeBaseID:  task.KnowledgeBaseID,
			DirectoryID:      task.TargetDirectoryID,
			Title:            documentTitle(task),
			SourceType:       task.SourceType,
			SourceURL:        task.SourceURL,
			OriginalFileName: task.FileName,
			FileSize:         task.FileSize,
			MIMEType:         task.MIMEType,
			FileHash:         task.SourceHash,
			MinIOBucket:      task.MinIOBucket,
			MinIOObjectKey:   task.MinIOObjectKey,
			// 文件/URL 文档正文由解析产物提供，content 为空；content_format 必须满足
			// 数据库检查约束（txt/markdown），这里固定为 txt 与列默认值一致。
			ContentFormat:    "txt",
			ProcessingStatus: string(contracts.ProcessingPending),
			ContentVersion:   1,
			ChunkVersion:     1,
		}
		if err := s.docs.Create(ctx, doc); err != nil {
			return fmt.Errorf("创建文档失败: %w", err)
		}
		if err := s.tasks.AttachDocument(ctx, task.ID, doc.ID); err != nil {
			if cleanupErr := s.docs.SoftDelete(ctx, doc.UserID, doc.ID); cleanupErr != nil {
				logger.Error("任务关联失败后清理文档失败", zap.String("task_id", task.ID), zap.String("document_id", doc.ID), zap.Error(cleanupErr))
			}
			return fmt.Errorf("持久化任务文档关联失败: %w", err)
		}
		task.DocumentID = &doc.ID
	}
	// Preview 是独立派生链路：调度失败只记录日志，绝不能使解析/索引任务失败。
	if s.previewScheduler != nil {
		if err := s.previewScheduler.EnsureDocument(ctx, doc.ID); err != nil {
			logger.Warn("调度文档预览失败，文档加工继续", zap.String("document_id", doc.ID), zap.Error(err))
		}
	}

	// 执行文档加工流水线（file/url 来源有 MinIO 对象或 URL；手工文档正文在 documents.content）。
	if s.processor != nil && ((task.MinIOObjectKey != nil && *task.MinIOObjectKey != "") || (task.SourceURL != nil && *task.SourceURL != "") || task.SourceType == string(contracts.DocumentSourceManual)) {
		if err := s.processDocument(ctx, task, doc); err != nil {
			// 加工失败：文档标记 failed，旧索引不受影响。
			// 任务状态由 Runner 的 Fail 路径统一回写（Source.Fail → FailTask），避免双重标记。
			s.markDocumentFailed(ctx, doc.ID, task.ID, err)
			return err
		}
	} else {
		logger.Warn("文档无 MinIO 对象或流水线未就绪，跳过加工",
			zap.String("task_id", task.ID), zap.String("document_id", doc.ID))
	}

	if err := s.tasks.CompleteSucceeded(ctx, task.ID, &doc.ID); err != nil {
		return fmt.Errorf("完成导入任务失败: %w", err)
	}
	logger.Info("导入任务处理完成", zap.String("task_id", task.ID), zap.String("document_id", doc.ID))
	return nil
}

// processDocument 调用 Eino 文档加工 Graph，并在加工过程中更新文档处理状态。
// 注意：active_index_version 只在新版本全部成功落库后才更新，构建期间旧版本继续可用。
func (s *documentProcessService) processDocument(ctx context.Context, task *entity.ImportTask, doc *entity.Document) error {
	// 在数据库行锁内领取候选版本。owner 使用稳定的任务 ID：同任务重试复用版本，
	// 并发任务不能清理当前候选；过期任务被接管时会分配更高版本。
	indexVersion, err := s.docs.BeginIndexBuild(ctx, doc.ID, task.ID, time.Now().UTC().Add(-repository.ImportTaskLease()))
	if err != nil {
		return fmt.Errorf("领取文档加工版本失败: %w", err)
	}
	_ = s.tasks.SetRunningStep(ctx, task.ID, "parsing")

	// 清理同索引版本的残留 Chunk 与向量（重试/部分失败后的幂等保障）。
	if err := s.chunks.DeleteByVersion(ctx, doc.ID, indexVersion); err != nil {
		return fmt.Errorf("清理旧版本 Chunk 失败: %w", err)
	}
	if s.vectors != nil {
		if err := s.vectors.DeleteByVersion(ctx, doc.ID, indexVersion); err != nil {
			return fmt.Errorf("清理旧版本向量失败: %w", err)
		}
	}
	var embeddingModelID string
	var embedder embedding.Embedder
	if s.embeddings != nil {
		var err error
		embeddingModelID, embedder, err = s.embeddings.Resolve(ctx, task.UserID, task.KnowledgeBaseID)
		if err != nil {
			return fmt.Errorf("解析文档 Embedding 模型失败: %w", err)
		}
	}

	// 执行 Eino 文档加工 Graph：
	// resolve_artifact → parse_if_missing → validate → persist_artifact → normalize
	// → enrich → structure_chunk → clean → token_count → persist → index。
	// 节点顺序由 pipeline 编译期固定，此处仅传入对象与元数据触发整条流水线。
	input := pipeline.ProcessInput{
		ObjectKey:        valueOrEmpty(task.MinIOObjectKey),
		SourceURL:        valueOrEmpty(task.SourceURL),
		FileName:         valueOrEmpty(task.FileName),
		MIMEType:         valueOrEmpty(task.MIMEType),
		Attachments:      task.Attachments,
		Embedder:         embedder,
		EmbeddingModelID: embeddingModelID,
		DocMeta: transformer.DocMeta{
			UserID:          task.UserID,
			KnowledgeBaseID: task.KnowledgeBaseID,
			DocumentID:      doc.ID,
			IndexVersion:    indexVersion,
			ContentVersion:  doc.ContentVersion,
			ChunkVersion:    doc.ChunkVersion,
			DocumentTitle:   doc.Title,
			SourceHash:      valueOrEmpty(task.SourceHash),
			SourceLocation: map[string]any{
				"source_type": task.SourceType,
				"file_name":   valueOrEmpty(task.FileName),
				"source_url":  valueOrEmpty(task.SourceURL),
			},
		},
	}
	input.ChunkStrategyOverride = s.resolveChunkStrategyOverride(ctx, doc, task)
	// 手工文档：正文存于 documents.content，直接交给流水线（不访问 MinIO）。
	if task.SourceType == string(contracts.DocumentSourceManual) {
		content := ""
		if doc.Content != nil {
			content = *doc.Content
		}
		input.Content = content
		if input.FileName == "" {
			input.FileName = manualTaskFileName(doc)
		}
	}
	out, err := s.processor.Run(ctx, input)
	if err != nil {
		return fmt.Errorf("文档加工失败: %w", err)
	}
	// 纯图片文档允许 0 Chunk 成功导入（无文字图片无可索引内容，资产与原文件保留）；
	// 有正文却分不出 Chunk 的情况由流水线 structure_chunk 节点拒绝。
	// 加工成功：切换 active_index_version，并记录 Embedding 模型与分段配置哈希。
	effectiveChunkConfigHash := out.ChunkConfigHash
	if effectiveChunkConfigHash == "" && s.processor != nil {
		// 兼容尚未返回文档级哈希的自定义 Processor 实现。
		effectiveChunkConfigHash = s.processor.ChunkConfigHash()
	}
	updates := map[string]any{
		"processing_status": string(contracts.ProcessingSucceeded),
		"chunk_config_hash": stringPtrOrNil(effectiveChunkConfigHash),
	}
	if out.FinalURL != "" {
		updates["source_url"] = out.FinalURL
	}
	if out.SourceHash != "" {
		updates["file_hash"] = out.SourceHash
	}
	if strings.TrimSpace(out.Title) != "" {
		updates["title"] = strings.TrimSpace(out.Title)
	}
	if len(out.Warnings) > 0 {
		warningsJSON, marshalErr := json.Marshal(out.Warnings)
		if marshalErr != nil {
			return fmt.Errorf("序列化解析警告失败: %w", marshalErr)
		}
		updates["parse_warnings"] = string(warningsJSON)
	} else {
		updates["parse_warnings"] = nil
	}
	if out.ChunkDiffReport != nil {
		diffJSON, marshalErr := json.Marshal(out.ChunkDiffReport)
		if marshalErr != nil {
			return fmt.Errorf("序列化分块差异报告失败: %w", marshalErr)
		}
		updates["chunk_diff_report"] = string(diffJSON)
	} else {
		updates["chunk_diff_report"] = nil
	}
	if task.SourceType == string(contracts.DocumentSourceURL) {
		if err := s.tasks.UpdateURLResult(ctx, task.ID, out.FinalURL, out.SourceHash); err != nil {
			return fmt.Errorf("保存 URL 导入结果失败: %w", err)
		}
	}
	if embeddingModelID != "" {
		updates["embedding_model_id"] = embeddingModelID
	} else if modelID := s.processor.EmbeddingModelID(); modelID != "" {
		updates["embedding_model_id"] = modelID
	} else {
		// 新活动版本未生成向量时清空旧模型，避免把关键词索引误报为混合索引。
		updates["embedding_model_id"] = nil
	}
	if publisher, ok := s.docs.(repository.IndexBuildCompletionRepository); ok {
		if err := publisher.PublishIndexBuildAndCompleteTask(ctx, doc.ID, task.ID, indexVersion, updates); err != nil {
			return fmt.Errorf("发布活动索引并完成任务失败: %w", err)
		}
		return nil
	}
	// 兼容测试替身和外部自定义仓储；生产 repository 使用上面的事务路径。
	if err := s.docs.PublishIndexBuild(ctx, doc.ID, task.ID, indexVersion, updates); err != nil {
		return fmt.Errorf("切换活动索引版本失败: %w", err)
	}
	if err := s.tasks.SetRunningStep(ctx, task.ID, "succeeded"); err != nil {
		// active 已经由 fencing 条件安全发布；步骤字段只是过程可观测信息，
		// 最终任务状态仍由 CompleteSucceeded 统一写入，不能因此把已发布文档误标失败。
		logger.Warn("更新导入任务完成步骤失败", zap.String("task_id", task.ID), zap.Error(err))
	}
	return nil
}

// resolveChunkStrategyOverride 按“文档 > 知识库 > 环境”解析人工覆盖。
// 环境级策略由 Pipeline 配置承担，因此无覆盖时返回空字符串。
func (s *documentProcessService) resolveChunkStrategyOverride(ctx context.Context, doc *entity.Document, task *entity.ImportTask) string {
	if doc != nil && doc.ChunkStrategy != nil {
		return strings.TrimSpace(*doc.ChunkStrategy)
	}
	if s.kbs == nil || task == nil {
		return ""
	}
	kb, err := s.kbs.FindByID(ctx, task.UserID, task.KnowledgeBaseID)
	if err != nil {
		logger.Warn("读取知识库分块策略失败，回退环境配置", zap.String("knowledge_base_id", task.KnowledgeBaseID), zap.Error(err))
		return ""
	}
	if kb.ChunkStrategy == nil {
		return ""
	}
	return strings.TrimSpace(*kb.ChunkStrategy)
}

// markDocumentFailed 标记文档处理失败，保留失败步骤与原因。
// 任务状态由 Runner 的 Fail 路径统一回写。
func (s *documentProcessService) markDocumentFailed(ctx context.Context, docID, owner string, cause error) {
	reason := cause.Error()
	if err := s.docs.FailIndexBuild(ctx, docID, owner, "document_pipeline", reason); err != nil {
		logger.Error("标记文档处理失败失败", zap.String("document_id", docID), zap.Error(err))
	}
}

// RecoverStaleTasks 恢复卡在 running 且超过租约的任务（由 Worker 启动时调用，
// 运行中由 Repository.ReservePending 每次领取前自动恢复）。
func (s *documentProcessService) RecoverStaleTasks(ctx context.Context) (int64, error) {
	staleBefore := time.Now().UTC().Add(-repository.ImportTaskLease()).Unix()
	return s.tasks.RecoverStale(ctx, staleBefore)
}

// CleanupInactiveIndexes 清理已软删除文档与超出保留版本的旧索引 Chunk/向量。
// 先删向量（外键引用 Chunk）再删 Chunk；正在构建的新版本不受影响。
func (s *documentProcessService) CleanupInactiveIndexes(ctx context.Context, retention int) (int64, error) {
	var total int64
	if s.vectors != nil {
		if n, err := s.vectors.CleanupInactive(ctx, retention); err != nil {
			return total, err
		} else {
			total += n
		}
	}
	if s.chunks != nil {
		if n, err := s.chunks.CleanupInactive(ctx, retention); err != nil {
			return total, err
		} else {
			total += n
		}
	}
	logger.Info("旧索引版本清理完成", zap.Int64("deleted", total), zap.Int("retention", retention))
	return total, nil
}

// CleanupImportTasks 清理知识库内已结束的导入任务记录，保留进行中任务。
func (s *documentProcessService) CleanupImportTasks(ctx context.Context, userID, kbID contracts.ID) (int64, error) {
	count, err := s.tasks.DeleteCompletedByKB(ctx, string(userID), string(kbID))
	if err != nil {
		return 0, apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("已完成导入任务已清理", zap.String("user_id", string(userID)), zap.String("knowledge_base_id", string(kbID)), zap.Int64("deleted", count))
	return count, nil
}

// importTaskView 将导入任务实体转换为对外视图，仅暴露稳定字段，隐藏内部存储细节。
func importTaskView(task *entity.ImportTask) ImportTaskView {
	view := ImportTaskView{
		ID:            contracts.ID(task.ID),
		BatchID:       task.BatchID,
		SourcePath:    task.SourcePath,
		SourceType:    contracts.DocumentSourceType(task.SourceType),
		FileName:      task.FileName,
		FileSize:      task.FileSize,
		MIMEType:      task.MIMEType,
		SourceURL:     task.SourceURL,
		Status:        contracts.ImportTaskStatus(task.Status),
		CurrentStep:   task.CurrentStep,
		FailureReason: task.FailureReason,
		CreatedAt:     task.CreatedAt,
		CompletedAt:   task.CompletedAt,
	}
	if task.DocumentID != nil {
		id := contracts.ID(*task.DocumentID)
		view.DocumentID = &id
	}
	return view
}

// documentTitle 生成文档标题：优先取文件名，其次取来源 URL，兜底为“未命名文档”。
func documentTitle(task *entity.ImportTask) string {
	if task.FileName != nil && strings.TrimSpace(*task.FileName) != "" {
		return strings.TrimSpace(*task.FileName)
	}
	if task.SourceURL != nil && strings.TrimSpace(*task.SourceURL) != "" {
		return strings.TrimSpace(*task.SourceURL)
	}
	return "未命名文档"
}

// valueOrEmpty 将字符串指针安全解引用，nil 时返回空串。
func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// stringPtrOrNil 将空字符串转换为 nil，便于写入可空配置字段。
func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// valueOrZero 将整数指针安全解引用，nil 时返回 0。
func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
