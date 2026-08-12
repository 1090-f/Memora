package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/pkg/asseturl"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"go.uber.org/zap"
)

// 文件上传限制（保守默认值，集中定义禁止散落魔法数字）。
const (
	// MaxUploadFileSize 单文件最大字节数（50MB）。
	MaxUploadFileSize = 50 * 1024 * 1024
	// MaxUploadFilesPerRequest 单次请求最大文件数。
	MaxUploadFilesPerRequest = 20
	// MaxManualContentBytes 手工文档正文最大字节数。
	MaxManualContentBytes = 2 * 1024 * 1024
	// minioUploadTimeout 单文件上传超时。
	minioUploadTimeout = 5 * time.Minute
)

// 支持的文件扩展名。
var supportedExtensions = map[string]bool{
	".md": true, ".txt": true, ".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true,
	".jpg": true, ".jpeg": true, ".png": true, ".bmp": true, ".tiff": true, ".tif": true, ".gif": true, ".webp": true,
	".zip": true,
}

// zip 导入限制。
const (
	// maxZipEntries 是 zip 内条目数上限（含目录）。
	maxZipEntries = 100
)

// ObjectStore 是文档服务依赖的对象存储能力接口，便于测试注入。
type ObjectStore interface {
	// Bucket 返回对象存储的默认桶名。
	Bucket() string
	// PutObject 将 reader 流式上传到指定 key，超时由调用方控制。
	PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	// OpenObject 返回对象读取流，调用方负责关闭。
	OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
	// StatObject 返回对象元信息。
	StatObject(ctx context.Context, objectKey string) (*objectstore.ObjectInfo, error)
	// RemoveObject 删除指定对象，用于上传失败的补偿清理。
	RemoveObject(ctx context.Context, objectKey string) error
}

// OfficePDFConverter 抽象 Office 文档转 PDF 能力，便于测试注入。
type OfficePDFConverter interface {
	// Available 报告转换能力是否可用。
	Available() bool
	// ConvertToPDF 将 srcPath 转换为 PDF 输出到 outDir，返回生成的 PDF 路径。
	ConvertToPDF(ctx context.Context, srcPath, outDir string) (string, error)
}

// documentService 是 DocumentService 接口的实现。
// 注意：上传流程不得使用数据库事务包裹 MinIO I/O（规范红线），
// 因此本服务不持有 Transactor；补偿通过显式删除实现。
type documentService struct {
	docs            repository.DocumentRepository
	tasks           repository.ImportTaskRepository
	kbs             repository.KnowledgeBaseRepository
	dirs            repository.DocumentDirectoryRepository
	store           ObjectStore
	parseConfigHash string
	assetSignKey    string
	office          OfficePDFConverter
}

// NewDocumentService 创建一个新的文档服务实例。
// assetSignKey 用于签名资产下载 URL（浏览器 <img> 无法携带 Bearer header）；
// office 为 nil 时 Office 文档的渲染预览返回明确错误。
func NewDocumentService(
	docs repository.DocumentRepository,
	tasks repository.ImportTaskRepository,
	kbs repository.KnowledgeBaseRepository,
	dirs repository.DocumentDirectoryRepository,
	store ObjectStore,
	parseConfigHash string,
	assetSignKey string,
	office *OfficeConverter,
) DocumentService {
	return &documentService{docs: docs, tasks: tasks, kbs: kbs, dirs: dirs, store: store, parseConfigHash: parseConfigHash, assetSignKey: assetSignKey, office: office}
}

