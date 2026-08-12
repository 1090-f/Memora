package preview

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
)

type processor struct {
	docs      repository.DocumentRepository
	previews  repository.DocumentPreviewRepository
	store     ObjectStore
	artifacts *ArtifactStore
	renderers map[Type]Renderer
}

func NewProcessor(docs repository.DocumentRepository, previews repository.DocumentPreviewRepository, store ObjectStore, renderers []Renderer, maxWorkbookBytes int64) Processor {
	byType := make(map[Type]Renderer, len(renderers))
	for _, renderer := range renderers {
		if renderer == nil {
			continue
		}
		info := renderer.Info()
		switch info.StrategyVersion {
		case "xlsx-table-v1":
			byType[TypeTable] = renderer
		case "office-pdf-v1":
			byType[TypePDF] = renderer
		}
	}
	return &processor{docs: docs, previews: previews, store: store, artifacts: NewArtifactStore(store, maxWorkbookBytes), renderers: byType}
}

func (p *processor) Process(ctx context.Context, previewID string) error {
	item, err := p.previews.FindByID(ctx, previewID)
	if err != nil {
		return err
	}
	if item.Status != string(StatusProcessing) {
		return nil
	}
	doc, err := p.docs.FindByIDInternal(ctx, item.DocumentID)
	if err != nil {
		return err
	}
	if doc.ContentVersion != item.ContentVersion {
		return fmt.Errorf("预览内容版本已过期: task=%d current=%d", item.ContentVersion, doc.ContentVersion)
	}
	if doc.MinIOObjectKey == nil || strings.TrimSpace(*doc.MinIOObjectKey) == "" {
		return fmt.Errorf("文档没有可渲染的原始文件")
	}
	renderer := p.renderers[Type(item.PreviewType)]
	if renderer == nil || !renderer.Info().Enabled {
		return fmt.Errorf("%w: renderer=%s", ErrUnsupportedRenderer, item.Renderer)
	}
	info := renderer.Info()
	if item.Renderer != info.Name || item.RendererVersion != info.Version {
		return fmt.Errorf("%w: 任务渲染器 %s/%s 与当前 %s/%s 不兼容", ErrUnsupportedRenderer, item.Renderer, item.RendererVersion, info.Name, info.Version)
	}
	source, err := p.store.OpenObject(ctx, *doc.MinIOObjectKey)
	if err != nil {
		return fmt.Errorf("读取预览源文件失败: %w", err)
	}
	defer func() { _ = source.Close() }()
	name := doc.Title
	if doc.OriginalFileName != nil && strings.TrimSpace(*doc.OriginalFileName) != "" {
		name = filepath.Base(strings.ReplaceAll(*doc.OriginalFileName, "\\", "/"))
	}
	result, err := renderer.Render(ctx, name, source)
	if err != nil {
		return err
	}
	sourceSHA := ""
	if doc.FileHash != nil {
		sourceSHA = *doc.FileHash
	}
	manifest, err := p.artifacts.Save(ctx, item, sourceSHA, info.StrategyVersion, result)
	if err != nil {
		return err
	}
	manifestKey := artifactPrefix(item) + "manifest.json"
	if err := p.previews.MarkReady(ctx, item.ID, manifest.Object.Key, manifestKey, manifest.Object.MediaType, manifest.Object.Size); err != nil {
		return err
	}
	return nil
}

var ErrUnsupportedRenderer = errors.New("预览渲染器不可用")

func ErrorCode(err error) contracts.ErrorCode {
	switch {
	case errors.Is(err, ErrRenderTimeout), errors.Is(err, context.DeadlineExceeded):
		return contracts.ErrPreviewRenderTimeout
	case errors.Is(err, ErrTableTooLarge):
		return contracts.ErrPreviewTableTooLarge
	case errors.Is(err, ErrUnsupportedRenderer):
		return contracts.ErrPreviewUnsupported
	case errors.Is(err, ErrArtifactMissing):
		return contracts.ErrPreviewArtifactMissing
	case errors.Is(err, ErrArtifactCorrupt):
		return contracts.ErrPreviewArtifactCorrupt
	default:
		return contracts.ErrPreviewRenderFailed
	}
}
