package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrDocumentNotFound 表示未找到指定文档。
	ErrDocumentNotFound = errors.New("文档不存在")
	// ErrImportTaskNotFound 表示未找到指定导入任务。
	ErrImportTaskNotFound = errors.New("导入任务不存在")
	// ErrImportTaskConflict 表示任务状态并发冲突（另一 Worker 抢先领取）。
	ErrImportTaskConflict = errors.New("导入任务状态冲突")
	// ErrDocumentProcessingConflict 表示文档候选索引正由另一任务构建，或当前任务已失去发布权。
	ErrDocumentProcessingConflict = errors.New("文档加工所有权冲突")
)

// importTaskLease 是导入任务的 Worker 租约时长：running 超过该时长视为 Worker 崩溃，恢复为 pending。
const importTaskLease = 10 * time.Minute

// ImportTaskLease 返回导入任务的租约时长，供 Service 计算恢复窗口。
func ImportTaskLease() time.Duration { return importTaskLease }

// documentRepository 是 DocumentRepository 接口的 GORM 实现。
type documentRepository struct{ db *gorm.DB }

// NewDocumentRepository 创建一个新的文档仓储实例。
func NewDocumentRepository(db *gorm.DB) DocumentRepository {
	return &documentRepository{db: db}
}

// Create 创建文档。
func (r *documentRepository) Create(ctx context.Context, doc *entity.Document) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).Create(doc).Error
	if err != nil {
		return fmt.Errorf("创建文档失败: %w", err)
	}
	return nil
}

// FindByID 按文档 ID 与用户查询文档。
func (r *documentRepository) FindByID(ctx context.Context, userID, documentID string) (*entity.Document, error) {
	var doc entity.Document
	// 查询强制带 user_id 与 deleted_at，防止越权访问他人文档或命中已删除记录。
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", documentID, userID).
		First(&doc).Error
	return mapDocumentResult(&doc, err)
}

// FindByIDInternal 按文档 ID 查询未删除文档（资产签名 URL 校验后使用）。
func (r *documentRepository) FindByIDInternal(ctx context.Context, documentID string) (*entity.Document, error) {
	var doc entity.Document
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", documentID).
		First(&doc).Error
	return mapDocumentResult(&doc, err)
}

// FindByIDInKB 按文档 ID、用户与知识库查询文档。
func (r *documentRepository) FindByIDInKB(ctx context.Context, userID, kbID, documentID string) (*entity.Document, error) {
	var doc entity.Document
	// 归属过滤包含 user_id 与 knowledge_base_id，确保文档确实位于该知识库。
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL", documentID, userID, kbID).
		First(&doc).Error
	return mapDocumentResult(&doc, err)
}

// ListByKB 分页查询知识库文档列表。
func (r *documentRepository) ListByKB(ctx context.Context, userID, kbID string, page, pageSize int, filter DocumentFilter) ([]*entity.Document, int64, error) {
	db := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.Document{})
	db = db.Where("user_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL", userID, kbID)
	// 关键词先去首尾空白并统一小写，标题同样转小写后模糊匹配，保证大小写不敏感。
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		db = db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	if filter.DirectoryID != nil {
		db = db.Where("directory_id = ?", *filter.DirectoryID)
	}
	if filter.ProcessingStatus != nil {
		db = db.Where("processing_status = ?", *filter.ProcessingStatus)
	}
	if filter.IndexMode != nil {
		switch *filter.IndexMode {
		case "none":
			db = db.Where("active_index_version IS NULL")
		case "keyword":
			db = db.Where("active_index_version IS NOT NULL AND embedding_model_id IS NULL")
		case "hybrid":
			db = db.Where("active_index_version IS NOT NULL AND embedding_model_id IS NOT NULL")
		}
	}
	if filter.SourceType != nil {
		db = db.Where("source_type = ?", *filter.SourceType)
	}
	// 先 Count 取总数（供前端分页），再以同样过滤条件 LIMIT/OFFSET 取当前页数据。
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计文档失败: %w", err)
	}
	var items []*entity.Document
	if err := db.Order("updated_at DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("查询文档列表失败: %w", err)
	}
	return items, total, nil
}