// CreateManual 手工创建只读知识文档。
func (s *documentService) CreateManual(ctx context.Context, userID, kbID string, req *request.CreateDocumentRequest) (*dto.DocumentResponse, error) {
	if req == nil || strings.TrimSpace(req.Title) == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	if _, err := s.kbs.FindByID(ctx, userID, kbID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if req.DirectoryID != nil && *req.DirectoryID != "" {
		if _, err := s.dirs.FindByIDInKB(ctx, userID, kbID, *req.DirectoryID); err != nil {
			if errors.Is(err, repository.ErrDirectoryNotFound) {
				return nil, apperrors.New(contracts.ErrInvalidArgument, err)
			}
			return nil, apperrors.New(contracts.ErrInternal, err)
		}
	}
	content := ""
	if req.Content != nil {
		content = *req.Content
	}
	if len([]byte(content)) > MaxManualContentBytes {
		return nil, apperrors.New(contracts.ErrPayloadTooLarge, nil)
	}
	contentFormat := "txt"
	if req.Format != nil && *req.Format == "markdown" {
		contentFormat = "markdown"
	}
	doc := &entity.Document{
		UserID: userID, KnowledgeBaseID: kbID, DirectoryID: req.DirectoryID,
		Title: strings.TrimSpace(req.Title), Content: &content, ContentFormat: contentFormat,
		SourceType:       string(contracts.DocumentSourceManual),
		SourceURL:        req.SourceURL,
		ProcessingStatus: string(contracts.ProcessingPending),
		ContentVersion:   1, ChunkVersion: 1,
	}
	if err := s.docs.Create(ctx, doc); err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	// 手工文档进入索引流水线：创建并关联任务后入队，由 Worker 完成分块、
	// 关键词索引与向量索引（与文件导入共用同一加工 Graph）。
	fileName := manualTaskFileName(doc)
	task := &entity.ImportTask{
		UserID: userID, KnowledgeBaseID: kbID, TargetDirectoryID: req.DirectoryID,
		SourceType:      string(contracts.DocumentSourceManual),
		FileName:        &fileName,
		DuplicatePolicy: "create_new",
		Status:          string(contracts.TaskStatusPending),
		DocumentID:      &doc.ID,
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		// 任务创建/入队失败时补偿删除文档，避免残留永远 pending 的孤文档。
		_ = s.docs.SoftDelete(ctx, userID, doc.ID)
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return documentResponse(doc), nil
}

// manualTaskFileName 为手工文档构造解析文件名：按正文格式追加扩展名，
// 供 ParserRouter 按扩展名路由到 Go Markdown/Text 解析器。
func manualTaskFileName(doc *entity.Document) string {
	ext := ".txt"
	if doc.ContentFormat == "markdown" {
		ext = ".markdown"
	}
	return strings.TrimSpace(doc.Title) + ext
}

// List 分页查询知识库文档列表。
func (s *documentService) List(ctx context.Context, userID, kbID string, page, pageSize int, filter request.DocumentListFilter) (*dto.DocumentList, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if filter.IndexMode != nil {
		switch contracts.DocumentIndexMode(*filter.IndexMode) {
		case contracts.DocumentIndexNone, contracts.DocumentIndexKeyword, contracts.DocumentIndexHybrid:
		default:
			return nil, apperrors.ErrInvalidArgument
		}
	}
	items, total, err := s.docs.ListByKB(ctx, userID, kbID, page, pageSize, repository.DocumentFilter{
		Keyword: filter.Keyword, DirectoryID: filter.DirectoryID,
		ProcessingStatus: filter.ProcessingStatus, IndexMode: filter.IndexMode, SourceType: filter.SourceType,
	})
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	result := &dto.DocumentList{Page: page, PageSize: pageSize, Total: total, Items: make([]*dto.DocumentListItem, 0, len(items))}
	for _, doc := range items {
		result.Items = append(result.Items, &dto.DocumentListItem{
			ID: doc.ID, Title: doc.Title, DirectoryID: doc.DirectoryID,
			SourceType: doc.SourceType, ProcessingStatus: doc.ProcessingStatus, IndexMode: documentIndexMode(doc),
			FileSize: doc.FileSize, CreatedAt: doc.CreatedAt, UpdatedAt: doc.UpdatedAt,
		})
	}
	return result, nil
}

// Get 查询文档详情。
func (s *documentService) Get(ctx context.Context, userID, documentID string) (*dto.DocumentResponse, error) {
	doc, err := s.docs.FindByID(ctx, userID, documentID)
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return documentResponse(doc), nil
}

// Preview 从解析 Artifact 读取完整 Markdown/纯文本，避免用检索 Chunk 反向拼接正文造成结构损失。
func (s *documentService) Preview(ctx context.Context, userID, documentID string) (*dto.DocumentPreviewResponse, error) {
	doc, err := s.docs.FindByID(ctx, userID, documentID)
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if doc.Content != nil {
		format := doc.ContentFormat
		if format == "" {
			format = "txt"
		}
		return &dto.DocumentPreviewResponse{Content: *doc.Content, Format: format}, nil
	}
	parsed, err := s.loadParsedDocument(ctx, userID, doc)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parsed.Document.Markdown) == "" {
		return nil, apperrors.ErrNotFound
	}
	content := parsed.Document.Markdown
	// Docling 输出中图片为 <!-- image --> 占位符（PDF/DOCX/XLSX/PPTX），
	// 按文档顺序替换为签名图片 URL；Markdown 的 ![alt](ref) 引用单独重写。
	content = s.rewriteDoclingImagePlaceholders(content, parsed.Blocks, parsed.Assets, documentID)
	if strings.EqualFold(parsed.Source.Format, "markdown") {
		content = s.rewriteMarkdownImageRefs(content, parsed.Assets, documentID)
	}
	return &dto.DocumentPreviewResponse{Content: content, Format: parsed.Source.Format}, nil
}

// doclingImagePlaceholderRe 匹配 Docling markdown 的图片占位符（<!-- image --> 等）。
var doclingImagePlaceholderRe = regexp.MustCompile(`<!--\s*image[^>]*-->`)

// rewriteDoclingImagePlaceholders 把 Docling 图片占位符按文档顺序替换为签名图片 URL。
// 占位符序号与 picture Block 顺序严格对齐：omitted/无对象的图片保持占位原样。
func (s *documentService) rewriteDoclingImagePlaceholders(markdown string, blocks []parser.Block, assets []parser.Asset, documentID string) string {
	if !doclingImagePlaceholderRe.MatchString(markdown) || s.assetSignKey == "" {
		return markdown
	}
	byID := make(map[string]parser.Asset, len(assets))
	for _, asset := range assets {
		byID[asset.ID] = asset
	}
	var pictureIDs []string
	for _, block := range blocks {
		if block.Type != parser.BlockTypePicture {
			continue
		}
		for _, ref := range block.AssetRefs {
			if _, ok := byID[ref]; ok {
				pictureIDs = append(pictureIDs, ref)
			}
		}
	}
	if len(pictureIDs) == 0 {
		return markdown
	}
	idx := 0
	return doclingImagePlaceholderRe.ReplaceAllStringFunc(markdown, func(match string) string {
		if idx >= len(pictureIDs) {
			return match
		}
		asset := byID[pictureIDs[idx]]
		idx++
		if asset.Omitted || strings.TrimSpace(asset.ObjectKey) == "" {
			return match
		}
		assetURL, err := asseturl.BuildAssetURL(s.assetSignKey, documentID, asset.ID, asseturl.DefaultTTL)
		if err != nil {
			return match
		}
		return "![图片](" + assetURL + ")"
	})
}

// markdownImageRefRe 匹配 Markdown 图片引用 ![alt](ref)。
var markdownImageRefRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)

