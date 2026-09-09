package preview

import (
	"context"
	"errors"
	"io"
	"regexp"
	"sort"
	"strings"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/pkg/asseturl"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"go.uber.org/zap"
)

type service struct {
	docs            repository.DocumentRepository
	previews        repository.DocumentPreviewRepository
	store           ObjectStore
	artifacts       *ArtifactStore
	parseConfigHash string
	assetSignKey    string
	scheduler       Scheduler
	office          RendererInfo
	xlsx            RendererInfo
}

func NewService(
	docs repository.DocumentRepository,
	previews repository.DocumentPreviewRepository,
	store ObjectStore,
	parseConfigHash, assetSignKey string,
	scheduler Scheduler,
	office, xlsx RendererInfo,
	maxWorkbookBytes int64,
) Service {
	return &service{
		docs: docs, previews: previews, store: store,
		artifacts:       NewArtifactStore(store, maxWorkbookBytes),
		parseConfigHash: parseConfigHash, assetSignKey: assetSignKey,
		scheduler: scheduler, office: office, xlsx: xlsx,
	}
}

func (s *service) GetDescriptor(ctx context.Context, userID, documentID string) (*Descriptor, error) {
	doc, err := s.findDocument(ctx, userID, documentID)
	if err != nil {
		return nil, err
	}
	if s.scheduler != nil {
		if err := s.scheduler.EnsureDocument(ctx, doc.ID); err != nil {
			logger.Warn("懒调度文档预览失败", zap.String("document_id", doc.ID), zap.Error(err))
		}
	}

	descriptor := &Descriptor{
		DocumentID: doc.ID, ContentVersion: doc.ContentVersion,
		Fallbacks: make([]Fallback, 0, 3),
	}
	if doc.SourceType == string(contracts.DocumentSourceFile) {
		descriptor.OriginalURL = documentURL(doc.ID, "/original")
	}
	textFallback := s.textFallback(doc)
	downloadFallback := s.downloadFallback(doc)

	switch classify(doc) {
	case kindText:
		descriptor.PreviewType, descriptor.MediaType = TypeText, "application/json"
		s.setDirectText(descriptor, doc)
		descriptor.Fallbacks = appendReady(descriptor.Fallbacks, downloadFallback)
	case kindMarkdown:
		descriptor.PreviewType, descriptor.MediaType = TypeMarkdown, "application/json"
		s.setDirectText(descriptor, doc)
		descriptor.Fallbacks = appendReady(descriptor.Fallbacks, downloadFallback)
	case kindPDF:
		descriptor.PreviewType, descriptor.Status, descriptor.MediaType = TypePDF, StatusReady, "application/pdf"
		descriptor.ContentURL = documentURL(doc.ID, "/original?inline=true")
		descriptor.Fallbacks = appendReady(descriptor.Fallbacks, textFallback, downloadFallback)
	case kindImage:
		descriptor.PreviewType, descriptor.Status = TypeImage, StatusReady
		if doc.MIMEType != nil {
			descriptor.MediaType = *doc.MIMEType
		}
		descriptor.ContentURL = documentURL(doc.ID, "/original?inline=true")
		descriptor.Fallbacks = appendReady(descriptor.Fallbacks, textFallback, downloadFallback)
	case kindDOCX:
		descriptor.PreviewType, descriptor.Status = TypeDOCX, StatusReady
		descriptor.MediaType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		descriptor.ContentURL = documentURL(doc.ID, "/original?inline=true")
		pdfFallback := s.artifactFallback(ctx, doc, TypePDF, s.office, documentURL(doc.ID, "/preview/rendered"), "application/pdf")
		descriptor.Fallbacks = appendReady(descriptor.Fallbacks, pdfFallback, textFallback, downloadFallback)
	case kindPPTX:
		descriptor.PreviewType, descriptor.Status = TypePPTX, StatusReady
		descriptor.MediaType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		descriptor.ContentURL = documentURL(doc.ID, "/original?inline=true")
		pdfFallback := s.artifactFallback(ctx, doc, TypePDF, s.office, documentURL(doc.ID, "/preview/rendered"), "application/pdf")
		descriptor.Fallbacks = appendReady(descriptor.Fallbacks, pdfFallback, textFallback, downloadFallback)
	case kindXLSX:
		descriptor.PreviewType, descriptor.MediaType = TypeTable, "application/json"
		s.applyArtifactState(ctx, descriptor, doc, TypeTable, s.xlsx, documentURL(doc.ID, "/preview/table"))
		pdfFallback := s.artifactFallback(ctx, doc, TypePDF, s.office, documentURL(doc.ID, "/preview/rendered"), "application/pdf")
		descriptor.Fallbacks = appendReady(descriptor.Fallbacks, pdfFallback, textFallback, downloadFallback)
	default:
		descriptor.PreviewType, descriptor.Status = TypeNone, StatusUnsupported
		descriptor.Error = &ErrorInfo{Code: string(contracts.ErrPreviewUnsupported), Message: "该文档类型不支持在线预览"}
		descriptor.Fallbacks = appendReady(descriptor.Fallbacks, downloadFallback)
	}
	return descriptor, nil
}