// SoftDelete 软删除文档。
func (r *documentRepository) SoftDelete(ctx context.Context, userID, documentID string) error {
	// WHERE 带 deleted_at IS NULL：仅更新未删除行，已删除或不存在时 RowsAffected 为 0 报未找到。
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.Document{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", documentID, userID).
		Update("deleted_at", time.Now().UTC())
	if result.Error != nil {
		return fmt.Errorf("删除文档失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// Move 更新文档所属目录，并显式刷新更新时间。
func (r *documentRepository) Move(ctx context.Context, userID, documentID string, directoryID *string) error {
	var directoryValue any
	if directoryID != nil {
		directoryValue = *directoryID
	}
	updates := map[string]any{"directory_id": directoryValue, "updated_at": time.Now().UTC()}
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.Document{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", documentID, userID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("移动文档失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// FindBySourceHash 按用户、知识库与源哈希查询未删除文档。
func (r *documentRepository) FindBySourceHash(ctx context.Context, userID, kbID, sourceHash string) (*entity.Document, error) {
	// 按文件哈希判重（duplicate_policy=skip 用），限定同一用户同一知识库且未删除。
	var doc entity.Document
	result := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND knowledge_base_id = ? AND file_hash = ? AND deleted_at IS NULL", userID, kbID, sourceHash).
		Limit(1).Find(&doc)
	if result.Error != nil {
		return nil, fmt.Errorf("查询文档失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrDocumentNotFound
	}
	return &doc, nil
}

// FindByImportTask 通过 import_tasks.document_id 查询导入任务关联的文档。
func (r *documentRepository) FindByImportTask(ctx context.Context, taskID string) (*entity.Document, error) {
	var doc entity.Document
	// 经 import_tasks.document_id 反查关联文档，用于导入流程中任务完成后回填。
	result := dbFromContext(ctx, r.db).WithContext(ctx).
		Table("documents").
		Joins("JOIN import_tasks ON import_tasks.document_id = documents.id").
		Where("import_tasks.id = ? AND documents.deleted_at IS NULL", taskID).
		Limit(1).Find(&doc)
	if result.Error != nil {
		return nil, fmt.Errorf("查询任务关联文档失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrDocumentNotFound
	}
	return &doc, nil
}

// UpdateProcessing 更新文档处理状态与相关字段。
func (r *documentRepository) UpdateProcessing(ctx context.Context, docID string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now().UTC()
	// 用 map 动态更新，仅修改调用方传入的字段，避免误覆盖其他列。
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.Document{}).
		Where("id = ? AND deleted_at IS NULL", docID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新文档处理状态失败: %w", result.Error)
	}
	// 目标行不存在或已软删除时 RowsAffected 为 0，报未找到。
	if result.RowsAffected == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// BeginIndexBuild 在文档行锁内领取候选索引构建权。
// 同一任务重试复用原候选版本；不同任务仅可接管过期构建，并始终分配更高版本，
// 避免新旧 Worker 清理或写入同一 index_version。
func (r *documentRepository) BeginIndexBuild(ctx context.Context, docID, owner string, staleBefore time.Time) (int, error) {
	var indexVersion int
	err := dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var doc entity.Document
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL", docID).First(&doc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDocumentNotFound
			}
			return fmt.Errorf("锁定文档加工版本失败: %w", err)
		}

		sameOwner := doc.IndexBuildOwner != nil && *doc.IndexBuildOwner == owner
		if doc.IndexBuildOwner != nil && !sameOwner &&
			(doc.IndexBuildStartedAt == nil || doc.IndexBuildStartedAt.After(staleBefore)) {
			return ErrDocumentProcessingConflict
		}

		if sameOwner && doc.IndexBuildVersion != nil {
			indexVersion = *doc.IndexBuildVersion
		} else {
			latestVersion := 0
			if doc.ActiveIndexVersion != nil && *doc.ActiveIndexVersion > latestVersion {
				latestVersion = *doc.ActiveIndexVersion
			}
			if doc.IndexBuildVersion != nil && *doc.IndexBuildVersion > latestVersion {
				latestVersion = *doc.IndexBuildVersion
			}
			indexVersion = latestVersion + 1
		}

		now := time.Now().UTC()
		result := tx.Model(&entity.Document{}).
			Where("id = ? AND deleted_at IS NULL", docID).
			Updates(map[string]any{
				"index_build_owner":      owner,
				"index_build_version":    indexVersion,
				"index_build_started_at": now,
				"processing_status":      string(contracts.ProcessingParsing),
				"failure_step":           nil,
				"failure_reason":         nil,
				"updated_at":             now,
			})
		if result.Error != nil {
			return fmt.Errorf("领取文档加工版本失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrDocumentNotFound
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return indexVersion, nil
}

// PublishIndexBuild 使用 owner + indexVersion 作为 fencing 条件发布候选索引。
func (r *documentRepository) PublishIndexBuild(ctx context.Context, docID, owner string, indexVersion int, updates map[string]any) error {
	if updates == nil {
		updates = make(map[string]any)
	}
	updates["active_index_version"] = indexVersion
	updates["index_build_owner"] = nil
	updates["index_build_version"] = nil
	updates["index_build_started_at"] = nil
	updates["updated_at"] = time.Now().UTC()
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.Document{}).
		Where("id = ? AND deleted_at IS NULL AND index_build_owner = ? AND index_build_version = ?", docID, owner, indexVersion).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("发布文档索引版本失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrDocumentProcessingConflict
	}
	return nil
}

// PublishIndexBuildAndCompleteTask 在同一事务中发布活动索引并完成其所有者任务。
// 任一步骤失败都会回滚，杜绝“索引已 active、任务却被标记 failed”的分裂状态。
func (r *documentRepository) PublishIndexBuildAndCompleteTask(ctx context.Context, docID, owner string, indexVersion int, updates map[string]any) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		docUpdates := make(map[string]any, len(updates)+5)
		for key, value := range updates {
			docUpdates[key] = value
		}
		docUpdates["active_index_version"] = indexVersion
		docUpdates["index_build_owner"] = nil
		docUpdates["index_build_version"] = nil
		docUpdates["index_build_started_at"] = nil
		docUpdates["updated_at"] = time.Now().UTC()
		docResult := tx.Model(&entity.Document{}).
			Where("id = ? AND deleted_at IS NULL AND index_build_owner = ? AND index_build_version = ?", docID, owner, indexVersion).
			Updates(docUpdates)
		if docResult.Error != nil {
			return fmt.Errorf("事务发布文档索引版本失败: %w", docResult.Error)
		}
		if docResult.RowsAffected == 0 {
			return ErrDocumentProcessingConflict
		}
		taskResult := tx.Model(&entity.ImportTask{}).
			Where("id = ? AND status = 'running' AND document_id = ?", owner, docID).
			Updates(map[string]any{
				"status": "succeeded", "current_step": "succeeded", "completed_at": time.Now().UTC(), "failure_reason": nil,
			})
		if taskResult.Error != nil {
			return fmt.Errorf("事务完成导入任务失败: %w", taskResult.Error)
		}
		if taskResult.RowsAffected == 0 {
			return ErrImportTaskConflict
		}
		return nil
	})
}

// FailIndexBuild 只允许当前 owner 写入失败状态；旧 Worker 失去所有权后静默退出，
// 防止其失败回调覆盖新任务的 parsing/succeeded 状态。
func (r *documentRepository) FailIndexBuild(ctx context.Context, docID, owner, failureStep, failureReason string) error {
	now := time.Now().UTC()
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.Document{}).
		Where("id = ? AND deleted_at IS NULL AND index_build_owner = ?", docID, owner).
		Updates(map[string]any{
			"processing_status":      string(contracts.ProcessingFailed),
			"failure_step":           failureStep,
			"failure_reason":         failureReason,
			"index_build_owner":      nil,
			"index_build_version":    nil,
			"index_build_started_at": nil,
			"updated_at":             now,
		})
	if result.Error != nil {
		return fmt.Errorf("标记文档加工失败: %w", result.Error)
	}
	return nil
}

func mapDocumentResult(doc *entity.Document, err error) (*entity.Document, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询文档失败: %w", err)
	}
	return doc, nil
}

// importTaskRepository 是 ImportTaskRepository 接口的 GORM 实现。
type importTaskRepository struct{ db *gorm.DB }

// NewImportTaskRepository 创建一个新的导入任务仓储实例。
func NewImportTaskRepository(db *gorm.DB) ImportTaskRepository {
	return &importTaskRepository{db: db}
}

// Create 创建导入任务。
func (r *importTaskRepository) Create(ctx context.Context, task *entity.ImportTask) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return fmt.Errorf("创建导入任务失败: %w", err)
		}
		if (task.MinIOObjectKey != nil && *task.MinIOObjectKey != "") || (task.SourceType == "url" && task.SourceURL != nil && *task.SourceURL != "") || task.SourceType == string(contracts.DocumentSourceManual) {
			return enqueueTaskEvent(tx, task.ID)
		}
		return nil
	})
}

