package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
)

type scheduler struct {
	docs     repository.DocumentRepository
	previews repository.DocumentPreviewRepository
	enabled  bool
	office   RendererInfo
	xlsx     RendererInfo
}

func NewScheduler(docs repository.DocumentRepository, previews repository.DocumentPreviewRepository, enabled bool, office, xlsx RendererInfo) Scheduler {
	return &scheduler{docs: docs, previews: previews, enabled: enabled, office: office, xlsx: xlsx}
}

func (s *scheduler) EnsureDocument(ctx context.Context, documentID string) error {
	if !s.enabled {
		return nil
	}
	doc, err := s.docs.FindByIDInternal(ctx, documentID)
	if err != nil {
		return err
	}
	return s.ensure(ctx, doc)
}

func (s *scheduler) ensure(ctx context.Context, doc *entity.Document) error {
	if doc == nil || doc.MinIOObjectKey == nil || strings.TrimSpace(*doc.MinIOObjectKey) == "" {
		return nil
	}
	kind := classify(doc)
	var specs []struct {
		typ  Type
		info RendererInfo
	}
	switch kind {
	case kindDOCX, kindPPTX:
		if s.office.Enabled {
			specs = append(specs, struct {
				typ  Type
				info RendererInfo
			}{TypePDF, s.office})
		}
	case kindXLSX:
		if s.xlsx.Enabled {
			specs = append(specs, struct {
				typ  Type
				info RendererInfo
			}{TypeTable, s.xlsx})
		}
		if s.office.Enabled {
			specs = append(specs, struct {
				typ  Type
				info RendererInfo
			}{TypePDF, s.office})
		}
	}
	for _, spec := range specs {
		renderHash, err := computeRenderHash(doc, spec.typ, spec.info)
		if err != nil {
			return err
		}
		_, _, err = s.previews.EnsurePendingWithOutbox(ctx, &entity.DocumentPreview{
			UserID: doc.UserID, DocumentID: doc.ID, ContentVersion: doc.ContentVersion,
			PreviewType: string(spec.typ), Status: string(StatusPending), RenderHash: renderHash,
			Renderer: spec.info.Name, RendererVersion: spec.info.Version,
		})
		if err != nil {
			return fmt.Errorf("调度 %s 预览失败: %w", spec.typ, err)
		}
	}
	return nil
}

func computeRenderHash(doc *entity.Document, typ Type, info RendererInfo) (string, error) {
	sourceHash := ""
	if doc.FileHash != nil {
		sourceHash = strings.TrimSpace(*doc.FileHash)
	}
	payload := struct {
		SourceHash      string `json:"source_hash"`
		ContentVersion  int    `json:"content_version"`
		PreviewType     Type   `json:"preview_type"`
		Renderer        string `json:"renderer"`
		RendererVersion string `json:"renderer_version"`
		StrategyVersion string `json:"strategy_version"`
	}{sourceHash, doc.ContentVersion, typ, info.Name, info.Version, info.StrategyVersion}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func findPreview(ctx context.Context, repo repository.DocumentPreviewRepository, doc *entity.Document, typ Type, info RendererInfo) (*entity.DocumentPreview, error) {
	if !info.Enabled {
		return nil, repository.ErrDocumentPreviewNotFound
	}
	hash, err := computeRenderHash(doc, typ, info)
	if err != nil {
		return nil, err
	}
	item, err := repo.FindCurrent(ctx, doc.ID, doc.ContentVersion, string(typ), hash)
	if errors.Is(err, repository.ErrDocumentPreviewNotFound) {
		return nil, err
	}
	return item, err
}
