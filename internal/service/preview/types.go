package preview

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrRenderTimeout = errors.New("预览渲染超时")
	ErrTableTooLarge = errors.New("表格超过结构化预览上限")
)

type Type string

const (
	TypeText     Type = "text"
	TypeMarkdown Type = "markdown"
	TypePDF      Type = "pdf"
	TypeImage    Type = "image"
	TypeTable    Type = "table"
	TypeDownload Type = "download"
	TypeNone     Type = "none"
)

type Status string

const (
	StatusPending     Status = "pending"
	StatusProcessing  Status = "processing"
	StatusReady       Status = "ready"
	StatusFailed      Status = "failed"
	StatusUnsupported Status = "unsupported"
)

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Fallback struct {
	PreviewType Type   `json:"preview_type"`
	Status      Status `json:"status"`
	ContentURL  string `json:"content_url,omitempty"`
	MediaType   string `json:"media_type,omitempty"`
}

type Descriptor struct {
	DocumentID     string     `json:"document_id"`
	ContentVersion int        `json:"content_version"`
	PreviewType    Type       `json:"preview_type"`
	Status         Status     `json:"status"`
	ContentURL     string     `json:"content_url,omitempty"`
	MediaType      string     `json:"media_type,omitempty"`
	OriginalURL    string     `json:"original_url,omitempty"`
	RetryAfterMS   int        `json:"retry_after_ms,omitempty"`
	Fallbacks      []Fallback `json:"fallbacks"`
	Error          *ErrorInfo `json:"error,omitempty"`
}

type TextPreview struct {
	Content string              `json:"content"`
	Format  string              `json:"format"`
	Slides  []PresentationSlide `json:"slides,omitempty"`
}

type PresentationSlide struct {
	Page    int                 `json:"page"`
	Content string              `json:"content"`
	Images  []PresentationImage `json:"images"`
}

type PresentationImage struct {
	URL    string `json:"url"`
	Alt    string `json:"alt"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type SheetSummary struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	RowCount    int    `json:"row_count"`
	ColumnCount int    `json:"column_count"`
}

type TableCell struct {
	Column int    `json:"column"`
	Value  string `json:"value"`
}

type TableRow struct {
	Row   int         `json:"row"`
	Cells []TableCell `json:"cells"`
}

type MergedCell struct {
	StartRow    int `json:"start_row"`
	StartColumn int `json:"start_column"`
	RowSpan     int `json:"row_span"`
	ColumnSpan  int `json:"column_span"`
}

type Workbook struct {
	SchemaVersion string          `json:"schema_version"`
	Sheets        []WorkbookSheet `json:"sheets"`
}

type WorkbookSheet struct {
	Index       int          `json:"index"`
	Name        string       `json:"name"`
	RowCount    int          `json:"row_count"`
	ColumnCount int          `json:"column_count"`
	Rows        []TableRow   `json:"rows"`
	MergedCells []MergedCell `json:"merged_cells"`
}

type TableQuery struct {
	SheetIndex int
	RowOffset  int
	RowLimit   int
}

type TablePage struct {
	DocumentID     string         `json:"document_id"`
	ContentVersion int            `json:"content_version"`
	Sheets         []SheetSummary `json:"sheets"`
	ActiveSheet    int            `json:"active_sheet"`
	RowOffset      int            `json:"row_offset"`
	RowLimit       int            `json:"row_limit"`
	Rows           []TableRow     `json:"rows"`
	MergedCells    []MergedCell   `json:"merged_cells"`
	NextRowOffset  *int           `json:"next_row_offset,omitempty"`
}

type File struct {
	Reader      io.ReadCloser
	FileName    string
	ContentType string
	Size        int64
}

type Service interface {
	GetDescriptor(ctx context.Context, userID, documentID string) (*Descriptor, error)
	GetText(ctx context.Context, userID, documentID string) (*TextPreview, error)
	OpenRendered(ctx context.Context, userID, documentID string) (*File, error)
	GetTable(ctx context.Context, userID, documentID string, query TableQuery) (*TablePage, error)
	Retry(ctx context.Context, userID, documentID string) error
}

type Processor interface {
	Process(ctx context.Context, previewID string) error
}

type Scheduler interface {
	EnsureDocument(ctx context.Context, documentID string) error
}

type RendererInfo struct {
	Enabled         bool
	Name            string
	Version         string
	StrategyVersion string
}

type RenderResult struct {
	Name      string
	MediaType string
	Reader    io.ReadCloser
	Size      int64
}

type Renderer interface {
	Info() RendererInfo
	Render(ctx context.Context, sourceName string, source io.Reader) (*RenderResult, error)
}

type ArtifactManifest struct {
	ArtifactSchemaVersion string         `json:"artifact_schema_version"`
	DocumentID            string         `json:"document_id"`
	ContentVersion        int            `json:"content_version"`
	SourceSHA256          string         `json:"source_sha256"`
	PreviewType           Type           `json:"preview_type"`
	RenderHash            string         `json:"render_hash"`
	Renderer              string         `json:"renderer"`
	RendererVersion       string         `json:"renderer_version"`
	StrategyVersion       string         `json:"strategy_version"`
	Object                ArtifactObject `json:"object"`
	CreatedAt             time.Time      `json:"created_at"`
}

type ArtifactObject struct {
	Key       string `json:"key"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	MediaType string `json:"media_type"`
}