// FindByID 按任务 ID 与用户查询任务。
func (r *importTaskRepository) FindByID(ctx context.Context, userID, taskID string) (*entity.ImportTask, error) {
	var task entity.ImportTask
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ?", taskID, userID).
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrImportTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询导入任务失败: %w", err)
	}
	return &task, nil
}

// FindByIDInternal 按任务 ID 查询任务，仅 Worker 内部使用。
func (r *importTaskRepository) FindByIDInternal(ctx context.Context, taskID string) (*entity.ImportTask, error) {
	var task entity.ImportTask
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ?", taskID).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrImportTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询导入任务失败: %w", err)
	}
	return &task, nil
}

// UpdateObjectInfo 更新任务的 MinIO 对象信息与源哈希。
func (r *importTaskRepository) UpdateObjectInfo(ctx context.Context, userID, taskID string, bucket, objectKey string, sourceHash *string) error {
	return r.updateObjectInfo(ctx, userID, taskID, bucket, objectKey, sourceHash, true)
}

// UpdateObjectInfoNoEnqueue 更新对象信息但不入队：Markdown/ZIP 需用户确认补传图片后
// 通过 StartPendingTask 显式触发解析。
func (r *importTaskRepository) UpdateObjectInfoNoEnqueue(ctx context.Context, userID, taskID string, bucket, objectKey string, sourceHash *string) error {
	return r.updateObjectInfo(ctx, userID, taskID, bucket, objectKey, sourceHash, false)
}

