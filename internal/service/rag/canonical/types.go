// Package canonical defines the stable, source-preserving representation used
// between parsing/normalization and document chunking.
package canonical

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

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

// SelectChunkSourceSpans 将实际 Chunk 内容反向定位到 Canonical Markdown，
// 再与 SourceMap 求交。普通长文本拆分因此只携带真正命中的 byte 区间；
// 表格、图片等检索文本包含较多生成前缀、无法逐字定位时，才回退到对象级来源。
func SelectChunkSourceSpans(doc *CanonicalDocument, content string, blockIDs, tableRefs, assetRefs []string) []SourceSpan {
	if doc == nil || strings.TrimSpace(content) == "" {
		return nil
	}
	blocks, tables, assets := stringSet(blockIDs), stringSet(tableRefs), stringSet(assetRefs)
	atoms := chunkContentAtoms(content)
	ranges := make([]byteRange, 0)
	fallbackNodes := make([]CanonicalNode, 0)
	for _, node := range doc.Nodes {
		if !nodeMatches(node, blocks, tables, assets) {
			continue
		}
		segment := canonicalNodeSegment(doc, node)
		nodeRanges := locateAtoms(segment, node.StartByte, atoms)
		if len(nodeRanges) == 0 {
			fallbackNodes = append(fallbackNodes, node)
			continue
		}
		ranges = append(ranges, nodeRanges...)
	}
	ranges = mergeByteRanges(ranges)
	out := intersectSourceMap(doc.SourceMap, ranges, blocks, tables, assets)
	if len(fallbackNodes) == 0 {
		return out
	}

	// 专用 splitter 生成的表头、行范围、图片标签不一定逐字存在于 Canonical
	// Markdown；只为完全无法定位的对象追加保守来源，不扩大已精确定位的文本节点。
	for _, node := range fallbackNodes {
		fallback := SelectSourceSpans(doc, node.BlockIDs, []string{node.TableRef}, node.AssetRefs)
		out = append(out, fallback...)
	}
	return deduplicateSourceSpans(out)
}

type byteRange struct{ start, end int }

func nodeMatches(node CanonicalNode, blocks, tables, assets map[string]bool) bool {
	for _, id := range node.BlockIDs {
		if blocks[id] {
			return true
		}
	}
	if tables[node.TableRef] {
		return true
	}
	for _, id := range node.AssetRefs {
		if assets[id] {
			return true
		}
	}
	return false
}

func canonicalNodeSegment(doc *CanonicalDocument, node CanonicalNode) string {
	if node.StartByte >= 0 && node.EndByte >= node.StartByte && node.EndByte <= len(doc.Markdown) {
		return doc.Markdown[node.StartByte:node.EndByte]
	}
	if node.Markdown != "" {
		return node.Markdown
	}
	return node.Text
}

func chunkContentAtoms(content string) []string {
	seen := make(map[string]bool)
	var atoms []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		atoms = append(atoms, line)
	}
	// 先匹配长文本，避免短标题或常见词抢先命中。
	sort.SliceStable(atoms, func(i, j int) bool { return len(atoms[i]) > len(atoms[j]) })
	return atoms
}

func locateAtoms(segment string, base int, atoms []string) []byteRange {
	var ranges []byteRange
	for _, atom := range atoms {
		searchAt := 0
		for searchAt <= len(segment) {
			index := strings.Index(segment[searchAt:], atom)
			if index < 0 {
				break
			}
			start := searchAt + index
			end := start + len(atom)
			if utf8.ValidString(segment[:start]) && utf8.ValidString(segment[:end]) {
				ranges = append(ranges, byteRange{start: base + start, end: base + end})
			}
			searchAt = end
		}
	}
	return mergeByteRanges(ranges)
}

func mergeByteRanges(ranges []byteRange) []byteRange {
	if len(ranges) < 2 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].start == ranges[j].start {
			return ranges[i].end < ranges[j].end
		}
		return ranges[i].start < ranges[j].start
	})
	out := []byteRange{ranges[0]}
	for _, current := range ranges[1:] {
		last := &out[len(out)-1]
		if current.start <= last.end {
			if current.end > last.end {
				last.end = current.end
			}
			continue
		}
		out = append(out, current)
	}
	return out
}

func intersectSourceMap(sourceMap []SourceSpan, ranges []byteRange, blocks, tables, assets map[string]bool) []SourceSpan {
	var out []SourceSpan
	for _, target := range ranges {
		for _, span := range sourceMap {
			start, end := maxInt(target.start, span.StartByte), minInt(target.end, span.EndByte)
			if start >= end {
				continue
			}
			matched := span.Generated
			for _, source := range span.Sources {
				if blocks[source.BlockID] || tables[source.TableRef] || assets[source.AssetRef] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			copySpan := span
			copySpan.StartByte, copySpan.EndByte = start, end
			copySpan.Sources = cloneSources(span.Sources)
			out = append(out, copySpan)
		}
	}
	return deduplicateSourceSpans(out)
}

func deduplicateSourceSpans(spans []SourceSpan) []SourceSpan {
	seen := make(map[string]bool)
	out := make([]SourceSpan, 0, len(spans))
	for _, span := range spans {
		key := fmtSourceSpanKey(span)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, span)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartByte == out[j].StartByte {
			return out[i].EndByte < out[j].EndByte
		}
		return out[i].StartByte < out[j].StartByte
	})
	return out
}

func fmtSourceSpanKey(span SourceSpan) string {
	var builder strings.Builder
	builder.WriteString(strconv.Itoa(span.StartByte))
	builder.WriteByte(':')
	builder.WriteString(strconv.Itoa(span.EndByte))
	builder.WriteByte(':')
	builder.WriteString(span.Reason)
	if span.Generated {
		builder.WriteString(":generated")
	}
	for _, source := range span.Sources {
		builder.WriteByte('|')
		builder.WriteString(source.BlockID)
		builder.WriteByte('/')
		builder.WriteString(source.TableRef)
		builder.WriteByte('/')
		builder.WriteString(source.AssetRef)
	}
	return builder.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