func (s *service) setDirectText(descriptor *Descriptor, doc *entity.Document) {
	if doc.Content != nil || doc.ActiveIndexVersion != nil {
		descriptor.Status = StatusReady
		descriptor.ContentURL = documentURL(doc.ID, "/preview/text")
		return
	}
	descriptor.Status = StatusProcessing
	descriptor.RetryAfterMS = 2000
}

func (s *service) applyArtifactState(ctx context.Context, descriptor *Descriptor, doc *entity.Document, typ Type, info RendererInfo, readyURL string) {
	if !info.Enabled {
		descriptor.Status = StatusUnsupported
		descriptor.Error = &ErrorInfo{Code: string(contracts.ErrPreviewUnsupported), Message: "预览渲染器未启用"}
		return
	}
	item, err := findPreview(ctx, s.previews, doc, typ, info)
	if errors.Is(err, repository.ErrDocumentPreviewNotFound) {
		descriptor.Status = StatusProcessing
		descriptor.RetryAfterMS = 2000
		return
	}
	if err != nil {
		descriptor.Status = StatusFailed
		descriptor.Error = &ErrorInfo{Code: string(contracts.ErrInternal), Message: "查询预览状态失败"}
		return
	}
	descriptor.Status = Status(item.Status)
	switch descriptor.Status {
	case StatusReady:
		descriptor.ContentURL = readyURL
	case StatusPending, StatusProcessing:
		descriptor.RetryAfterMS = 2000
	case StatusFailed:
		code, message := string(contracts.ErrPreviewRenderFailed), "预览生成失败"
		if item.ErrorCode != nil && *item.ErrorCode != "" {
			code = *item.ErrorCode
		}
		if item.ErrorMessage != nil && *item.ErrorMessage != "" {
			message = *item.ErrorMessage
		}
		descriptor.Error = &ErrorInfo{Code: code, Message: message}
	}
}

func (s *service) artifactFallback(ctx context.Context, doc *entity.Document, typ Type, info RendererInfo, url, mediaType string) Fallback {
	fallback := Fallback{PreviewType: typ, MediaType: mediaType}
	if !info.Enabled {
		fallback.Status = StatusUnsupported
		return fallback
	}
	item, err := findPreview(ctx, s.previews, doc, typ, info)
	if errors.Is(err, repository.ErrDocumentPreviewNotFound) {
		fallback.Status = StatusProcessing
		return fallback
	}
	if err != nil {
		fallback.Status = StatusFailed
		return fallback
	}
	fallback.Status = Status(item.Status)
	if fallback.Status == StatusReady {
		fallback.ContentURL = url
	}
	return fallback
}

func (s *service) textFallback(doc *entity.Document) Fallback {
	typ := TypeMarkdown
	if classify(doc) == kindText {
		typ = TypeText
	}
	status := StatusProcessing
	url := ""
	if doc.Content != nil || doc.ActiveIndexVersion != nil {
		status, url = StatusReady, documentURL(doc.ID, "/preview/text")
	}
	return Fallback{PreviewType: typ, Status: status, ContentURL: url, MediaType: "application/json"}
}