func (r *importTaskRepository) updateObjectInfo(ctx context.Context, userID, taskID string, bucket, objectKey string, sourceHash *string, enqueue bool) error {
	updates := map[string]any{
		"minio_bucket":     bucket,
		"minio_object_key": objectKey,
	}
	if sourceHash != nil {
		updates["source_hash"] = *sourceHash
	}
	return dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&entity.ImportTask{}).
			Where("id = ? AND user_id = ?", taskID, userID).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("更新导入任务对象信息失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrImportTaskNotFound
		}
		if enqueue {
			return enqueueTaskEvent(tx, taskID)
		}
		return nil
	})
}

// StartPendingTask 将 pending 任务入队（触发 Worker 处理）；仅允许未处理的任务。
// 供 Markdown/ZIP 导入在用户确认图片补传后显式触发。
func (r *importTaskRepository) StartPendingTask(ctx context.Context, userID, taskID string) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task entity.ImportTask
		if err := tx.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrImportTaskNotFound
			}
			return fmt.Errorf("查询导入任务失败: %w", err)
		}
		if task.Status != string(contracts.TaskStatusPending) {
			return fmt.Errorf("任务状态 %q 不允许开始（仅 pending）", task.Status)
		}
		return enqueueTaskEvent(tx, taskID)
	})
}
func (r *importTaskRepository) UpdateURLResult(ctx context.Context, taskID, finalURL, sourceHash string) error {
	updates := map[string]any{}
	if finalURL != "" {
		updates["source_url"] = finalURL
	}
	if sourceHash != "" {
		updates["source_hash"] = sourceHash
	}
	if len(updates) == 0 {
		return nil
	}
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{}).
		Where("id = ? AND source_type = 'url'", taskID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新 URL 导入结果失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrImportTaskNotFound
	}
	return nil
}