// rewriteMarkdownImageRefs 把已解析资产的图片引用重写为签名下载 URL；
// 未解析（unresolved）的引用保持原样，便于前端展示与后续排查。
// URL 与前端 API 前缀一致（internal/api/router.go 挂载于 /api/v1）。
func (s *documentService) rewriteMarkdownImageRefs(content string, assets []parser.Asset, documentID string) string {
	byRef := make(map[string]string, len(assets))
	for _, asset := range assets {
		if asset.Omitted || strings.TrimSpace(asset.SourceRef) == "" || strings.TrimSpace(asset.ObjectKey) == "" {
			continue
		}
		byRef[asset.SourceRef] = asset.ID
	}
	if len(byRef) == 0 {
		return content
	}
	return markdownImageRefRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := markdownImageRefRe.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		assetID, ok := byRef[sub[2]]
		if !ok {
			return match
		}
		alt := sub[1]
		if alt == "" {
			alt = "图片"
		}
		assetURL, err := asseturl.BuildAssetURL(s.assetSignKey, documentID, assetID, asseturl.DefaultTTL)
		if err != nil {
			return match
		}
		return "![" + alt + "](" + assetURL + ")"
	})
}

// loadParsedDocument 按当前解析配置加载完整解析产物（Artifact）；缺失视为 NotFound。
func (s *documentService) loadParsedDocument(ctx context.Context, userID string, doc *entity.Document) (*parser.ParsedDocument, error) {
	if doc.ActiveIndexVersion == nil || strings.TrimSpace(s.parseConfigHash) == "" {
		return nil, apperrors.ErrNotFound
	}
	prefix := parser.ArtifactKeyPrefix(userID, doc.ID, doc.ContentVersion, s.parseConfigHash)
	artifactStore := parser.NewArtifactStore(&documentArtifactObjectStore{inner: s.store}, parser.DefaultValidateLimits())
	expectedHash := ""
	if doc.FileHash != nil {
		expectedHash = *doc.FileHash
	}
	ref, err := artifactStore.Resolve(ctx, prefix, expectedHash)
	if err != nil {
		if errors.Is(err, parser.ErrArtifactNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	parsed, err := artifactStore.Load(ctx, ref)
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return parsed, nil
}

// OpenAsset 流式返回文档资产（图片等）字节。
// 调用方必须已通过签名 URL 校验（浏览器 <img> 无法携带 Bearer header）；
// 文档所属用户从实体读取，避免签名 URL 无用户上下文导致 Artifact 前缀错误。
func (s *documentService) OpenAsset(ctx context.Context, userID, documentID, assetID string) (*OriginalDocumentFile, error) {
	doc, err := s.docs.FindByIDInternal(ctx, documentID)
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	parsed, err := s.loadParsedDocument(ctx, doc.UserID, doc)
	if err != nil {
		return nil, err
	}
	var asset *parser.Asset
	for i := range parsed.Assets {
		if parsed.Assets[i].ID == assetID {
			asset = &parsed.Assets[i]
			break
		}
	}
	if asset == nil || asset.Omitted || strings.TrimSpace(asset.ObjectKey) == "" {
		return nil, apperrors.ErrNotFound
	}
	reader, err := s.store.OpenObject(ctx, asset.ObjectKey)
	if err != nil {
		if errors.Is(err, objectstore.ErrObjectNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	contentType := strings.TrimSpace(asset.MIMEType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &OriginalDocumentFile{Reader: reader, FileName: assetID + ".img", ContentType: contentType, Size: -1}, nil
}

// documentArtifactObjectStore 适配文档服务对象存储与 parser ArtifactStore 的元信息类型。
type documentArtifactObjectStore struct{ inner ObjectStore }

func (s *documentArtifactObjectStore) OpenObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.inner.OpenObject(ctx, key)
}

func (s *documentArtifactObjectStore) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	return s.inner.PutObject(ctx, key, reader, size, contentType)
}

func (s *documentArtifactObjectStore) StatObject(ctx context.Context, key string) (*parser.ObjectInfo, error) {
	info, err := s.inner.StatObject(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrObjectNotFound) {
			return nil, parser.ErrObjectNotFound
		}
		return nil, err
	}
	return &parser.ObjectInfo{Key: info.Key, Size: info.Size, ContentType: info.ContentType, ETag: info.ETag}, nil
}

func (s *documentArtifactObjectStore) RemoveObject(ctx context.Context, key string) error {
	return s.inner.RemoveObject(ctx, key)
}

func (s *documentArtifactObjectStore) Bucket() string { return s.inner.Bucket() }

// OpenOriginal 在校验文档所有权后流式返回原始上传文件。
// OpenRendered 返回适合在线预览的 PDF：PDF 直接返回原文件，
// Office 文档（PPTX/DOCX/XLSX）通过 LibreOffice 转换并缓存到 MinIO。
func (s *documentService) OpenRendered(ctx context.Context, userID, documentID string) (*OriginalDocumentFile, error) {
	doc, err := s.docs.FindByID(ctx, userID, documentID)
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if doc.SourceType != string(contracts.DocumentSourceFile) || doc.MinIOObjectKey == nil || strings.TrimSpace(*doc.MinIOObjectKey) == "" {
		return nil, apperrors.ErrNotFound
	}
	fileName := doc.Title
	if doc.OriginalFileName != nil && strings.TrimSpace(*doc.OriginalFileName) != "" {
		fileName = strings.TrimSpace(*doc.OriginalFileName)
	}
	ext := strings.ToLower(path.Ext(fileName))
	// PDF 直接返回原始文件。
	if ext == ".pdf" {
		return s.OpenOriginal(ctx, userID, documentID)
	}
	if !isOfficeRenderableExt(ext) {
		return nil, apperrors.New(contracts.ErrInvalidState, fmt.Errorf("文档类型 %q 不支持渲染预览", ext))
	}

	// 缓存 key 按内容版本隔离：文档重新导入后渲染结果自动失效。
	renderedKey := path.Join("rendered", userID, doc.ID, fmt.Sprintf("v%d.pdf", doc.ContentVersion))
	// 缓存命中时校验 PDF 魔数：历史坏缓存（转换失败但已上传）必须剔除并重转。
	if cached, err := s.loadRenderedCache(ctx, renderedKey); err == nil {
		return cached, nil
	} else if !errors.Is(err, objectstore.ErrObjectNotFound) {
		logger.Warn("渲染缓存校验失败，重新转换", zap.String("document_id", doc.ID), zap.Error(err))
		_ = s.store.RemoveObject(ctx, renderedKey)
	}

	if s.office == nil || !s.office.Available() {
		return nil, apperrors.New(contracts.ErrServiceUnavailable, fmt.Errorf("LibreOffice 不可用，无法生成预览"))
	}
	rendered, err := s.renderOfficeDocument(ctx, doc, renderedKey)
	if err != nil {
		return nil, err
	}
	return rendered, nil
}

// loadRenderedCache 读取渲染缓存并校验 PDF 魔数与大小；对象缺失返回 ErrObjectNotFound。
func (s *documentService) loadRenderedCache(ctx context.Context, key string) (*OriginalDocumentFile, error) {
	reader, err := s.store.OpenObject(ctx, key)
	if err != nil {
		return nil, err
	}
	head := make([]byte, 8)
	n, readErr := io.ReadFull(reader, head)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		_ = reader.Close()
		return nil, readErr
	}
	if n < 8 || !bytes.HasPrefix(head, []byte("%PDF-")) {
		_ = reader.Close()
		return nil, fmt.Errorf("渲染缓存内容不是有效 PDF")
	}
	// 大文件防"空壳"缓存：最小 PDF 应大于 1KB。
	info, statErr := s.store.StatObject(ctx, key)
	if statErr != nil || info.Size < 1024 {
		_ = reader.Close()
		return nil, fmt.Errorf("渲染缓存 PDF 过小")
	}
	// 上面的魔数校验已经消费了文件头。MinIO 返回的是前向流，不能 Seek；必须关闭并
	// 重新打开对象，确保响应从 PDF 的第 0 字节（%PDF-）开始，否则浏览器会拒绝加载。
	if err := reader.Close(); err != nil {
		return nil, err
	}
	reader, err = s.store.OpenObject(ctx, key)
	if err != nil {
		return nil, err
	}
	return &OriginalDocumentFile{Reader: reader, FileName: "preview.pdf", ContentType: "application/pdf", Size: info.Size}, nil
}

// renderOfficeDocument 下载原文件 → LibreOffice 转 PDF → 上传 MinIO 缓存 → 返回流。
func (s *documentService) renderOfficeDocument(ctx context.Context, doc *entity.Document, renderedKey string) (*OriginalDocumentFile, error) {
	tempDir, err := os.MkdirTemp("", "memora-render-*")
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	sourceReader, err := s.store.OpenObject(ctx, *doc.MinIOObjectKey)
	if err != nil {
		if errors.Is(err, objectstore.ErrObjectNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	sourceName := "source"
	if doc.OriginalFileName != nil && strings.TrimSpace(*doc.OriginalFileName) != "" {
		sourceName = filepath.Base(strings.ReplaceAll(strings.TrimSpace(*doc.OriginalFileName), "\\", "/"))
	}
	sourcePath := filepath.Join(tempDir, sourceName)
	sourceFile, err := os.Create(sourcePath)
	if err != nil {
		_ = sourceReader.Close()
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if _, err := io.Copy(sourceFile, sourceReader); err != nil {
		_ = sourceReader.Close()
		_ = sourceFile.Close()
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	_ = sourceReader.Close()
	_ = sourceFile.Close()

	pdfPath, err := s.office.ConvertToPDF(ctx, sourcePath, tempDir)
	if err != nil {
		logger.Warn("Office 转 PDF 失败", zap.String("document_id", doc.ID), zap.Error(err))
		return nil, apperrors.New(contracts.ErrServiceUnavailable, err)
	}
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	// PDF 魔数校验：LibreOffice 可能输出损坏文件，避免缓存无效 PDF 导致预览永久失败。
	if !bytes.HasPrefix(pdfData, []byte("%PDF-")) || len(pdfData) < 8 {
		logger.Warn("Office 转 PDF 输出无效", zap.String("document_id", doc.ID), zap.Int("size", len(pdfData)))
		return nil, apperrors.New(contracts.ErrServiceUnavailable, fmt.Errorf("转换生成的 PDF 无效"))
	}
	logger.Info("Office 转 PDF 完成", zap.String("document_id", doc.ID), zap.Int("size", len(pdfData)))
	if err := s.store.PutObject(ctx, renderedKey, bytes.NewReader(pdfData), int64(len(pdfData)), "application/pdf"); err != nil {
		return nil, apperrors.New(contracts.ErrServiceUnavailable, err)
	}
	reader, err := s.store.OpenObject(ctx, renderedKey)
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	ext := strings.ToLower(path.Ext(sourceName))
	return &OriginalDocumentFile{Reader: reader, FileName: strings.TrimSuffix(sourceName, ext) + ".pdf", ContentType: "application/pdf", Size: -1}, nil
}

// isOfficeRenderableExt 判断扩展名是否支持 LibreOffice 渲染预览。
func isOfficeRenderableExt(ext string) bool {
	switch ext {
	case ".docx", ".xlsx", ".pptx":
		return true
	}
	return false
}

// OpenOriginal 在校验文档所有权后流式返回原始上传文件。
func (s *documentService) OpenOriginal(ctx context.Context, userID, documentID string) (*OriginalDocumentFile, error) {
	doc, err := s.docs.FindByID(ctx, userID, documentID)
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if doc.SourceType != string(contracts.DocumentSourceFile) || doc.MinIOObjectKey == nil || strings.TrimSpace(*doc.MinIOObjectKey) == "" {
		return nil, apperrors.ErrNotFound
	}
	reader, err := s.store.OpenObject(ctx, *doc.MinIOObjectKey)
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	fileName := doc.Title
	if doc.OriginalFileName != nil && strings.TrimSpace(*doc.OriginalFileName) != "" {
		fileName = strings.TrimSpace(*doc.OriginalFileName)
	}
	contentType := "application/octet-stream"
	if doc.MIMEType != nil && strings.TrimSpace(*doc.MIMEType) != "" {
		contentType = strings.TrimSpace(*doc.MIMEType)
	}
	size := int64(-1)
	if doc.FileSize != nil {
		size = *doc.FileSize
	}
	return &OriginalDocumentFile{Reader: reader, FileName: fileName, ContentType: contentType, Size: size}, nil
}

// Delete 软删除文档。
func (s *documentService) Delete(ctx context.Context, userID, documentID string) error {
	err := s.docs.SoftDelete(ctx, userID, documentID)
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("文档已删除", zap.String("user_id", userID), zap.String("document_id", documentID))
	return nil
}

// UploadFiles 文件导入：先创建 import_tasks，再流式上传 MinIO，最后更新任务对象信息。
// 校验阶段（扩展名/大小/数量/归属）全部通过后才开始创建任务；
// 上传阶段任一文件失败时，补偿删除已创建但未完成的任务与已上传对象。
func (s *documentService) UploadFiles(ctx context.Context, userID, kbID string, directoryID *string, duplicatePolicy string, files []UploadFileInput) (*dto.UploadFilesResponse, error) {
	if len(files) == 0 {
		return nil, apperrors.ErrInvalidArgument
	}
	if len(files) > MaxUploadFilesPerRequest {
		return nil, apperrors.New(contracts.ErrInvalidArgument, nil)
	}
	if duplicatePolicy == "" {
		duplicatePolicy = "skip"
	}
	if duplicatePolicy != "create_new" && duplicatePolicy != "skip" {
		return nil, apperrors.ErrInvalidArgument
	}
	for _, file := range files {
		if err := validateUploadFile(file); err != nil {
			return nil, err
		}
	}
	if directoryID != nil && *directoryID != "" {
		if _, err := s.dirs.FindByIDInKB(ctx, userID, kbID, *directoryID); err != nil {
			if errors.Is(err, repository.ErrDirectoryNotFound) {
				return nil, apperrors.New(contracts.ErrInvalidArgument, err)
			}
			return nil, apperrors.New(contracts.ErrInternal, err)
		}
	}
	if _, err := s.kbs.FindByID(ctx, userID, kbID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}

	result := &dto.UploadFilesResponse{Tasks: make([]dto.UploadTaskItem, 0, len(files))}
	bucket := s.store.Bucket()
	createdTaskIDs := make([]string, 0, len(files))
	// 任务创建与对象上传之间不共享数据库事务：MinIO 属外部 I/O 无法纳入事务原子性，
	// 任一步失败改走显式补偿删除（见 compensateUploads），避免长事务持锁与不可回滚的问题。
	for _, file := range files {
		var item *dto.UploadTaskItem
		var objectKey string
		var err error
		switch strings.ToLower(path.Ext(file.FileName)) {
		case ".zip":
			item, objectKey, err = s.uploadZip(ctx, userID, kbID, directoryID, duplicatePolicy, file, bucket)
		case ".md", ".markdown":
			// Markdown 上传后不自动触发解析：等待前端扫描图片引用并确认补传后显式开始。
			item, objectKey, err = s.uploadOneDeferred(ctx, userID, kbID, directoryID, duplicatePolicy, file, bucket)
		default:
			item, objectKey, err = s.uploadOne(ctx, userID, kbID, directoryID, duplicatePolicy, file, bucket)
		}
		if err != nil {
			// 补偿：删除本次请求中已创建的任务与已上传对象，避免残留半成品。
			s.compensateUploads(ctx, userID, createdTaskIDs)
			return nil, err
		}
		createdTaskIDs = append(createdTaskIDs, item.TaskID)
		_ = objectKey
		result.Tasks = append(result.Tasks, *item)
	}
	return result, nil
}

// ImportURL 只创建 pending 任务；网络抓取、SSRF 校验、正文解析和索引全部在 Worker 内执行。
func (s *documentService) ImportURL(ctx context.Context, userID, kbID string, req *request.ImportURLRequest) (*dto.UploadTaskItem, error) {
	if req == nil || len(req.URL) > 4096 {
		return nil, apperrors.ErrInvalidArgument
	}
	target, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || target.Hostname() == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil {
		return nil, apperrors.ErrInvalidArgument
	}
	if _, err := s.kbs.FindByID(ctx, userID, kbID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if req.DirectoryID != nil && *req.DirectoryID != "" {
		if _, err := s.dirs.FindByIDInKB(ctx, userID, kbID, *req.DirectoryID); err != nil {
			if errors.Is(err, repository.ErrDirectoryNotFound) {
				return nil, apperrors.ErrInvalidArgument
			}
			return nil, apperrors.New(contracts.ErrInternal, err)
		}
	}
	policy := req.DuplicatePolicy
	if policy == "" {
		policy = "skip"
	}
	if policy != "skip" && policy != "create_new" {
		return nil, apperrors.ErrInvalidArgument
	}
	sourceURL := target.String()
	task := &entity.ImportTask{
		UserID: userID, KnowledgeBaseID: kbID, TargetDirectoryID: req.DirectoryID,
		SourceType: string(contracts.DocumentSourceURL), SourceURL: &sourceURL,
		DuplicatePolicy: policy, Status: string(contracts.TaskStatusPending),
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return &dto.UploadTaskItem{TaskID: task.ID, FileName: sourceURL, Status: task.Status}, nil
}

// compensateUploads 删除已创建但未成功返回的任务，并尝试删除对应 MinIO 对象。
func (s *documentService) compensateUploads(ctx context.Context, userID string, taskIDs []string) {
	for _, taskID := range taskIDs {
		task, err := s.tasks.FindByID(ctx, userID, taskID)
		if err != nil {
			logger.Warn("补偿查找任务失败", zap.String("user_id", userID), zap.String("task_id", taskID), zap.Error(err))
			continue
		}
		if task.MinIOObjectKey != nil && *task.MinIOObjectKey != "" {
			if removeErr := s.store.RemoveObject(ctx, *task.MinIOObjectKey); removeErr != nil {
				logger.Error("补偿删除 MinIO 对象失败",
					zap.String("user_id", userID), zap.String("object_key", *task.MinIOObjectKey), zap.Error(removeErr))
			}
		}
		if deleteErr := s.tasks.Delete(ctx, userID, taskID); deleteErr != nil {
			logger.Error("补偿删除导入任务失败",
				zap.String("user_id", userID), zap.String("task_id", taskID), zap.Error(deleteErr))
		}
	}
}

// validateUploadFile 校验单个文件的扩展名、大小与空文件。
func validateUploadFile(file UploadFileInput) error {
	ext := strings.ToLower(path.Ext(file.FileName))
	if !supportedExtensions[ext] {
		return apperrors.New(contracts.ErrUnsupportedFileType, fmt.Errorf("不支持的文件类型 %q", file.FileName))
	}
	if file.Size <= 0 {
		return apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("空文件 %q", file.FileName))
	}
	if file.Size > MaxUploadFileSize {
		return apperrors.New(contracts.ErrPayloadTooLarge, nil)
	}
	return nil
}

// uploadOne 处理单个文件：先建 pending 任务，再流式上传 MinIO，最后回写对象信息。
// 三个失败点逐层补偿，避免留下孤儿任务或孤岛对象。
func (s *documentService) uploadOne(ctx context.Context, userID, kbID string, directoryID *string, duplicatePolicy string, file UploadFileInput, bucket string) (*dto.UploadTaskItem, string, error) {
	ext := strings.ToLower(path.Ext(file.FileName))
	mimeType := mimeTypeOf(ext)
	task := &entity.ImportTask{
		UserID: userID, KnowledgeBaseID: kbID, TargetDirectoryID: directoryID,
		SourceType:      string(contracts.DocumentSourceFile),
		FileName:        &file.FileName,
		FileSize:        &file.Size,
		MIMEType:        &mimeType,
		DuplicatePolicy: duplicatePolicy,
		Status:          string(contracts.TaskStatusPending),
	}
	// 失败点 1：任务先落库以拿到稳定 task_id，用其构造对象 key（唯一且可追溯）。
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, "", apperrors.New(contracts.ErrInternal, err)
	}

	objectKey := objectstore.BuildObjectKey(userID, kbID, task.ID, file.FileName)
	hash, err := s.putObjectWithHash(ctx, objectKey, file.Reader, file.Size, mimeType)
	if err != nil {
		// 失败点 2：对象上传失败 → 删除刚创建的任务记录，避免残留 pending 孤儿任务。
		if deleteErr := s.tasks.Delete(ctx, userID, task.ID); deleteErr != nil {
			logger.Error("上传失败后删除任务记录失败",
				zap.String("user_id", userID), zap.String("task_id", task.ID), zap.Error(deleteErr))
		}
		return nil, "", apperrors.New(contracts.ErrServiceUnavailable, err)
	}

	if err := s.tasks.UpdateObjectInfo(ctx, userID, task.ID, bucket, objectKey, &hash); err != nil {
		// 失败点 3：对象已上传但元信息回写失败 → 删除 MinIO 对象，避免孤岛对象。
		// MinIO 上传成功但数据库更新失败：补偿删除对象。
		if removeErr := s.store.RemoveObject(ctx, objectKey); removeErr != nil {
			logger.Error("补偿删除 MinIO 对象失败",
				zap.String("user_id", userID), zap.String("object_key", objectKey), zap.Error(removeErr))
		}
		return nil, "", apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("文件上传完成", zap.String("user_id", userID), zap.String("task_id", task.ID), zap.String("object_key", objectKey))
	return &dto.UploadTaskItem{TaskID: task.ID, FileName: file.FileName, Status: task.Status}, objectKey, nil
}

// uploadOneDeferred 处理 Markdown/ZIP 上传：创建任务并落 MinIO，但不入队触发解析；
// 用户在界面确认图片补传后通过 StartPendingTask 显式开始。
func (s *documentService) uploadOneDeferred(ctx context.Context, userID, kbID string, directoryID *string, duplicatePolicy string, file UploadFileInput, bucket string) (*dto.UploadTaskItem, string, error) {
	ext := strings.ToLower(path.Ext(file.FileName))
	mimeType := mimeTypeOf(ext)
	task := &entity.ImportTask{
		UserID: userID, KnowledgeBaseID: kbID, TargetDirectoryID: directoryID,
		SourceType:      string(contracts.DocumentSourceFile),
		FileName:        &file.FileName,
		FileSize:        &file.Size,
		MIMEType:        &mimeType,
		DuplicatePolicy: duplicatePolicy,
		Status:          string(contracts.TaskStatusPending),
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, "", apperrors.New(contracts.ErrInternal, err)
	}
	objectKey := objectstore.BuildObjectKey(userID, kbID, task.ID, file.FileName)
	hash, err := s.putObjectWithHash(ctx, objectKey, file.Reader, file.Size, mimeType)
	if err != nil {
		if deleteErr := s.tasks.Delete(ctx, userID, task.ID); deleteErr != nil {
			logger.Error("上传失败后删除任务记录失败",
				zap.String("user_id", userID), zap.String("task_id", task.ID), zap.Error(deleteErr))
		}
		return nil, "", apperrors.New(contracts.ErrServiceUnavailable, err)
	}
	if err := s.tasks.UpdateObjectInfoNoEnqueue(ctx, userID, task.ID, bucket, objectKey, &hash); err != nil {
		if removeErr := s.store.RemoveObject(ctx, objectKey); removeErr != nil {
			logger.Error("补偿删除 MinIO 对象失败",
				zap.String("user_id", userID), zap.String("object_key", objectKey), zap.Error(removeErr))
		}
		return nil, "", apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("Markdown/ZIP 上传完成（待确认补传图片）",
		zap.String("user_id", userID), zap.String("task_id", task.ID), zap.String("object_key", objectKey))
	return &dto.UploadTaskItem{TaskID: task.ID, FileName: file.FileName, Status: task.Status}, objectKey, nil
}

// uploadZip 处理 zip 打包导入：zip 内主文档（md/txt/pdf/docx）走正常导入流程，
// 图片附件上传 MinIO 并记录相对路径 → object key 映射，供 Worker 解析 Markdown 图片引用。
// 任一步失败时补偿删除任务与已上传对象。
func (s *documentService) uploadZip(ctx context.Context, userID, kbID string, directoryID *string, duplicatePolicy string, file UploadFileInput, bucket string) (*dto.UploadTaskItem, string, error) {
	data, err := io.ReadAll(io.LimitReader(file.Reader, MaxUploadFileSize+1))
	if err != nil {
		return nil, "", apperrors.New(contracts.ErrServiceUnavailable, err)
	}
	if int64(len(data)) > MaxUploadFileSize {
		return nil, "", apperrors.New(contracts.ErrPayloadTooLarge, nil)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, "", apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("不是有效的 zip 文件 %q", file.FileName))
	}
	if len(reader.File) > maxZipEntries {
		return nil, "", apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("zip 内条目数超过上限 %d", maxZipEntries))
	}

	// 第一遍：定位主文档并收集图片附件条目（zip 内顺序即相对引用顺序）。
	var mainEntry *zip.File
	type imageEntry struct {
		file *zip.File
		name string
	}
	var imageEntries []imageEntry
	for _, entry := range reader.File {
		name := safeZipPath(entry.Name)
		if name == "" {
			continue
		}
		ext := strings.ToLower(path.Ext(name))
		switch {
		case isMainDocumentExt(ext):
			if mainEntry == nil {
				mainEntry = entry
			}
		case isImageExt(ext):
			imageEntries = append(imageEntries, imageEntry{file: entry, name: name})
		}
	}
	if mainEntry == nil {
		return nil, "", apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("zip %q 内没有主文档（md/txt/pdf/docx）", file.FileName))
	}

	// 创建 pending 任务（主文档信息）。
	mainName := safeZipPath(mainEntry.Name)
	mimeType := mimeTypeOf(strings.ToLower(path.Ext(mainName)))
	mainSize := int64(mainEntry.UncompressedSize64)
	task := &entity.ImportTask{
		UserID: userID, KnowledgeBaseID: kbID, TargetDirectoryID: directoryID,
		SourceType:      string(contracts.DocumentSourceFile),
		FileName:        &mainName,
		FileSize:        &mainSize,
		MIMEType:        &mimeType,
		DuplicatePolicy: duplicatePolicy,
		Status:          string(contracts.TaskStatusPending),
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, "", apperrors.New(contracts.ErrInternal, err)
	}
	cleanup := func() {
		_ = s.tasks.Delete(ctx, userID, task.ID)
	}

	// 上传图片附件并收集映射；失败时补偿删除附件对象。
	attachments := make(map[string]string, len(imageEntries))
	uploadedKeys := make([]string, 0, len(imageEntries))
	for _, entry := range imageEntries {
		rc, openErr := entry.file.Open()
		if openErr != nil {
			cleanup()
			s.removeObjects(ctx, uploadedKeys)
			return nil, "", apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("读取 zip 条目 %q 失败: %w", entry.name, openErr))
		}
		entryData, readErr := io.ReadAll(io.LimitReader(rc, MaxUploadFileSize+1))
		_ = rc.Close()
		if readErr != nil {
			cleanup()
			s.removeObjects(ctx, uploadedKeys)
			return nil, "", apperrors.New(contracts.ErrServiceUnavailable, readErr)
		}
		if int64(len(entryData)) > MaxUploadFileSize {
			cleanup()
			s.removeObjects(ctx, uploadedKeys)
			return nil, "", apperrors.New(contracts.ErrPayloadTooLarge, nil)
		}
		ext := strings.ToLower(path.Ext(entry.name))
		key := objectstore.BuildObjectKey(userID, kbID, task.ID, "attachments/"+entry.name)
		if putErr := s.store.PutObject(ctx, key, bytes.NewReader(entryData), int64(len(entryData)), mimeTypeOf(ext)); putErr != nil {
			cleanup()
			s.removeObjects(ctx, uploadedKeys)
			return nil, "", apperrors.New(contracts.ErrServiceUnavailable, putErr)
		}
		uploadedKeys = append(uploadedKeys, key)
		attachments[entry.name] = key
	}

	// 上传主文档内容（流式 + 哈希）。
	mainReader, openErr := mainEntry.Open()
	if openErr != nil {
		cleanup()
		s.removeObjects(ctx, uploadedKeys)
		return nil, "", apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("读取主文档 %q 失败: %w", mainName, openErr))
	}
	objectKey := objectstore.BuildObjectKey(userID, kbID, task.ID, mainName)
	hash, putErr := s.putObjectWithHash(ctx, objectKey, mainReader, mainSize, mimeType)
	_ = mainReader.Close()
	if putErr != nil {
		cleanup()
		s.removeObjects(ctx, uploadedKeys)
		return nil, "", apperrors.New(contracts.ErrServiceUnavailable, putErr)
	}

	// 主文档为 Markdown 时不入队（等待图片补传确认）；其他格式立即进入解析。
	updateErr := s.tasks.UpdateObjectInfo(ctx, userID, task.ID, bucket, objectKey, &hash)
	if strings.EqualFold(path.Ext(mainName), ".md") || strings.EqualFold(path.Ext(mainName), ".markdown") {
		updateErr = s.tasks.UpdateObjectInfoNoEnqueue(ctx, userID, task.ID, bucket, objectKey, &hash)
	}
	if updateErr != nil {
		s.removeObjects(ctx, append(uploadedKeys, objectKey))
		cleanup()
		return nil, "", apperrors.New(contracts.ErrInternal, updateErr)
	}
	if updateErr := s.tasks.UpdateAttachments(ctx, userID, task.ID, attachments); updateErr != nil {
		s.removeObjects(ctx, append(uploadedKeys, objectKey))
		cleanup()
		return nil, "", apperrors.New(contracts.ErrInternal, updateErr)
	}
	logger.Info("zip 导入上传完成",
		zap.String("user_id", userID), zap.String("task_id", task.ID),
		zap.String("object_key", objectKey), zap.Int("attachments", len(attachments)))
	return &dto.UploadTaskItem{TaskID: task.ID, FileName: mainName, Status: task.Status}, objectKey, nil
}

// removeObjects 批量删除对象（补偿用，失败仅记录）。
func (s *documentService) removeObjects(ctx context.Context, keys []string) {
	for _, key := range keys {
		if err := s.store.RemoveObject(ctx, key); err != nil {
			logger.Error("补偿删除 MinIO 对象失败",
				zap.String("object_key", key), zap.Error(err))
		}
	}
}

// safeZipPath 净化 zip 条目路径：统一分隔符、拒绝绝对路径与 .. 穿越。
// 返回 "" 表示应忽略该条目。
func safeZipPath(name string) string {
	cleaned := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(cleaned, "/") {
		return ""
	}
	if len(cleaned) >= 2 && cleaned[1] == ':' {
		return ""
	}
	parts := strings.Split(cleaned, "/")
	var kept []string
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return ""
		}
		kept = append(kept, part)
	}
	return strings.Join(kept, "/")
}

