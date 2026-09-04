package chunking

import (
	"context"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/canonical"
)

func TestCanonicalStructuredChunkPreservesMultipleSources(t *testing.T) {
	doc := &canonical.CanonicalDocument{
		SchemaVersion: canonical.SchemaVersion, RendererVersion: canonical.DefaultRendererVersion,
		Markdown: "第一页正文\n\n第二页正文",
		Profile:  canonical.DocumentProfile{SourceFormat: "docx", PageCount: 2},
		Nodes: []canonical.CanonicalNode{
			{ID: "b1", Kind: canonical.NodeKindParagraph, StartByte: 0, EndByte: len("第一页正文"), Text: "第一页正文", Markdown: "第一页正文", BlockIDs: []string{"b1"}, Sources: []canonical.SourceRef{{BlockID: "b1", Page: 1}}},
			{ID: "b2", Kind: canonical.NodeKindParagraph, StartByte: len("第一页正文\n\n"), EndByte: len("第一页正文\n\n第二页正文"), Text: "第二页正文", Markdown: "第二页正文", BlockIDs: []string{"b2"}, Sources: []canonical.SourceRef{{BlockID: "b2", Page: 2}}},
		},
		SourceMap: []canonical.SourceSpan{
			{StartByte: 0, EndByte: len("第一页正文"), Sources: []canonical.SourceRef{{BlockID: "b1", Page: 1}}},
			{StartByte: len("第一页正文"), EndByte: len("第一页正文\n\n"), Generated: true, Reason: "node_separator"},
			{StartByte: len("第一页正文\n\n"), EndByte: len("第一页正文\n\n第二页正文"), Sources: []canonical.SourceRef{{BlockID: "b2", Page: 2}}},
		},
	}
	chunks, err := newChunker().ChunkCanonicalStrategy(context.Background(), doc, DefaultChunkOptions(), StrategyStructured)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want 1", len(chunks))
	}
	pages := map[int]bool{}
	for _, span := range chunks[0].SourceSpans {
		for _, source := range span.Sources {
			pages[source.Page] = true
		}
	}
	if !pages[1] || !pages[2] {
		t.Fatalf("multi-page sources lost: %+v", chunks[0].SourceSpans)
	}
}

func TestCanonicalSpecializedNodesUseTypedPayload(t *testing.T) {
	doc := &canonical.CanonicalDocument{
		SchemaVersion: canonical.SchemaVersion, RendererVersion: canonical.DefaultRendererVersion,
		Profile: canonical.DocumentProfile{SourceFormat: "xlsx"},
		Nodes: []canonical.CanonicalNode{
			{
				ID: "table-node", Kind: canonical.NodeKindTable, BlockIDs: []string{"table-block", "caption-block"},
				TableRef: "table-1", HeadingPath: []string{"报表"}, Sources: []canonical.SourceRef{{BlockID: "table-block", Page: 2}},
				Table: &canonical.TableData{ID: "table-1", Caption: "收入表", Headers: [][]string{{"月份", "收入"}}, Rows: [][]string{{"一月", "100"}}},
			},
			{
				ID: "picture-node", Kind: canonical.NodeKindPicture, BlockIDs: []string{"picture-block"}, AssetRefs: []string{"asset-1"},
				Sources:  []canonical.SourceRef{{BlockID: "picture-block", AssetRef: "asset-1", Page: 3}},
				Pictures: []canonical.PictureData{{ID: "asset-1", Caption: "图 1", OCRText: "系统架构", Description: "服务关系图"}},
			},
		},
	}
	opts := DefaultChunkOptions()
	opts.MinTokens = 0
	chunks, err := newChunker().ChunkCanonical(context.Background(), doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("typed 专用节点 chunks=%d，期望 2: %+v", len(chunks), chunks)
	}
	if !strings.Contains(chunks[0].Content, "收入表") || !strings.Contains(chunks[0].Content, "| 一月 | 100 |") {
		t.Fatalf("表格结构化字段未进入检索文本: %+v", chunks[0])
	}
	if len(chunks[0].BlockIDs) != 2 || chunks[0].TableRefs[0] != "table-1" {
		t.Fatalf("表格多来源关系丢失: %+v", chunks[0])
	}
	if !strings.Contains(chunks[1].Content, "系统架构") || !strings.Contains(chunks[1].Content, "服务关系图") {
		t.Fatalf("图片 typed 文本未进入 Chunk: %+v", chunks[1])
	}
}