// UpdateAttachments 保存 zip 导入的附件映射（相对路径 → MinIO object key）。
func (r *importTaskRepository) UpdateAttachments(ctx context.Context, userID, taskID string, attachments map[string]string) error {
	if len(attachments) == 0 {
		return nil
	}
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{}).
		Where("id = ? AND user_id = ?", taskID, userID).Update("attachments", attachments)
	if result.Error != nil {
		return fmt.Errorf("更新导入任务附件映射失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrImportTaskNotFound
	}
	return nil
}

func (r *importTaskRepository) AttachDocument(ctx context.Context, taskID, documentID string) error {
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{}).
		Where("id = ? AND status = 'running'", taskID).Update("document_id", documentID)
	if result.Error != nil {
		return fmt.Errorf("关联导入任务文档失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrImportTaskNotFound
	}
	return nil
}

// Delete 物理删除任务记录。
func (r *importTaskRepository) Delete(ctx context.Context, userID, taskID string) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ?", taskID, userID).Delete(&entity.ImportTask{}).Error
	if err != nil {
		return fmt.Errorf("删除导入任务失败: %w", err)
	}
	return nil
}

// ListByKB 分页查询知识库导入任务。
func (r *importTaskRepository) ListByKB(ctx context.Context, userID, kbID string, page, pageSize int) ([]*entity.ImportTask, int64, error) {
	db := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{})
	db = db.Where("user_id = ? AND knowledge_base_id = ?", userID, kbID)
	// 先 Count 总数再 LIMIT/OFFSET 取当前页，保证列表分页元数据完整。
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计导入任务失败: %w", err)
	}
	var items []*entity.ImportTask
	if err := db.Order("created_at DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("查询导入任务列表失败: %w", err)
	}
	return items, total, nil
}

// ReservePending 使用 FOR UPDATE SKIP LOCKED 领取一个 pending 任务。
// 领取前先恢复卡在 running 且超过租约时间的任务（防运行中 Worker 崩溃导致任务卡死）。
// 领取成功时：将任务置为 running 并写 started_at；无任务时返回 nil, nil。
func (r *importTaskRepository) ReservePending(ctx context.Context) (*entity.ImportTask, error) {
	db := dbFromContext(ctx, r.db)
	if err := r.recoverStaleLocked(ctx, db); err != nil {
		return nil, err
	}
	var task entity.ImportTask
	// 事务内 SELECT ... FOR UPDATE SKIP LOCKED：跳过被其他 Worker 已锁定的行，天然避免并发重复领取。
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Raw(`
			SELECT id, user_id, knowledge_base_id, batch_id, source_path, target_directory_id, source_type,
			       file_name, file_size, mime_type, source_url, source_hash,
			       minio_bucket, minio_object_key, duplicate_policy, status, attempt,
			       current_step, failure_reason, document_id, attachments,
			       created_at, started_at, completed_at
			FROM import_tasks
			WHERE status = 'pending'
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED`).Scan(&task).Error
		if err != nil {
			return fmt.Errorf("领取导入任务失败: %w", err)
		}
		if task.ID == "" {
			return nil
		}
		now := time.Now().UTC()
		// 带 status='pending' 条件做状态迁移，防止行锁释放前被其他进程抢先改为 running。
		result := tx.Model(&entity.ImportTask{}).
			Where("id = ? AND status = 'pending'", task.ID).
			Updates(map[string]any{"status": "running", "started_at": now})
		if result.Error != nil {
			return fmt.Errorf("领取导入任务失败: %w", result.Error)
		}
		// 0 行受影响说明状态已被抢占，返回冲突交由外层转为"无任务"。
		if result.RowsAffected == 0 {
			return ErrImportTaskConflict
		}
		task.Status = "running"
		task.StartedAt = &now
		return nil
	})
	if errors.Is(err, ErrImportTaskConflict) {
		// 另一个 Worker 抢先领取，返回无任务。
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if task.ID == "" {
		return nil, nil
	}
	return &task, nil
}

