package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/pipeline"
	"github.com/1090-f/Memora/internal/service/rag/transformer"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/cloudwego/eino/components/embedding"
	"go.uber.org/zap"
)

// documentProcessService 是 DocumentProcessService 接口的实现。
// 任务包 04 起：创建文档行后调用 Eino 文档加工 Graph（解析/清洗/分段/落库/向量）。
type documentProcessService struct {
	tasks      repository.ImportTaskRepository
	docs       repository.DocumentRepository
	chunks     repository.DocumentChunkRepository
	vectors    repository.VectorRepository
	processor  DocumentProcessor
	embeddings DocumentEmbeddingResolver
}

// NewDocumentProcessService 创建一个新的文档处理服务实例。
// processor 为 nil 时退回任务包 03 行为（仅创建文档行，不执行加工）。
func NewDocumentProcessService(
	tasks repository.ImportTaskRepository,
	docs repository.DocumentRepository,
	chunks repository.DocumentChunkRepository,
	vectors repository.VectorRepository,
	processor DocumentProcessor,
	embeddings DocumentEmbeddingResolver,
) DocumentProcessService {
	return &documentProcessService{tasks: tasks, docs: docs, chunks: chunks, vectors: vectors, processor: processor, embeddings: embeddings}
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

// Reindex 为现有文档创建指向原文来源的新导入任务；Worker 构建新版本后原子切换活动索引。
func (s *documentProcessService) Reindex(ctx context.Context, userID, kbID, documentID contracts.ID) error {
	doc, err := s.docs.FindByIDInKB(ctx, string(userID), string(kbID), string(documentID))
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	if (doc.MinIOObjectKey == nil || *doc.MinIOObjectKey == "") && (doc.SourceURL == nil || *doc.SourceURL == "") {
		return apperrors.New(contracts.ErrInvalidState, fmt.Errorf("文档没有可重建的原始来源"))
	}
	task := &entity.ImportTask{
		UserID: doc.UserID, KnowledgeBaseID: doc.KnowledgeBaseID, TargetDirectoryID: doc.DirectoryID,
		SourceType: doc.SourceType, FileName: doc.OriginalFileName, FileSize: doc.FileSize, MIMEType: doc.MIMEType,
		SourceURL: doc.SourceURL, SourceHash: doc.FileHash, MinIOBucket: doc.MinIOBucket, MinIOObjectKey: doc.MinIOObjectKey,
		DuplicatePolicy: "create_new", Status: string(contracts.TaskStatusPending), DocumentID: &doc.ID,
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

	// 执行文档加工流水线（file 来源且已落 MinIO 时）。
	if s.processor != nil && ((task.MinIOObjectKey != nil && *task.MinIOObjectKey != "") || (task.SourceURL != nil && *task.SourceURL != "")) {
		if err := s.processDocument(ctx, task, doc); err != nil {
			// 加工失败：文档标记 failed，旧索引不受影响。
			// 任务状态由 Runner 的 Fail 路径统一回写（Source.Fail → FailTask），避免双重标记。
			s.markDocumentFailed(ctx, doc.ID, err)
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
	indexVersion := 1
	if doc.ActiveIndexVersion != nil {
		indexVersion = *doc.ActiveIndexVersion + 1
	}
	// 每次加工递增 index_version：新版本全部落库成功前不切换 active_index_version，
	// 构建期间旧版本索引继续对外可用。
	// 加工开始：仅更新状态，不切换 active_index_version。
	_ = s.docs.UpdateProcessing(ctx, doc.ID, map[string]any{
		"processing_status": string(contracts.ProcessingParsing),
		"failure_step":      nil,
		"failure_reason":    nil,
	})
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
	out, err := s.processor.Run(ctx, pipeline.ProcessInput{
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
	})
	if err != nil {
		return fmt.Errorf("文档加工失败: %w", err)
	}
	if out.ChunkCount == 0 {
		return fmt.Errorf("文档加工未产生任何 Chunk")
	}
	// 加工成功：切换 active_index_version，并记录 Embedding 模型与分段配置哈希。
	updates := map[string]any{
		"processing_status":    string(contracts.ProcessingSucceeded),
		"active_index_version": indexVersion,
		"chunk_config_hash":    s.chunkConfigHash(doc),
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
	if err := s.docs.UpdateProcessing(ctx, doc.ID, updates); err != nil {
		return fmt.Errorf("切换活动索引版本失败: %w", err)
	}
	if err := s.tasks.SetRunningStep(ctx, task.ID, "succeeded"); err != nil {
		return fmt.Errorf("更新导入任务完成步骤失败: %w", err)
	}
	return nil
}

// chunkConfigHash 返回当前分段配置哈希（与 pipeline 使用的保持一致）。
func (s *documentProcessService) chunkConfigHash(_ *entity.Document) *string {
	if s.processor == nil {
		return nil
	}
	hash := s.processor.ChunkConfigHash()
	if hash == "" {
		return nil
	}
	return &hash
}

// markDocumentFailed 标记文档处理失败，保留失败步骤与原因。
// 任务状态由 Runner 的 Fail 路径统一回写。
func (s *documentProcessService) markDocumentFailed(ctx context.Context, docID string, cause error) {
	reason := cause.Error()
	if err := s.docs.UpdateProcessing(ctx, docID, map[string]any{
		"processing_status": string(contracts.ProcessingFailed),
		"failure_step":      "document_pipeline",
		"failure_reason":    reason,
	}); err != nil {
		logger.Error("标记文档处理失败失败", zap.String("document_id", docID), zap.Error(err))
	}
}

// RecoverStaleTasks 恢复卡在 running 且超过租约的任务（由 Worker 启动时调用，
// 运行中由 Repository.ReservePending 每次领取前自动恢复）。
func (s *documentProcessService) RecoverStaleTasks(ctx context.Context) (int64, error) {
	staleBefore := time.Now().UTC().Add(-repository.ImportTaskLease()).Unix()
	return s.tasks.RecoverStale(ctx, staleBefore)
}

// importTaskView 将导入任务实体转换为对外视图，仅暴露稳定字段，隐藏内部存储细节。
func importTaskView(task *entity.ImportTask) ImportTaskView {
	view := ImportTaskView{
		ID:            contracts.ID(task.ID),
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

// valueOrZero 将整数指针安全解引用，nil 时返回 0。
func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