// isMainDocumentExt 判断 zip 内的主文档扩展名。
func isMainDocumentExt(ext string) bool {
	switch ext {
	case ".md", ".markdown", ".txt", ".pdf", ".docx", ".xlsx", ".pptx":
		return true
	}
	return false
}

// isImageExt 判断 zip 内的图片附件扩展名。
func isImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".tiff", ".tif":
		return true
	}
	return false
}

// putObjectWithHash 流式上传并计算 SHA-256，不把完整文件读入内存。
// 上传设置显式超时，避免大文件或网络故障导致请求无限挂起。
func (s *documentService) putObjectWithHash(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) (string, error) {
	uploadCtx, cancel := context.WithTimeout(ctx, minioUploadTimeout)
	defer cancel()
	// TeeReader 在数据流经时同步喂给 SHA-256，实现边上传边计算，无需二次读取。
	hash := sha256.New()
	tee := io.TeeReader(reader, hash)
	if err := s.store.PutObject(uploadCtx, objectKey, tee, size, contentType); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// mimeTypeOf 由扩展名推断 Content-Type：Go 标准库不认识的扩展名补常用类型，最后兜底 octet-stream。
func mimeTypeOf(ext string) string {
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		switch ext {
		case ".md":
			contentType = "text/markdown"
		case ".txt":
			contentType = "text/plain"
		case ".xlsx":
			contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case ".pptx":
			contentType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return contentType
}

// documentResponse 将文档实体转换为响应 DTO，隐藏内部存储结构。
func documentResponse(doc *entity.Document) *dto.DocumentResponse {
	return &dto.DocumentResponse{
		ID: doc.ID, KnowledgeBaseID: doc.KnowledgeBaseID, DirectoryID: doc.DirectoryID,
		Title: doc.Title, Content: doc.Content, ContentFormat: doc.ContentFormat, SourceType: doc.SourceType,
		SourceURL: doc.SourceURL, OriginalFileName: doc.OriginalFileName,
		FileSize: doc.FileSize, MIMEType: doc.MIMEType,
		ProcessingStatus: doc.ProcessingStatus, IndexMode: documentIndexMode(doc), FailureStep: doc.FailureStep, FailureReason: doc.FailureReason,
		ParseWarnings:  doc.ParseWarnings,
		ContentVersion: doc.ContentVersion, ChunkVersion: doc.ChunkVersion,
		ActiveIndexVersion: doc.ActiveIndexVersion, CreatedAt: doc.CreatedAt, UpdatedAt: doc.UpdatedAt,
	}
}

func documentIndexMode(doc *entity.Document) string {
	if doc == nil || doc.ActiveIndexVersion == nil {
		return string(contracts.DocumentIndexNone)
	}
	if doc.EmbeddingModelID != nil && strings.TrimSpace(*doc.EmbeddingModelID) != "" {
		return string(contracts.DocumentIndexHybrid)
	}
	return string(contracts.DocumentIndexKeyword)
}
