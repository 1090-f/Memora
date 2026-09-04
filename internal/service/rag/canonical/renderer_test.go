package canonical

import (
	"context"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

type runeCounter struct{}

func (runeCounter) Count(value string) (int, error) { return len([]rune(value)), nil }

func canonicalFixture() *parser.ParsedDocument {
	return &parser.ParsedDocument{
		SchemaVersion: parser.SchemaVersion,
		Source:        parser.SourceInfo{Format: "docx"},
		Document:      parser.DocumentInfo{PageCount: 2},
		Blocks: []parser.Block{
			{ID: "h1", Type: parser.BlockTypeHeading, Text: "第一章", HeadingPath: []string{"第一章"}, Source: parser.SourceLocation{Page: 1}},
			{ID: "p1", Type: parser.BlockTypeParagraph, Text: "中文正文。", HeadingPath: []string{"第一章"}, Source: parser.SourceLocation{Page: 1, BBox: []float64{1, 2, 3, 4}}},
			{ID: "t1-block", Type: parser.BlockTypeTable, TableRef: "t1", HeadingPath: []string{"第一章"}, Source: parser.SourceLocation{Page: 1}},
			{ID: "pic1-block", Type: parser.BlockTypePicture, AssetRefs: []string{"a1"}, HeadingPath: []string{"第一章"}, Source: parser.SourceLocation{Page: 2}},
			{ID: "footer", Type: parser.BlockTypePageFooter, Text: "第 1 页", Source: parser.SourceLocation{Page: 1}},
		},
		Tables: []parser.Table{{
			ID: "t1", Caption: "统计表", Headers: [][]string{{"列A", "列B"}}, Rows: [][]string{{"甲", "乙"}},
			Cells: []parser.TableCell{{Row: 0, Column: 0, RowSpan: 1, ColSpan: 2, Text: "合并表头"}}, PageStart: 1,
		}},
		Assets: []parser.Asset{{
			ID: "a1", Kind: "picture", Caption: "架构图", Page: 2,
			Metadata: map[string]any{"ocr_text": "OCR 文本", "description": "系统关系图"},
		}},
	}
}

func TestRendererPreservesTypedStructureAndUTF8Offsets(t *testing.T) {
	renderer := NewParsedDocumentRenderer(RenderOptions{})
	doc, err := renderer.Render(context.Background(), canonicalFixture())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if err := NewValidator().Validate(doc); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	for _, want := range []string{"# 第一章", "中文正文。", "统计表", "OCR 文本", "系统关系图"} {
		if !strings.Contains(doc.Markdown, want) {
			t.Errorf("Markdown missing %q: %q", want, doc.Markdown)
		}
	}
	if strings.Contains(doc.Markdown, "第 1 页") {
		t.Error("page footer should not enter canonical Markdown by default")
	}
	var tableNode, pictureNode *CanonicalNode
	for i := range doc.Nodes {
		switch doc.Nodes[i].Kind {
		case NodeKindTable:
			tableNode = &doc.Nodes[i]
		case NodeKindPicture:
			pictureNode = &doc.Nodes[i]
		}
	}
	if tableNode == nil || tableNode.Table == nil || len(tableNode.Table.Cells) != 1 || tableNode.Table.Cells[0].ColSpan != 2 {
		t.Fatalf("table semantics lost: %+v", tableNode)
	}
	if pictureNode == nil || len(pictureNode.Pictures) != 1 || pictureNode.Pictures[0].OCRText != "OCR 文本" {
		t.Fatalf("picture semantics lost: %+v", pictureNode)
	}
	generated := false
	for _, span := range doc.SourceMap {
		if span.Generated {
			generated = true
			break
		}
	}
	if !generated {
		t.Error("expected generated marker/separator spans")
	}
}

func TestRendererHashIsDeterministicAndProfiled(t *testing.T) {
	renderer := NewParsedDocumentRenderer(RenderOptions{})
	a, err := renderer.Render(context.Background(), canonicalFixture())
	if err != nil {
		t.Fatal(err)
	}
	b, err := renderer.Render(context.Background(), canonicalFixture())
	if err != nil {
		t.Fatal(err)
	}
	if a.ContentHash == "" || a.ContentHash != b.ContentHash {
		t.Fatalf("hash not deterministic: %q != %q", a.ContentHash, b.ContentHash)
	}
	profile, err := Profile(a, canonicalFixture(), runeCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.HeadingCount != 1 || profile.TableRatio == 0 || profile.PictureRatio == 0 || !profile.HasReliableHeadingPath {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestValidatorRejectsBrokenByteBoundary(t *testing.T) {
	doc, err := NewParsedDocumentRenderer(RenderOptions{}).Render(context.Background(), canonicalFixture())
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[1].StartByte++ // falls inside the first UTF-8 rune for 中文正文
	if err := NewValidator().Validate(doc); err == nil {
		t.Fatal("expected invalid UTF-8 boundary or node slice error")
	}
}

func TestCanonicalConfigHashChangesWithRendererOptions(t *testing.T) {
	info := NewParsedDocumentRenderer(RenderOptions{}).Info()
	a, err := ConfigHash(info, RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := ConfigHash(info, RenderOptions{IncludePageHeaders: true})
	if err != nil {
		t.Fatal(err)
	}
	if a == "" || a == b {
		t.Fatalf("renderer option must change canonical config hash: %q == %q", a, b)
	}
}
