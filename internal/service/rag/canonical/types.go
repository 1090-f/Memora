// Package canonical defines the stable, source-preserving representation used
// between parsing/normalization and document chunking.
package canonical

import (
	"context"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

const (
	// SchemaVersion is the wire-contract version of CanonicalDocument.
	SchemaVersion = "canonical-v1"
	// DefaultRendererVersion identifies the deterministic ParsedDocument renderer.
	DefaultRendererVersion = "parsed-document-renderer-v1"
)

// NodeKind is the stable semantic kind of a canonical node.
type NodeKind string

const (
	NodeKindHeading    NodeKind = "heading"
	NodeKindParagraph  NodeKind = "paragraph"
	NodeKindListItem   NodeKind = "list_item"
	NodeKindCode       NodeKind = "code"
	NodeKindFormula    NodeKind = "formula"
	NodeKindTable      NodeKind = "table"
	NodeKindTableRow   NodeKind = "table_row"
	NodeKindPicture    NodeKind = "picture"
	NodeKindCaption    NodeKind = "caption"
	NodeKindFootnote   NodeKind = "footnote"
	NodeKindPageHeader NodeKind = "page_header"
	NodeKindPageFooter NodeKind = "page_footer"
	NodeKindUnknown    NodeKind = "unknown"
)

// SourceRef identifies one original parsed object and its source location.
type SourceRef struct {
	BlockID    string    `json:"block_id,omitempty"`
	TableRef   string    `json:"table_ref,omitempty"`
	AssetRef   string    `json:"asset_ref,omitempty"`
	Page       int       `json:"page,omitempty"`
	BBox       []float64 `json:"bbox,omitempty"`
	DoclingRef string    `json:"docling_ref,omitempty"`
}

// SourceSpan maps a UTF-8 byte interval [StartByte, EndByte) to original sources.
// Generated spans explicitly represent renderer/chunker-created text.
type SourceSpan struct {
	StartByte int         `json:"start_byte"`
	EndByte   int         `json:"end_byte"`
	Sources   []SourceRef `json:"sources,omitempty"`
	Generated bool        `json:"generated,omitempty"`
	Reason    string      `json:"reason,omitempty"`
}

// CanonicalNode is an ordered semantic node in the canonical document.
type CanonicalNode struct {
	ID          string         `json:"id"`
	Kind        NodeKind       `json:"kind"`
	StartByte   int            `json:"start_byte"`
	EndByte     int            `json:"end_byte"`
	Text        string         `json:"text,omitempty"`
	Markdown    string         `json:"markdown,omitempty"`
	HeadingPath []string       `json:"heading_path,omitempty"`
	BlockIDs    []string       `json:"block_ids,omitempty"`
	TableRef    string         `json:"table_ref,omitempty"`
	AssetRefs   []string       `json:"asset_refs,omitempty"`
	Sources     []SourceRef    `json:"sources,omitempty"`
	Atomic      bool           `json:"atomic,omitempty"`
	Generated   bool           `json:"generated,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Table       *TableData     `json:"table,omitempty"`
	Pictures    []PictureData  `json:"pictures,omitempty"`
}

// TableData preserves table semantics that cannot be represented losslessly in Markdown.
type TableData struct {
	ID          string      `json:"id"`
	Caption     string      `json:"caption,omitempty"`
	Headers     [][]string  `json:"headers,omitempty"`
	Rows        [][]string  `json:"rows,omitempty"`
	Cells       []TableCell `json:"cells,omitempty"`
	RowCount    int         `json:"row_count,omitempty"`
	ColumnCount int         `json:"column_count,omitempty"`
	PageStart   int         `json:"page_start,omitempty"`
	PageEnd     int         `json:"page_end,omitempty"`
	BBox        []float64   `json:"bbox,omitempty"`
	Markdown    string      `json:"markdown,omitempty"`
}

// TableCell preserves merged-cell relationships.
type TableCell struct {
	Row     int    `json:"row"`
	Column  int    `json:"column"`
	RowSpan int    `json:"row_span,omitempty"`
	ColSpan int    `json:"col_span,omitempty"`
	Text    string `json:"text,omitempty"`
}

// PictureData preserves searchable picture text without carrying binary payloads.
type PictureData struct {
	ID          string    `json:"id"`
	Caption     string    `json:"caption,omitempty"`
	OCRText     string    `json:"ocr_text,omitempty"`
	Description string    `json:"description,omitempty"`
	Omitted     bool      `json:"omitted,omitempty"`
	Page        int       `json:"page,omitempty"`
	BBox        []float64 `json:"bbox,omitempty"`
}

// DocumentProfile contains deterministic routing features derived from nodes.
type DocumentProfile struct {
	SourceFormat           string  `json:"source_format,omitempty"`
	PageCount              int     `json:"page_count,omitempty"`
	DocumentBytes          int     `json:"document_bytes,omitempty"`
	DocumentTokens         int     `json:"document_tokens,omitempty"`
	NodeCount              int     `json:"node_count,omitempty"`
	HeadingCount           int     `json:"heading_count,omitempty"`
	HeadingDepth           int     `json:"heading_depth,omitempty"`
	HeadingCoverage        float64 `json:"heading_coverage,omitempty"`
	ParagraphCount         int     `json:"paragraph_count,omitempty"`
	AverageParagraphTokens float64 `json:"average_paragraph_tokens,omitempty"`
	ParagraphTokenVariance float64 `json:"paragraph_token_variance,omitempty"`
	TableRatio             float64 `json:"table_ratio,omitempty"`
	PictureRatio           float64 `json:"picture_ratio,omitempty"`
	CodeRatio              float64 `json:"code_ratio,omitempty"`
	HasReliableHeadingPath bool    `json:"has_reliable_heading_path,omitempty"`
	WarningCount           int     `json:"warning_count,omitempty"`
}

// CanonicalDocument is the stable chunking input. Markdown is a standard text
// view; Nodes and SourceMap retain structure and provenance.
type CanonicalDocument struct {
	SchemaVersion   string          `json:"schema_version"`
	RendererVersion string          `json:"renderer_version"`
	Markdown        string          `json:"markdown"`
	Nodes           []CanonicalNode `json:"nodes"`
	SourceMap       []SourceSpan    `json:"source_map"`
	Profile         DocumentProfile `json:"profile"`
	ContentHash     string          `json:"content_hash"`
}

// RendererInfo identifies canonical rendering semantics for version hashing.
type RendererInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Identity returns a stable renderer identity.
func (i RendererInfo) Identity() string { return i.Name + ":" + i.Version }

// RenderOptions controls deterministic canonical rendering.
type RenderOptions struct {
	IncludePageHeaders bool `json:"include_page_headers"`
	IncludePageFooters bool `json:"include_page_footers"`
}

// Renderer converts a normalized/enriched ParsedDocument into CanonicalDocument.
type Renderer interface {
	Info() RendererInfo
	Render(ctx context.Context, doc *parser.ParsedDocument) (*CanonicalDocument, error)
}

// Validator validates canonical interval, source and hash invariants.
type Validator interface {
	Validate(doc *CanonicalDocument) error
}

// TokenCounter is the narrow tokenizer dependency used by profiling.
type TokenCounter interface {
	Count(text string) (int, error)
}

// SelectSourceSpans returns canonical spans that reference any requested parsed object.
// Generated separator/marker spans are omitted unless they also carry a matching source.
func SelectSourceSpans(doc *CanonicalDocument, blockIDs, tableRefs, assetRefs []string) []SourceSpan {
	if doc == nil {
		return nil
	}
	blocks, tables, assets := stringSet(blockIDs), stringSet(tableRefs), stringSet(assetRefs)
	out := make([]SourceSpan, 0)
	for _, span := range doc.SourceMap {
		matched := false
		for _, source := range span.Sources {
			if blocks[source.BlockID] || tables[source.TableRef] || assets[source.AssetRef] {
				matched = true
				break
			}
		}
		if matched {
			copySpan := span
			copySpan.Sources = cloneSources(span.Sources)
			out = append(out, copySpan)
		}
	}
	return out
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		if value != "" {
			out[value] = true
		}
	}
	return out
}
