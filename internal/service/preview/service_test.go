package preview

import (
	"context"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
)

type descriptorDocumentRepository struct {
	repository.DocumentRepository
	document *entity.Document
}

func (r *descriptorDocumentRepository) FindByID(_ context.Context, _, _ string) (*entity.Document, error) {
	return r.document, nil
}

func (r *descriptorDocumentRepository) FindByIDInternal(_ context.Context, _ string) (*entity.Document, error) {
	return r.document, nil
}

type descriptorPreviewRepository struct {
	repository.DocumentPreviewRepository
	current *entity.DocumentPreview
	findErr error
	ensured []*entity.DocumentPreview
}

func (r *descriptorPreviewRepository) FindCurrent(_ context.Context, _ string, _ int, _, _ string) (*entity.DocumentPreview, error) {
	return r.current, r.findErr
}

func (r *descriptorPreviewRepository) EnsurePendingWithOutbox(_ context.Context, item *entity.DocumentPreview) (*entity.DocumentPreview, bool, error) {
	r.ensured = append(r.ensured, item)
	return item, true, nil
}

func officeDocument(name, mimeType string) *entity.Document {
	objectKey := "original/doc"
	return &entity.Document{
		BaseEntity:       entity.BaseEntity{ID: "doc-1"},
		UserID:           "user-1",
		SourceType:       string(contracts.DocumentSourceFile),
		Title:            name,
		OriginalFileName: &name,
		MIMEType:         &mimeType,
		MinIOObjectKey:   &objectKey,
		ContentVersion:   2,
	}
}

func TestOfficeDescriptorUsesBrowserPreviewAndOptionalPDFFallback(t *testing.T) {
	tests := []struct {
		name        string
		fileName    string
		mimeType    string
		previewType Type
	}{
		{name: "docx", fileName: "report.docx", mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", previewType: TypeDOCX},
		{name: "pptx", fileName: "slides.pptx", mimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", previewType: TypePPTX},
	}

	for _, tt := range tests {
		t.Run(tt.name+" without LibreOffice", func(t *testing.T) {
			doc := officeDocument(tt.fileName, tt.mimeType)
			repo := &descriptorPreviewRepository{}
			svc := NewService(&descriptorDocumentRepository{document: doc}, repo, nil, "", "", nil,
				RendererInfo{Enabled: false, Name: "libreoffice", Version: "disabled"}, RendererInfo{}, 1024)

			descriptor, err := svc.GetDescriptor(context.Background(), "user-1", doc.ID)
			if err != nil {
				t.Fatalf("GetDescriptor() error = %v", err)
			}
			if descriptor.PreviewType != tt.previewType || descriptor.Status != StatusReady {
				t.Fatalf("primary preview = %s/%s, want %s/ready", descriptor.PreviewType, descriptor.Status, tt.previewType)
			}
			if descriptor.ContentURL != "/api/v1/documents/doc-1/original?inline=true" {
				t.Fatalf("ContentURL = %q", descriptor.ContentURL)
			}
			for _, fallback := range descriptor.Fallbacks {
				if fallback.PreviewType == TypePDF {
					t.Fatal("disabled LibreOffice must not expose a PDF fallback")
				}
			}
		})

		t.Run(tt.name+" with LibreOffice", func(t *testing.T) {
			doc := officeDocument(tt.fileName, tt.mimeType)
			repo := &descriptorPreviewRepository{findErr: repository.ErrDocumentPreviewNotFound}
			office := RendererInfo{Enabled: true, Name: "libreoffice", Version: "1", StrategyVersion: "office-pdf-v1"}
			scheduler := NewScheduler(&descriptorDocumentRepository{document: doc}, repo, true, office, RendererInfo{})
			svc := NewService(&descriptorDocumentRepository{document: doc}, repo, nil, "", "", scheduler, office, RendererInfo{}, 1024)

			descriptor, err := svc.GetDescriptor(context.Background(), "user-1", doc.ID)
			if err != nil {
				t.Fatalf("GetDescriptor() error = %v", err)
			}
			if len(repo.ensured) != 1 || repo.ensured[0].PreviewType != string(TypePDF) {
				t.Fatalf("scheduled previews = %#v, want one PDF fallback", repo.ensured)
			}
			foundPDF := false
			for _, fallback := range descriptor.Fallbacks {
				if fallback.PreviewType == TypePDF {
					foundPDF = fallback.Status == StatusProcessing
				}
			}
			if !foundPDF {
				t.Fatal("enabled LibreOffice must expose the processing PDF fallback")
			}
		})
	}
}

func TestXLSXStillSchedulesTableWithoutLibreOffice(t *testing.T) {
	doc := officeDocument("book.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	repo := &descriptorPreviewRepository{}
	xlsx := RendererInfo{Enabled: true, Name: "go-openxml", Version: "1", StrategyVersion: "xlsx-table-v1"}
	scheduler := NewScheduler(&descriptorDocumentRepository{document: doc}, repo, true,
		RendererInfo{Enabled: false, Name: "libreoffice"}, xlsx)

	if err := scheduler.EnsureDocument(context.Background(), doc.ID); err != nil {
		t.Fatalf("EnsureDocument() error = %v", err)
	}
	if len(repo.ensured) != 1 || repo.ensured[0].PreviewType != string(TypeTable) {
		t.Fatalf("scheduled previews = %#v, want one table preview", repo.ensured)
	}
}