func (s *service) downloadFallback(doc *entity.Document) Fallback {
	if doc.SourceType != string(contracts.DocumentSourceFile) || doc.MinIOObjectKey == nil || strings.TrimSpace(*doc.MinIOObjectKey) == "" {
		return Fallback{PreviewType: TypeDownload, Status: StatusUnsupported}
	}
	return Fallback{PreviewType: TypeDownload, Status: StatusReady, ContentURL: documentURL(doc.ID, "/original"), MediaType: "application/octet-stream"}
}

func appendReady(target []Fallback, items ...Fallback) []Fallback {
	for _, item := range items {
		if item.Status != "" && item.Status != StatusUnsupported {
			target = append(target, item)
		}
	}
	return target
}

func (s *service) GetText(ctx context.Context, userID, documentID string) (*TextPreview, error) {
	doc, err := s.findDocument(ctx, userID, documentID)
	if err != nil {
		return nil, err
	}
	if doc.Content != nil {
		format := doc.ContentFormat
		if format == "" {
			format = "txt"
		}
		return &TextPreview{Content: *doc.Content, Format: format}, nil
	}
	parsed, err := s.loadParsedDocument(ctx, doc)
	if err != nil {
		// 图片正文只是可选的 OCR 结果；历史图片可能没有解析 Artifact，
		// 此时原图仍然是完整可用的预览，不应把“无 OCR 正文”报成资源不存在。
		if classify(doc) == kindImage && errors.Is(err, apperrors.ErrNotFound) {
			return &TextPreview{Content: "", Format: "txt"}, nil
		}
		return nil, err
	}
	content := parsed.Document.Markdown
	if classify(doc) == kindPPTX {
		// PPTX 以页级结构返回抽取正文与同页图片，供前端分栏阅读；Content
		// 保留纯文本版本，兼容尚未消费 Slides 字段的调用方。
		content = stripDoclingImagePlaceholders(content)
		return &TextPreview{
			Content: content,
			Format:  parsed.Source.Format,
			Slides:  s.buildPresentationSlides(parsed, doc.ID),
		}, nil
	}
	if strings.TrimSpace(content) == "" {
		return &TextPreview{Content: "", Format: parsed.Source.Format}, nil
	}
	content = s.rewriteDoclingImagePlaceholders(content, parsed.Blocks, parsed.Assets, doc.ID)
	if strings.EqualFold(parsed.Source.Format, "markdown") {
		content = s.rewriteMarkdownImageRefs(content, parsed.Assets, doc.ID)
	}
	return &TextPreview{Content: content, Format: parsed.Source.Format}, nil
}

