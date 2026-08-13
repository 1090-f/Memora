package importtask

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/service"
	"github.com/gin-gonic/gin"
)

// Controller 处理导入任务与文档处理相关的 HTTP 请求。
type Controller struct {
	process service.DocumentProcessService
}

// NewController 创建一个新的导入任务控制器实例。
func NewController(process service.DocumentProcessService) *Controller {
	return &Controller{process: process}
}

// List 分页查询知识库导入任务。
func (ctrl *Controller) List(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, total, err := ctrl.process.ListImportTasks(
		c.Request.Context(),
		contracts.ID(user.ID),
		contracts.ID(c.Param("kb_id")),
		parseIntDefault(c.Query("page"), 1),
		parseIntDefault(c.Query("page_size"), 20),
	)
	if err != nil {
		response.Failure(c, err)
		return
	}
	items := make([]taskResponse, 0, len(result))
	for _, view := range result {
		items = append(items, taskResponseFromView(view))
	}
	response.Success(c, http.StatusOK, gin.H{
		"items": items, "page": parseIntDefault(c.Query("page"), 1),
		"page_size": parseIntDefault(c.Query("page_size"), 20), "total": total,
	})
}

// Get 查询导入任务详情。
func (ctrl *Controller) Get(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	view, err := ctrl.process.GetImportTask(c.Request.Context(), contracts.ID(user.ID), contracts.ID(c.Param("task_id")))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, taskResponseFromView(view))
}

// Retry 显式重试失败的导入任务。
func (ctrl *Controller) Retry(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	if err := ctrl.process.Retry(c.Request.Context(), contracts.ID(user.ID), contracts.ID(c.Param("task_id"))); err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"retried": true})
}

// GetProcessing 查询文档处理状态。
func (ctrl *Controller) GetProcessing(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	status, err := ctrl.process.GetProcessingStatus(
		c.Request.Context(),
		contracts.ID(user.ID),
		contracts.ID(c.Param("document_id")),
	)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{
		"document_id":           status.DocumentID,
		"processing_status":     status.Status,
		"current_index_version": status.IndexVersion,
		"active_index_version":  status.ActiveVersion,
		"failure_step":          status.CurrentStep,
		"failure_reason":        status.FailureReason,
	})
}

// RetryProcessing 重试失败的文档处理（仅允许 processing_status=failed）。
// 任务包 03 阶段：校验状态后返回；真实重试编排在任务包 04 接入解析流水线后实现。
func (ctrl *Controller) RetryProcessing(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	status, err := ctrl.process.GetProcessingStatus(
		c.Request.Context(),
		contracts.ID(user.ID),
		contracts.ID(c.Param("document_id")),
	)
	if err != nil {
		response.Failure(c, err)
		return
	}
	if status.Status != contracts.ProcessingFailed {
		response.Failure(c, apperrors.New(contracts.ErrInvalidState, nil))
		return
	}
	if err := ctrl.process.Reindex(c.Request.Context(), contracts.ID(user.ID), status.KnowledgeBaseID, contracts.ID(c.Param("document_id"))); err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"retried": true})
}

// Reindex 触发文档重新索引。
func (ctrl *Controller) Reindex(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	status, err := ctrl.process.GetProcessingStatus(
		c.Request.Context(),
		contracts.ID(user.ID),
		contracts.ID(c.Param("document_id")),
	)
	if err != nil {
		response.Failure(c, err)
		return
	}
	if err := ctrl.process.Reindex(
		c.Request.Context(),
		contracts.ID(user.ID),
		status.KnowledgeBaseID,
		contracts.ID(c.Param("document_id")),
	); err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{
		"document_id": c.Param("document_id"), "status": "processing",
	})
}

// Cleanup 清理知识库内已结束（succeeded/skipped/failed）的导入任务记录，
// 保留 pending/running 的进行中任务。
func (ctrl *Controller) Cleanup(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	count, err := ctrl.process.CleanupImportTasks(
		c.Request.Context(),
		contracts.ID(user.ID),
		contracts.ID(c.Param("kb_id")),
	)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": count})
}

// ListIndexVersions 返回从 Chunk/Vector 聚合的文档索引版本。
func (ctrl *Controller) ListIndexVersions(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	versions, err := ctrl.process.ListIndexVersions(c.Request.Context(), contracts.ID(user.ID), contracts.ID(c.Param("document_id")))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"items": versions})
}

// Start 显式触发 pending 任务进入解析（Markdown/ZIP 上传后默认等待确认）。
func (ctrl *Controller) Start(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	if err := ctrl.process.StartImportTask(c.Request.Context(), contracts.ID(user.ID), contracts.ID(c.Param("task_id"))); err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"started": true})
}

// Scan 扫描 Markdown 任务的图片引用并分类（内联/网络/已匹配/待补传）。
func (ctrl *Controller) Scan(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.process.ScanImportTask(c.Request.Context(), contracts.ID(user.ID), contracts.ID(c.Param("task_id")))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Attachments 向任务补传图片附件（multipart files 字段）。
func (ctrl *Controller) Attachments(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	headers := form.File["files"]
	if len(headers) == 0 || len(headers) > 20 {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	files := make([]service.UploadFileInput, 0, len(headers))
	closers := make([]io.Closer, 0, len(headers))
	defer func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}()
	for _, header := range headers {
		if header.Size > 32*1024*1024 {
			response.Failure(c, apperrors.New(contracts.ErrPayloadTooLarge, nil))
			return
		}
		file, openErr := header.Open()
		if openErr != nil {
			response.Failure(c, apperrors.New(contracts.ErrInternal, openErr))
			return
		}
		closers = append(closers, file)
		files = append(files, service.UploadFileInput{FileName: header.Filename, Size: header.Size, Reader: file})
	}
	if err := ctrl.process.UploadTaskAttachments(c.Request.Context(), contracts.ID(user.ID), contracts.ID(c.Param("task_id")), files); err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"uploaded": len(files)})
}

// taskResponse 是导入任务的 API 响应结构。
type taskResponse struct {
	ID            string     `json:"id"`
	BatchID       *string    `json:"batch_id,omitempty"`
	SourcePath    *string    `json:"source_path,omitempty"`
	SourceType    string     `json:"source_type"`
	FileName      *string    `json:"file_name,omitempty"`
	FileSize      *int64     `json:"file_size,omitempty"`
	MIMEType      *string    `json:"mime_type,omitempty"`
	SourceURL     *string    `json:"source_url,omitempty"`
	Status        string     `json:"status"`
	CurrentStep   *string    `json:"current_step,omitempty"`
	FailureReason *string    `json:"failure_reason,omitempty"`
	DocumentID    *string    `json:"document_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// taskResponseFromView 将服务层视图转换为 API 响应结构，并处理文档 ID 的指针转换。
func taskResponseFromView(view service.ImportTaskView) taskResponse {
	result := taskResponse{
		ID: string(view.ID), SourceType: string(view.SourceType),
		BatchID: view.BatchID, SourcePath: view.SourcePath,
		FileName: view.FileName, FileSize: view.FileSize, MIMEType: view.MIMEType,
		SourceURL: view.SourceURL, Status: string(view.Status),
		CurrentStep: view.CurrentStep, FailureReason: view.FailureReason,
		CreatedAt: view.CreatedAt, CompletedAt: view.CompletedAt,
	}
	if view.DocumentID != nil {
		id := string(*view.DocumentID)
		result.DocumentID = &id
	}
	return result
}

// parseIntDefault 解析查询参数为整数，缺失或非法时返回默认值。
func parseIntDefault(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