func TestCanonicalFlatStrategiesExposeDecision(t *testing.T) {
	doc := &canonical.CanonicalDocument{
		SchemaVersion: canonical.SchemaVersion, RendererVersion: canonical.DefaultRendererVersion,
		Markdown: "# 标题\n\n正文", Profile: canonical.DocumentProfile{SourceFormat: "txt"},
		Nodes: []canonical.CanonicalNode{
			{ID: "h", Kind: canonical.NodeKindHeading, Text: "标题", Markdown: "# 标题", BlockIDs: []string{"h"}},
			{ID: "p", Kind: canonical.NodeKindParagraph, Text: "正文", Markdown: "正文", BlockIDs: []string{"p"}},
		},
	}
	for _, strategy := range []string{StrategyParagraph, StrategyRecursive} {
		chunks, err := newChunker().ChunkCanonicalStrategy(context.Background(), doc, DefaultChunkOptions(), strategy)
		if err != nil {
			t.Fatalf("%s: %v", strategy, err)
		}
		if len(chunks) == 0 || chunks[0].Strategy != strategy {
			t.Fatalf("%s decision missing: %+v", strategy, chunks)
		}
		wantVersion := ParagraphVersion
		if strategy == StrategyRecursive {
			wantVersion = RecursiveVersion
		}
		if chunks[0].StrategyVersion != wantVersion {
			t.Fatalf("%s version = %q, want %q", strategy, chunks[0].StrategyVersion, wantVersion)
		}
	}
}

func TestRecursiveStrategyKeepsAtomicCodeSeparate(t *testing.T) {
	markdown := "前文\n\n```go\nfmt.Println(1)\n```\n\n后文"
	doc := &canonical.CanonicalDocument{
		SchemaVersion: canonical.SchemaVersion, RendererVersion: canonical.DefaultRendererVersion,
		Markdown: markdown, Profile: canonical.DocumentProfile{SourceFormat: "txt"},
		Nodes: []canonical.CanonicalNode{
			{ID: "p1", Kind: canonical.NodeKindParagraph, Text: "前文", Markdown: "前文", BlockIDs: []string{"p1"}},
			{ID: "c1", Kind: canonical.NodeKindCode, Text: "fmt.Println(1)", Markdown: "```go\nfmt.Println(1)\n```", BlockIDs: []string{"c1"}, Atomic: true},
			{ID: "p2", Kind: canonical.NodeKindParagraph, Text: "后文", Markdown: "后文", BlockIDs: []string{"p2"}},
		},
	}
	opts := DefaultChunkOptions()
	opts.MinTokens = 0
	chunks, err := newChunker().ChunkCanonicalStrategy(context.Background(), doc, opts, StrategyRecursive)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("recursive chunks = %d, want 3: %+v", len(chunks), chunks)
	}
	if chunks[1].Content != "```go\nfmt.Println(1)\n```" || chunks[1].StrategyVersion != RecursiveVersion {
		t.Fatalf("Atomic code 未独立保留: %+v", chunks[1])
	}
}

func TestCanonicalLongNodeUsesClippedAndOverlapSpans(t *testing.T) {
	markdown := "甲乙。丙丁。戊己。"
	doc := &canonical.CanonicalDocument{
		SchemaVersion: canonical.SchemaVersion, RendererVersion: canonical.DefaultRendererVersion,
		Markdown: markdown, Profile: canonical.DocumentProfile{SourceFormat: "txt"},
		Nodes: []canonical.CanonicalNode{{
			ID: "b1", Kind: canonical.NodeKindParagraph, StartByte: 0, EndByte: len(markdown),
			Text: markdown, Markdown: markdown, BlockIDs: []string{"b1"},
			Sources: []canonical.SourceRef{{BlockID: "b1", Page: 1}},
		}},
		SourceMap: []canonical.SourceSpan{{
			StartByte: 0, EndByte: len(markdown), Sources: []canonical.SourceRef{{BlockID: "b1", Page: 1}},
		}},
	}
	opts := DefaultChunkOptions()
	opts.MaxTokens = 8
	opts.MinTokens = 0
	opts.OverlapTokens = 2
	chunks, err := newChunker().ChunkCanonicalStrategy(context.Background(), doc, opts, StrategyStructured)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want >= 2", len(chunks))
	}
	overlapSeen := false
	for i, chunk := range chunks {
		if len(chunk.SourceSpans) == 0 {
			t.Fatalf("chunk %d 缺少来源: %+v", i, chunk)
		}
		for _, span := range chunk.SourceSpans {
			if span.EndByte-span.StartByte >= len(markdown) {
				t.Fatalf("chunk %d 复制了整个节点来源: %+v", i, span)
			}
			if span.Reason == "overlap" {
				overlapSeen = true
			}
		}
	}
	if !overlapSeen {
		t.Fatalf("未标记 overlap: %+v", chunks)
	}
}
