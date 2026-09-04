package canonical

import (
	"context"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// ParsedDocumentRenderer deterministically renders normalized parser output.
type ParsedDocumentRenderer struct {
	opts RenderOptions
}

// NewParsedDocumentRenderer creates the default canonical renderer.
func NewParsedDocumentRenderer(opts RenderOptions) *ParsedDocumentRenderer {
	return &ParsedDocumentRenderer{opts: opts}
}

// Info returns the renderer identity used by canonical/chunk hashes.
func (r *ParsedDocumentRenderer) Info() RendererInfo {
	return RendererInfo{Name: "parsed-document", Version: DefaultRendererVersion}
}

type renderSegment struct {
	text      string
	sources   []SourceRef
	generated bool
	reason    string
}

type documentBuilder struct {
	markdown strings.Builder
	spans    []SourceSpan
}

func (b *documentBuilder) append(segment renderSegment) (int, int) {
	start := b.markdown.Len()
	if segment.text == "" {
		return start, start
	}
	b.markdown.WriteString(segment.text)
	end := b.markdown.Len()
	b.spans = append(b.spans, SourceSpan{
		StartByte: start, EndByte: end, Sources: cloneSources(segment.sources),
		Generated: segment.generated, Reason: segment.reason,
	})
	return start, end
}

// Render converts ParsedDocument blocks in reading order and preserves table/asset references.
func (r *ParsedDocumentRenderer) Render(ctx context.Context, doc *parser.ParsedDocument) (*CanonicalDocument, error) {
	if doc == nil {
		return nil, fmt.Errorf("Canonical Renderer 收到 nil ParsedDocument")
	}
	tables := make(map[string]parser.Table, len(doc.Tables))
	for _, table := range doc.Tables {
		tables[table.ID] = table
	}
	assets := make(map[string]parser.Asset, len(doc.Assets))
	for _, asset := range doc.Assets {
		assets[asset.ID] = asset
	}

	out := &CanonicalDocument{
		SchemaVersion: SchemaVersion, RendererVersion: r.Info().Version,
		Nodes: make([]CanonicalNode, 0, len(doc.Blocks)),
	}
	builder := &documentBuilder{}
	for index, block := range doc.Blocks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node, segments := r.renderBlock(index, block, tables, assets)
		if len(segments) > 0 && builder.markdown.Len() > 0 {
			builder.append(renderSegment{text: "\n\n", generated: true, reason: "node_separator"})
		}
		node.StartByte = builder.markdown.Len()
		for _, segment := range segments {
			builder.append(segment)
		}
		node.EndByte = builder.markdown.Len()
		if node.EndByte > node.StartByte {
			node.Markdown = builder.markdown.String()[node.StartByte:node.EndByte]
		}
		out.Nodes = append(out.Nodes, node)
	}
	out.Markdown = builder.markdown.String()
	out.SourceMap = builder.spans
	hash, err := HashDocument(out)
	if err != nil {
		return nil, fmt.Errorf("计算 CanonicalDocument hash 失败: %w", err)
	}
	out.ContentHash = hash
	return out, nil
}

func (r *ParsedDocumentRenderer) renderBlock(index int, block parser.Block, tables map[string]parser.Table, assets map[string]parser.Asset) (CanonicalNode, []renderSegment) {
	node := CanonicalNode{
		ID: block.ID, Kind: nodeKind(block.Type), Text: block.Text,
		HeadingPath: append([]string(nil), block.HeadingPath...),
		BlockIDs:    []string{block.ID}, TableRef: block.TableRef,
		AssetRefs: append([]string(nil), block.AssetRefs...),
		Sources:   []SourceRef{sourceFromBlock(block)},
	}
	if node.ID == "" {
		node.ID = fmt.Sprintf("node-%d", index)
	}
	baseSources := cloneSources(node.Sources)
	sourced := func(text string) renderSegment { return renderSegment{text: text, sources: baseSources} }
	generated := func(text, reason string) renderSegment {
		return renderSegment{text: text, generated: true, reason: reason}
	}

	switch block.Type {
	case parser.BlockTypeTitle, parser.BlockTypeHeading:
		level := len(block.HeadingPath)
		if block.Type == parser.BlockTypeTitle || level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		return node, []renderSegment{generated(strings.Repeat("#", level)+" ", "heading_marker"), sourced(block.Text)}
	case parser.BlockTypeListItem:
		return node, []renderSegment{generated("- ", "list_marker"), sourced(block.Text)}
	case parser.BlockTypeCode:
		node.Atomic = true
		if strings.TrimSpace(block.Markdown) != "" {
			return node, []renderSegment{sourced(block.Markdown)}
		}
		return node, []renderSegment{generated("```\n", "code_fence"), sourced(block.Text), generated("\n```", "code_fence")}
	case parser.BlockTypeFormula:
		node.Atomic = true
		if strings.TrimSpace(block.Markdown) != "" {
			return node, []renderSegment{sourced(block.Markdown)}
		}
		return node, []renderSegment{generated("$$\n", "formula_fence"), sourced(block.Text), generated("\n$$", "formula_fence")}
	case parser.BlockTypeTable:
		node.Atomic = true
		if table, ok := tables[block.TableRef]; ok {
			node.Table = canonicalTable(table)
			tableSource := SourceRef{TableRef: table.ID, Page: table.PageStart, BBox: cloneBBox(table.BBox)}
			node.Sources = appendUniqueSource(node.Sources, tableSource)
			text := strings.TrimSpace(table.Markdown)
			if text == "" {
				text = renderTable(table)
			}
			node.Text = text
			return node, []renderSegment{{text: text, sources: cloneSources(node.Sources)}}
		}
		return node, []renderSegment{sourced(nonEmpty(block.Markdown, block.Text))}
	case parser.BlockTypePicture:
		node.Atomic = true
		var parts []string
		for _, ref := range block.AssetRefs {
			asset, ok := assets[ref]
			if !ok {
				continue
			}
			node.Sources = appendUniqueSource(node.Sources, SourceRef{
				AssetRef: asset.ID, Page: asset.Page, BBox: cloneBBox(asset.BBox),
			})
			picture := PictureData{
				ID: asset.ID, Caption: strings.TrimSpace(asset.Caption), Omitted: asset.Omitted,
				Page: asset.Page, BBox: cloneBBox(asset.BBox),
			}
			if asset.Metadata != nil {
				picture.OCRText, _ = asset.Metadata["ocr_text"].(string)
				picture.Description, _ = asset.Metadata["description"].(string)
			}
			node.Pictures = append(node.Pictures, picture)
			if asset.Omitted {
				continue
			}
			parts = appendUniqueText(parts, asset.Caption)
			if asset.Metadata != nil {
				if value, _ := asset.Metadata["ocr_text"].(string); value != "" {
					parts = appendUniqueText(parts, value)
				}
				if value, _ := asset.Metadata["description"].(string); value != "" {
					parts = appendUniqueText(parts, value)
				}
			}
		}
		node.Text = strings.Join(parts, "\n")
		if node.Text == "" {
			return node, nil
		}
		return node, []renderSegment{generated("[图片] ", "picture_marker"), {text: node.Text, sources: cloneSources(node.Sources)}}
	case parser.BlockTypePageHeader:
		if !r.opts.IncludePageHeaders {
			return node, nil
		}
	case parser.BlockTypePageFooter:
		if !r.opts.IncludePageFooters {
			return node, nil
		}
	}
	return node, []renderSegment{sourced(nonEmpty(block.Markdown, block.Text))}
}