// ClaimPendingByID 原子认领 Redis Stream 投递的指定任务。
func (r *importTaskRepository) ClaimPendingByID(ctx context.Context, taskID string) (*entity.ImportTask, error) {
	now := time.Now().UTC()
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{}).
		Where("id = ? AND status = 'pending'", taskID).
		Updates(map[string]any{"status": "running", "started_at": now, "completed_at": nil, "attempt": gorm.Expr("attempt + 1")})
	if result.Error != nil {
		return nil, fmt.Errorf("认领导入任务失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return r.FindByIDInternal(ctx, taskID)
}

// recoverStaleLocked 将 running 且 started_at 超过租约的任务恢复为 pending。
// 每次领取前执行，成本低（受 idx_import_tasks_worker 索引约束）。
func (r *importTaskRepository) recoverStaleLocked(ctx context.Context, db *gorm.DB) error {
	staleBefore := time.Now().UTC().Add(-importTaskLease).Unix()
	result := db.WithContext(ctx).Model(&entity.ImportTask{}).
		Where("status = 'running' AND started_at IS NOT NULL AND started_at < ?", time.Unix(staleBefore, 0).UTC()).
		Updates(map[string]any{"status": "pending", "started_at": nil, "failure_reason": "worker 租约过期，任务恢复为待处理"})
	if result.Error != nil {
		return fmt.Errorf("恢复过期导入任务失败: %w", result.Error)
	}
	return nil
}

// RecoverStale 将卡在 running 且 started_at 早于 staleBefore 的任务恢复为 pending。
func (r *importTaskRepository) RecoverStale(ctx context.Context, staleBefore int64) (int64, error) {
	staleTime := time.Unix(staleBefore, 0).UTC()
	var recovered int64
	err := dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var taskIDs []string
		if err := tx.Model(&entity.ImportTask{}).
			Where("status = 'running' AND started_at IS NOT NULL AND started_at < ?", staleTime).
			Pluck("id", &taskIDs).Error; err != nil {
			return fmt.Errorf("查询过期导入任务失败: %w", err)
		}
		if len(taskIDs) == 0 {
			return nil
		}
		result := tx.Model(&entity.ImportTask{}).Where("id IN ? AND status = 'running'", taskIDs).
			Updates(map[string]any{"status": "pending", "started_at": nil, "failure_reason": "消费者租约过期，任务恢复为待处理"})
		if result.Error != nil {
			return fmt.Errorf("恢复过期导入任务失败: %w", result.Error)
		}
		recovered = result.RowsAffected
		for _, taskID := range taskIDs {
			if err := enqueueTaskEvent(tx, taskID); err != nil {
				return err
			}
		}
		return nil
	})
	return recovered, err
}

// CompleteSucceeded 将任务标记为 succeeded 并记录完成时间与关联文档。
// 幂等：允许 running 或已 succeeded 状态（Handler 编排与 Runner 完成回调可能重复调用）。
func (r *importTaskRepository) CompleteSucceeded(ctx context.Context, taskID string, documentID *string) error {
	updates := map[string]any{"status": "succeeded", "completed_at": time.Now().UTC(), "failure_reason": nil}
	if documentID != nil {
		updates["document_id"] = *documentID
	}
	// 允许 running 或已 succeeded：Handler 编排与 Runner 完成回调可能重复调用，保证幂等。
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{}).
		Where("id = ? AND status IN ('running', 'succeeded')", taskID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("完成导入任务失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrImportTaskNotFound
	}
	return nil
}

