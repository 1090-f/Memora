package chunking

import (
	"context"
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
	}
}