func (s *service) OpenRendered(ctx context.Context, userID, documentID string) (*File, error) {
	doc, err := s.findDocument(ctx, userID, documentID)
	if err != nil {
		return nil, err
	}
	if kind := classify(doc); kind != kindDOCX && kind != kindPPTX && kind != kindXLSX {
		return nil, apperrors.New(contracts.ErrPreviewUnsupported, nil)
	}
	item, err := findPreview(ctx, s.previews, doc, TypePDF, s.office)
	if errors.Is(err, repository.ErrDocumentPreviewNotFound) || (err == nil && item.Status != string(StatusReady)) {
		return nil, apperrors.New(contracts.ErrPreviewNotReady, nil)
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	file, err := s.artifacts.Open(ctx, item)
	if err != nil {
		return nil, mapArtifactError(err)
	}
	return file, nil
}

func (s *service) GetTable(ctx context.Context, userID, documentID string, query TableQuery) (*TablePage, error) {
	doc, err := s.findDocument(ctx, userID, documentID)
	if err != nil {
		return nil, err
	}
	if classify(doc) != kindXLSX {
		return nil, apperrors.New(contracts.ErrPreviewUnsupported, nil)
	}
	item, err := findPreview(ctx, s.previews, doc, TypeTable, s.xlsx)
	if errors.Is(err, repository.ErrDocumentPreviewNotFound) || (err == nil && item.Status != string(StatusReady)) {
		return nil, apperrors.New(contracts.ErrPreviewNotReady, nil)
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	workbook, err := s.artifacts.LoadWorkbook(ctx, item)
	if err != nil {
		return nil, mapArtifactError(err)
	}
	return buildTablePage(doc, workbook, query)
}

func buildTablePage(doc *entity.Document, workbook *Workbook, query TableQuery) (*TablePage, error) {
	if query.RowLimit == 0 {
		query.RowLimit = 200
	}
	if query.RowLimit < 20 || query.RowLimit > 500 || query.RowOffset < 0 || query.SheetIndex < 0 || query.SheetIndex >= len(workbook.Sheets) {
		return nil, apperrors.ErrInvalidArgument
	}
	page := &TablePage{
		DocumentID: doc.ID, ContentVersion: doc.ContentVersion, ActiveSheet: query.SheetIndex,
		RowOffset: query.RowOffset, RowLimit: query.RowLimit, Sheets: make([]SheetSummary, 0, len(workbook.Sheets)),
	}
	for _, sheet := range workbook.Sheets {
		page.Sheets = append(page.Sheets, SheetSummary{Index: sheet.Index, Name: sheet.Name, RowCount: sheet.RowCount, ColumnCount: sheet.ColumnCount})
	}
	sheet := workbook.Sheets[query.SheetIndex]
	end := query.RowOffset + query.RowLimit
	if end > sheet.RowCount {
		end = sheet.RowCount
	}
	byRow := make(map[int]TableRow, len(sheet.Rows))
	for _, row := range sheet.Rows {
		byRow[row.Row] = row
	}
	for rowIndex := query.RowOffset; rowIndex < end; rowIndex++ {
		row, ok := byRow[rowIndex]
		if !ok {
			row = TableRow{Row: rowIndex, Cells: []TableCell{}}
		}
		page.Rows = append(page.Rows, row)
	}
	page.MergedCells = sheet.MergedCells
	if end < sheet.RowCount {
		page.NextRowOffset = &end
	}
	return page, nil
}

func (s *service) Retry(ctx context.Context, userID, documentID string) error {
	doc, err := s.findDocument(ctx, userID, documentID)
	if err != nil {
		return err
	}
	if _, err := s.previews.RetryFailed(ctx, userID, documentID, doc.ContentVersion); err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	if s.scheduler != nil {
		if err := s.scheduler.EnsureDocument(ctx, doc.ID); err != nil {
			return apperrors.New(contracts.ErrInternal, err)
		}
	}
	return nil
}

func (s *service) findDocument(ctx context.Context, userID, documentID string) (*entity.Document, error) {
	doc, err := s.docs.FindByID(ctx, userID, documentID)
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return doc, nil
}

func mapArtifactError(err error) error {
	switch {
	case errors.Is(err, ErrArtifactMissing):
		return apperrors.New(contracts.ErrPreviewArtifactMissing, err)
	case errors.Is(err, ErrArtifactCorrupt):
		return apperrors.New(contracts.ErrPreviewArtifactCorrupt, err)
	default:
		return apperrors.New(contracts.ErrInternal, err)
	}
}

func (s *service) loadParsedDocument(ctx context.Context, doc *entity.Document) (*parser.ParsedDocument, error) {
	if doc.ActiveIndexVersion == nil || strings.TrimSpace(s.parseConfigHash) == "" {
		return nil, apperrors.New(contracts.ErrPreviewNotReady, nil)
	}
	prefix := parser.ArtifactKeyPrefix(doc.UserID, doc.ID, doc.ContentVersion, s.parseConfigHash)
	artifactStore := parser.NewArtifactStore(&parserStoreAdapter{inner: s.store}, parser.DefaultValidateLimits())
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

type parserStoreAdapter struct{ inner ObjectStore }

func (p *parserStoreAdapter) OpenObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return p.inner.OpenObject(ctx, key)
}
func (p *parserStoreAdapter) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	return p.inner.PutObject(ctx, key, reader, size, contentType)
}
func (p *parserStoreAdapter) StatObject(ctx context.Context, key string) (*parser.ObjectInfo, error) {
	info, err := p.inner.StatObject(ctx, key)
	if errors.Is(err, objectstore.ErrObjectNotFound) {
		return nil, parser.ErrObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return &parser.ObjectInfo{Key: info.Key, Size: info.Size, ContentType: info.ContentType, ETag: info.ETag}, nil
}
func (p *parserStoreAdapter) RemoveObject(ctx context.Context, key string) error {
	return p.inner.RemoveObject(ctx, key)
}
func (p *parserStoreAdapter) Bucket() string { return "" }

var doclingImagePlaceholderRe = regexp.MustCompile(`<!--\s*image[^>]*-->`)
var markdownImageRefRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
var excessiveBlankLinesRe = regexp.MustCompile(`\n[ \t]*\n(?:[ \t]*\n)+`)

func stripDoclingImagePlaceholders(markdown string) string {
	content := doclingImagePlaceholderRe.ReplaceAllString(markdown, "")
	return strings.TrimSpace(excessiveBlankLinesRe.ReplaceAllString(content, "\n\n"))
}

func (s *service) buildPresentationSlides(parsed *parser.ParsedDocument, documentID string) []PresentationSlide {
	if parsed == nil {
		return nil
	}
	type slideDraft struct {
		parts  []string
		images []PresentationImage
		seen   map[string]struct{}
	}
	drafts := make(map[int]*slideDraft)
	draftFor := func(page int) *slideDraft {
		if page < 1 {
			page = 1
		}
		if drafts[page] == nil {
			drafts[page] = &slideDraft{seen: make(map[string]struct{})}
		}
		return drafts[page]
	}
	assets := make(map[string]parser.Asset, len(parsed.Assets))
	for _, asset := range parsed.Assets {
		assets[asset.ID] = asset
	}
	for _, block := range parsed.Blocks {
		draft := draftFor(block.Source.Page)
		if block.Type == parser.BlockTypePicture {
			for _, ref := range block.AssetRefs {
				asset, ok := assets[ref]
				if !ok || asset.Omitted || strings.TrimSpace(asset.ObjectKey) == "" {
					continue
				}
				if _, exists := draft.seen[asset.ID]; exists {
					continue
				}
				url, err := asseturl.BuildAssetURL(s.assetSignKey, documentID, asset.ID, asseturl.DefaultTTL)
				if err != nil {
					continue
				}
				draft.seen[asset.ID] = struct{}{}
				draft.images = append(draft.images, PresentationImage{
					URL: url, Alt: presentationImageAlt(asset), Width: asset.Width, Height: asset.Height,
				})
			}
			continue
		}
		if part := presentationBlockMarkdown(block); part != "" {
			draft.parts = append(draft.parts, part)
		}
	}

	pages := make([]int, 0, len(drafts))
	for page := range drafts {
		pages = append(pages, page)
	}
	sort.Ints(pages)
	result := make([]PresentationSlide, 0, len(pages))
	for _, page := range pages {
		draft := drafts[page]
		images := draft.images
		if images == nil {
			images = make([]PresentationImage, 0)
		}
		result = append(result, PresentationSlide{
			Page: page, Content: strings.Join(draft.parts, "\n\n"), Images: images,
		})
	}
	return result
}

func presentationBlockMarkdown(block parser.Block) string {
	text := strings.TrimSpace(block.Markdown)
	if text == "" {
		text = strings.TrimSpace(block.Text)
	}
	if text == "" {
		return ""
	}
	switch block.Type {
	case parser.BlockTypeTitle:
		return "### " + text
	case parser.BlockTypeHeading:
		return "#### " + text
	case parser.BlockTypeListItem:
		return "- " + text
	default:
		return text
	}
}

func presentationImageAlt(asset parser.Asset) string {
	alt := strings.Join(strings.Fields(strings.TrimSpace(asset.Caption)), " ")
	if alt == "" {
		return "幻灯片图片"
	}
	return alt
}

func (s *service) rewriteDoclingImagePlaceholders(markdown string, blocks []parser.Block, assets []parser.Asset, documentID string) string {
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
		url, err := asseturl.BuildAssetURL(s.assetSignKey, documentID, asset.ID, asseturl.DefaultTTL)
		if err != nil {
			return match
		}
		return "![图片](" + url + ")"
	})
}

func (s *service) rewriteMarkdownImageRefs(content string, assets []parser.Asset, documentID string) string {
	byRef := make(map[string]string, len(assets))
	for _, asset := range assets {
		if !asset.Omitted && strings.TrimSpace(asset.SourceRef) != "" && strings.TrimSpace(asset.ObjectKey) != "" {
			byRef[asset.SourceRef] = asset.ID
		}
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
		url, err := asseturl.BuildAssetURL(s.assetSignKey, documentID, assetID, asseturl.DefaultTTL)
		if err != nil {
			return match
		}
		return "![" + alt + "](" + url + ")"
	})
}