// FailTask 将任务标记为 failed 并记录失败原因。
func (r *importTaskRepository) FailTask(ctx context.Context, taskID, failureReason string) error {
	// 仅允许 running 状态标记失败，避免终态(succeeded/skipped)被意外覆盖。
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{}).
		Where("id = ? AND status = 'running'", taskID).
		Updates(map[string]any{"status": "failed", "failure_reason": failureReason, "completed_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("标记导入任务失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrImportTaskNotFound
	}
	return nil
}

// RetryTask 将 failed 任务重置为 pending（显式重试）。
func (r *importTaskRepository) RetryTask(ctx context.Context, taskID string) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&entity.ImportTask{}).Where("id = ? AND status = 'failed'", taskID).
			Updates(map[string]any{"status": "pending", "attempt": 0, "failure_reason": nil, "completed_at": nil, "started_at": nil})
		if result.Error != nil {
			return fmt.Errorf("重试导入任务失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrImportTaskNotFound
		}
		return enqueueTaskEvent(tx, taskID)
	})
}

func (r *importTaskRepository) RequeueTask(ctx context.Context, taskID, failureReason string) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&entity.ImportTask{}).Where("id = ? AND status = 'running'", taskID).
			Updates(map[string]any{"status": "pending", "failure_reason": failureReason, "started_at": nil})
		if result.Error != nil {
			return fmt.Errorf("重新排队导入任务失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrImportTaskNotFound
		}
		return enqueueTaskEvent(tx, taskID)
	})
}

func enqueueTaskEvent(db *gorm.DB, taskID string) error {
	payload, err := json.Marshal(map[string]string{"task_id": taskID})
	if err != nil {
		return fmt.Errorf("序列化任务 Outbox 事件失败: %w", err)
	}
	event := &entity.TaskOutbox{EventType: "document.parse", AggregateID: taskID, Payload: string(payload)}
	if err := db.Create(event).Error; err != nil {
		return fmt.Errorf("创建任务 Outbox 事件失败: %w", err)
	}
	return nil
}

// SetRunningStep 更新 running 任务的当前步骤。
func (r *importTaskRepository) SetRunningStep(ctx context.Context, taskID, step string) error {
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{}).
		Where("id = ? AND status = 'running'", taskID).Update("current_step", step)
	if result.Error != nil {
		return fmt.Errorf("更新导入任务步骤失败: %w", result.Error)
	}
	return nil
}

// SkipTask 将任务标记为 skipped。
func (r *importTaskRepository) SkipTask(ctx context.Context, taskID string) error {
	// 去重策略命中时跳过；不限状态直接置为终态 skipped 并记录完成时间。
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.ImportTask{}).
		Where("id = ?", taskID).
		Updates(map[string]any{"status": "skipped", "completed_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("跳过导入任务失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrImportTaskNotFound
	}
	return nil
}

// DeleteCompletedByKB 物理删除知识库内已结束（succeeded/skipped/failed）的导入任务，
// 保留 pending/running 的进行中任务，避免清理到正在处理的记录。
func (r *importTaskRepository) DeleteCompletedByKB(ctx context.Context, userID, kbID string) (int64, error) {
	result := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND knowledge_base_id = ? AND status IN ?",
			userID, kbID,
			[]string{
				string(contracts.TaskStatusSucceeded),
				string(contracts.TaskStatusSkipped),
				string(contracts.TaskStatusFailed),
			}).
		Delete(&entity.ImportTask{})
	if result.Error != nil {
		return 0, fmt.Errorf("清理已完成导入任务失败: %w", result.Error)
	}
	return result.RowsAffected, nil
}
