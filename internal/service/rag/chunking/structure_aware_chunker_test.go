package chunking

import (
	"context"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

func newChunker() *StructureAwareChunker {
	return NewStructureAwareChunker(NewHeuristicTokenizer(), "structure-v1")
}

func textBlock(id, text, heading string, page int) parser.Block {
	var path []string
	if heading != "" {
		path = []string{heading}
	}
	return parser.Block{
		ID: id, Type: parser.BlockTypeParagraph, Text: text, Markdown: text,
		HeadingPath: path, Source: parser.SourceLocation{Page: page},
	}
}

func headingBlock(id, text string) parser.Block {
	return parser.Block{
		ID: id, Type: parser.BlockTypeHeading, Text: text, Markdown: text,
		HeadingPath: []string{text}, Source: parser.SourceLocation{Page: 1},
	}
}

func TestChunkRespectsHeadingGroups(t *testing.T) {
	doc := &parser.ParsedDocument{Blocks: []parser.Block{
		headingBlock("h1", "第一章"),
		textBlock("b1", "第一章的内容", "第一章", 1),
		headingBlock("h2", "第二章"),
		textBlock("b2", "第二章的内容", "第二章", 2),
	}}
	chunks, err := newChunker().Chunk(context.Background(), doc, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("应产出 2 个 Chunk，实际 %d", len(chunks))
	}
	if len(chunks[0].HeadingPath) != 1 || chunks[0].HeadingPath[0] != "第一章" {
		t.Errorf("chunk0 标题路径 = %v", chunks[0].HeadingPath)
	}
	if !strings.Contains(chunks[0].Content, "第一章 / \n第一章") {
		// 前缀 + 内容
		if !strings.Contains(chunks[0].Content, "第一章") {
			t.Errorf("chunk0 内容缺少标题上下文: %q", chunks[0].Content)
		}
	}
	if chunks[0].BlockIDs[0] != "b1" || chunks[1].BlockIDs[0] != "b2" {
		t.Errorf("BlockIDs 错误: %v %v", chunks[0].BlockIDs, chunks[1].BlockIDs)
	}
}

func TestChunkSplitsOverlongBlock(t *testing.T) {
	// 1500 tokens 的单段落（无标题）。
	long := strings.Repeat("很长很长的正文内容。", 100) // 1100 tokens
	doc := &parser.ParsedDocument{Blocks: []parser.Block{
		textBlock("b1", long, "", 1),
	}}
	opts := DefaultChunkOptions()
	opts.MaxTokens = 300
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("超长正文应拆分，实际 %d 个", len(chunks))
	}
	for _, chunk := range chunks {
		if chunk.TokenCount > opts.MaxTokens {
			t.Errorf("Chunk %d tokens 超过上限", chunk.TokenCount)
		}
		if len(chunk.BlockIDs) != 1 || chunk.BlockIDs[0] != "b1" {
			t.Errorf("拆分后 BlockIDs 丢失: %v", chunk.BlockIDs)
		}
		if chunk.SourceLocation.Page != 1 {
			t.Errorf("拆分后页码丢失: %v", chunk.SourceLocation)
		}
	}
}

func TestChunkMergesShortNeighbors(t *testing.T) {
	doc := &parser.ParsedDocument{Blocks: []parser.Block{
		headingBlock("h1", "第一章"),
		textBlock("b1", "短句一", "第一章", 1),
		textBlock("b2", "短句二", "第一章", 1),
	}}
	chunks, err := newChunker().Chunk(context.Background(), doc, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("过短相邻应合并，实际 %d 个", len(chunks))
	}
	if len(chunks[0].BlockIDs) != 2 {
		t.Errorf("合并后 BlockIDs = %v", chunks[0].BlockIDs)
	}
}

func TestChunkDoesNotMergeAcrossSections(t *testing.T) {
	doc := &parser.ParsedDocument{Blocks: []parser.Block{
		headingBlock("h1", "第一章"),
		textBlock("b1", "短句一", "第一章", 1),
		headingBlock("h2", "第二章"),
		textBlock("b2", "短句二", "第二章", 2),
	}}
	chunks, err := newChunker().Chunk(context.Background(), doc, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 2 {
		t.Errorf("不跨一级标题合并，期望 2 个 Chunk，实际 %d", len(chunks))
	}
}

func TestChunkKeepsCodeWhole(t *testing.T) {
	code := "func foo() {\n  return 1\n}\n" + strings.Repeat("// comment\n", 50)
	doc := &parser.ParsedDocument{Blocks: []parser.Block{
		headingBlock("h1", "代码节"),
		{
			ID: "c1", Type: parser.BlockTypeCode, Text: code, Markdown: "```\n" + code + "\n```",
			HeadingPath: []string{"代码节"}, Source: parser.SourceLocation{Page: 1},
		},
	}}
	opts := DefaultChunkOptions()
	opts.MaxTokens = 2000
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("代码应保持完整，期望 1 个 Chunk，实际 %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "```") {
		t.Errorf("代码 Chunk 应保留 markdown 围栏")
	}
	if chunks[0].ContentTypes[0] != parser.BlockTypeCode {
		t.Errorf("ContentTypes = %v", chunks[0].ContentTypes)
	}
}

func TestChunkCaptionMergesWithTable(t *testing.T) {
	table := parser.Table{
		ID: "table-1", Caption: "表 1 数据",
		Headers: [][]string{{"a", "b"}}, Rows: [][]string{{"1", "2"}},
		RowCount: 1, ColumnCount: 2,
	}
	doc := &parser.ParsedDocument{
		Tables: []parser.Table{table},
		Blocks: []parser.Block{
			headingBlock("h1", "第一章"),
			{ID: "cap1", Type: parser.BlockTypeCaption, Text: "表 1 数据", HeadingPath: []string{"第一章"}, Source: parser.SourceLocation{Page: 1}},
			{ID: "tb1", Type: parser.BlockTypeTable, Text: "| a | b |", TableRef: "table-1", HeadingPath: []string{"第一章"}, Source: parser.SourceLocation{Page: 1}},
		},
	}
	chunks, err := newChunker().Chunk(context.Background(), doc, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("caption 应与表格同 Chunk，实际 %d", len(chunks))
	}
	if chunks[0].TableRefs[0] != "table-1" {
		t.Errorf("TableRefs = %v", chunks[0].TableRefs)
	}
	if !strings.Contains(chunks[0].Content, "表 1 数据") {
		t.Errorf("表格 Chunk 应含 caption: %q", chunks[0].Content)
	}
}

func TestChunkRejectsBadOptions(t *testing.T) {
	doc := &parser.ParsedDocument{}
	opts := DefaultChunkOptions()
	opts.MaxTokens = 0
	if _, err := newChunker().Chunk(context.Background(), doc, opts); err == nil {
		t.Error("MaxTokens=0 应报错")
	}
}
