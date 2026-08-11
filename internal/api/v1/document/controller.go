package document

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/asseturl"
	"github.com/gin-gonic/gin"
)

// Controller 处理文档管理相关的 HTTP 请求。
type Controller struct {
	docs         service.DocumentService
	reader       contracts.DocumentService
	assetSignKey string
}

// NewController 创建一个新的文档控制器实例。
// assetSignKey 用于校验资产下载签名 URL（与 documentService 使用同一密钥）。
func NewController(docs service.DocumentService, reader contracts.DocumentService, assetSignKey string) *Controller {
	return &Controller{docs: docs, reader: reader, assetSignKey: assetSignKey}
}

// CreateManual 手工创建只读知识文档。
func (ctrl *Controller) CreateManual(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.docs.CreateManual(c.Request.Context(), user.ID, c.Param("kb_id"), &req)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}

// List 分页查询知识库文档列表。
func (ctrl *Controller) List(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	filter := request.DocumentListFilter{Keyword: c.Query("keyword")}
	// 可选过滤条件：目录 ID、处理状态、来源类型，均从查询参数解析。
	if value := c.Query("directory_id"); value != "" {
		filter.DirectoryID = &value
	}
	if value := c.Query("processing_status"); value != "" {
		filter.ProcessingStatus = &value
	}
	if value := c.Query("index_mode"); value != "" {
		filter.IndexMode = &value
	}
	if value := c.Query("source_type"); value != "" {
		filter.SourceType = &value
	}
	result, err := ctrl.docs.List(c.Request.Context(), user.ID, c.Param("kb_id"),
		parseIntDefault(c.Query("page"), 1), parseIntDefault(c.Query("page_size"), 20), filter)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Get 查询文档详情。
func (ctrl *Controller) Get(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.docs.Get(c.Request.Context(), user.ID, c.Param("document_id"))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Preview 返回完整解析产物中的阅读版正文，不使用检索 Chunk 拼接。
func (ctrl *Controller) Preview(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.docs.Preview(c.Request.Context(), user.ID, c.Param("document_id"))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Original 流式返回文件导入文档的原始文件；inline=true 时优先交给浏览器预览。
func (ctrl *Controller) Original(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	file, err := ctrl.docs.OpenOriginal(c.Request.Context(), user.ID, c.Param("document_id"))
	if err != nil {
		response.Failure(c, err)
		return
	}
	defer file.Reader.Close()
	disposition := "attachment"
	if c.Query("inline") == "true" {
		disposition = "inline"
	}
	headers := map[string]string{
		"Content-Disposition":    mime.FormatMediaType(disposition, map[string]string{"filename": file.FileName}),
		"Cache-Control":          "private, no-store",
		"X-Content-Type-Options": "nosniff",
	}
	c.DataFromReader(http.StatusOK, file.Size, file.ContentType, file.Reader, headers)
}

// Asset 流式返回文档资产（图片等）字节，用于预览 Markdown 图片。
// 不依赖 Bearer 认证（浏览器 <img> 无法携带 header），改用 HMAC 签名 URL 校验。
func (ctrl *Controller) Asset(c *gin.Context) {
	documentID := c.Param("document_id")
	assetID := c.Param("asset_id")
	exp, sig := c.Query("exp"), c.Query("sig")
	if !asseturl.Verify(ctrl.assetSignKey, documentID, assetID, exp, sig) {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	file, err := ctrl.docs.OpenAsset(c.Request.Context(), "", documentID, assetID)
	if err != nil {
		response.Failure(c, err)
		return
	}
	defer file.Reader.Close()
	headers := map[string]string{
		"Content-Disposition":    mime.FormatMediaType("inline", map[string]string{"filename": file.FileName}),
		"Cache-Control":          "private, max-age=3600",
		"X-Content-Type-Options": "nosniff",
	}
	c.DataFromReader(http.StatusOK, file.Size, file.ContentType, file.Reader, headers)
}

// ReadContent 使用受限正文读取服务按 token 分页返回已索引文档内容和引用。
func (ctrl *Controller) ReadContent(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	if ctrl.reader == nil {
		response.Failure(c, apperrors.New(contracts.ErrServiceUnavailable, nil))
		return
	}
	result, err := ctrl.reader.Read(c.Request.Context(), contracts.DocumentReadRequest{
		UserID:          contracts.ID(user.ID),
		KnowledgeBaseID: contracts.ID(c.Param("kb_id")),
		DocumentID:      contracts.ID(c.Param("document_id")),
		Section:         c.Query("section"),
		Cursor:          c.Query("cursor"),
		MaxTokens:       parseIntDefault(c.Query("max_tokens"), 2000),
	})
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Delete 软删除文档。
func (ctrl *Controller) Delete(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	if err := ctrl.docs.Delete(c.Request.Context(), user.ID, c.Param("document_id")); err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// UploadFiles 文件导入，支持多文件上传到指定知识库。
// 请求格式：multipart/form-data，字段 "files" 为文件列表，可选字段 "directory_id" 和 "duplicate_policy"。
// 每个文件会创建一条 pending 状态的 ImportTask，由 Worker 异步解析。
func (ctrl *Controller) UploadFiles(c *gin.Context) {
	// 1. 从认证中间件获取当前用户
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	// 2. 解析 multipart 表单
	form, err := c.MultipartForm()
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	// 3. 提取 "files" 字段的文件列表，校验非空和数量上限
	headers := form.File["files"]
	if len(headers) == 0 {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	if len(headers) > service.MaxUploadFilesPerRequest {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	// 4. 读取可选表单字段：目标目录 ID 和重复策略（skip/create_new）
	var directoryID *string
	if value := c.PostForm("directory_id"); value != "" {
		directoryID = &value
	}
	duplicatePolicy := c.PostForm("duplicate_policy")

	// 5. 逐个打开文件，校验单文件大小，构建 UploadFileInput 列表
	files := make([]service.UploadFileInput, 0, len(headers))
	closers := make([]io.Closer, 0, len(headers))
	defer func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}()
	for _, header := range headers {
		if header.Size > service.MaxUploadFileSize {
			response.Failure(c, apperrors.New(contracts.ErrPayloadTooLarge, nil))
			return
		}
		file, err := header.Open()
		if err != nil {
			response.Failure(c, apperrors.New(contracts.ErrInternal, err))
			return
		}
		closers = append(closers, file) // 收集文件句柄，函数返回时统一关闭
		files = append(files, service.UploadFileInput{
			FileName: header.Filename,
			Size:     header.Size,
			Reader:   file, // 流式读取，不加载完整文件到内存
		})
	}

	// 6. 调用 Service 执行上传：创建 ImportTask → 流式上传 MinIO → 回写元信息
	result, err := ctrl.docs.UploadFiles(c.Request.Context(), user.ID, c.Param("kb_id"), directoryID, duplicatePolicy, files)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}

// ImportURL 创建异步 URL 导入任务。
func (ctrl *Controller) ImportURL(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.ImportURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.docs.ImportURL(c.Request.Context(), user.ID, c.Param("kb_id"), &req)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusAccepted, result)
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