func nodeKind(blockType string) NodeKind {
	switch blockType {
	case parser.BlockTypeTitle, parser.BlockTypeHeading:
		return NodeKindHeading
	case parser.BlockTypeParagraph:
		return NodeKindParagraph
	case parser.BlockTypeListItem:
		return NodeKindListItem
	case parser.BlockTypeCode:
		return NodeKindCode
	case parser.BlockTypeFormula:
		return NodeKindFormula
	case parser.BlockTypeTable:
		return NodeKindTable
	case parser.BlockTypePicture:
		return NodeKindPicture
	case parser.BlockTypeCaption:
		return NodeKindCaption
	case parser.BlockTypeFootnote:
		return NodeKindFootnote
	case parser.BlockTypePageHeader:
		return NodeKindPageHeader
	case parser.BlockTypePageFooter:
		return NodeKindPageFooter
	default:
		return NodeKindUnknown
	}
}

func sourceFromBlock(block parser.Block) SourceRef {
	return SourceRef{
		BlockID: block.ID, TableRef: block.TableRef,
		Page: block.Source.Page, BBox: cloneBBox(block.Source.BBox),
		DoclingRef: block.Source.DoclingRef,
	}
}

func renderTable(table parser.Table) string {
	var lines []string
	if value := strings.TrimSpace(table.Caption); value != "" {
		lines = append(lines, value)
	}
	for _, header := range table.Headers {
		lines = append(lines, "| "+strings.Join(header, " | ")+" |")
	}
	for _, row := range table.Rows {
		lines = append(lines, "| "+strings.Join(row, " | ")+" |")
	}
	return strings.Join(lines, "\n")
}

func canonicalTable(table parser.Table) *TableData {
	cells := make([]TableCell, len(table.Cells))
	for i, cell := range table.Cells {
		cells[i] = TableCell{
			Row: cell.Row, Column: cell.Column, RowSpan: cell.RowSpan,
			ColSpan: cell.ColSpan, Text: cell.Text,
		}
	}
	return &TableData{
		ID: table.ID, Caption: table.Caption, Headers: cloneMatrix(table.Headers),
		Rows: cloneMatrix(table.Rows), Cells: cells, RowCount: table.RowCount,
		ColumnCount: table.ColumnCount, PageStart: table.PageStart, PageEnd: table.PageEnd,
		BBox: cloneBBox(table.BBox), Markdown: table.Markdown,
	}
}

func cloneMatrix(value [][]string) [][]string {
	out := make([][]string, len(value))
	for i := range value {
		out[i] = append([]string(nil), value[i]...)
	}
	return out
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func appendUniqueText(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueSource(values []SourceRef, value SourceRef) []SourceRef {
	for _, existing := range values {
		if sameSource(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func sameSource(a, b SourceRef) bool {
	return a.BlockID == b.BlockID && a.TableRef == b.TableRef && a.AssetRef == b.AssetRef &&
		a.Page == b.Page && a.DoclingRef == b.DoclingRef && fmt.Sprint(a.BBox) == fmt.Sprint(b.BBox)
}

func cloneSources(values []SourceRef) []SourceRef {
	out := make([]SourceRef, len(values))
	for i, value := range values {
		out[i] = value
		out[i].BBox = cloneBBox(value.BBox)
	}
	return out
}

func cloneBBox(value []float64) []float64 { return append([]float64(nil), value...) }
