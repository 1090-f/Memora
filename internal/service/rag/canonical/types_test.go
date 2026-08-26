package canonical

import "testing"

func TestSelectChunkSourceSpansClipsToActualText(t *testing.T) {
	markdown := "甲乙。丙丁。戊己。"
	doc := &CanonicalDocument{
		Markdown: markdown,
		Nodes: []CanonicalNode{{
			ID: "b1", Kind: NodeKindParagraph, StartByte: 0, EndByte: len(markdown),
			Text: markdown, Markdown: markdown, BlockIDs: []string{"b1"},
		}},
		SourceMap: []SourceSpan{{
			StartByte: 0, EndByte: len(markdown), Sources: []SourceRef{{BlockID: "b1", Page: 1}},
		}},
	}

	spans := SelectChunkSourceSpans(doc, "丙丁。", []string{"b1"}, nil, nil)
	if len(spans) != 1 {
		t.Fatalf("spans = %+v", spans)
	}
	wantStart := len("甲乙。")
	wantEnd := len("甲乙。丙丁。")
	if spans[0].StartByte != wantStart || spans[0].EndByte != wantEnd {
		t.Fatalf("span = [%d,%d), want [%d,%d)", spans[0].StartByte, spans[0].EndByte, wantStart, wantEnd)
	}
	if spans[0].EndByte-spans[0].StartByte == len(markdown) {
		t.Fatal("子块不应复制整个节点来源区间")
	}
}

func TestSelectChunkSourceSpansFallsBackForGeneratedTableText(t *testing.T) {
	doc := &CanonicalDocument{
		Markdown: "| 列 |\n| 值 |",
		Nodes: []CanonicalNode{{
			ID: "table-block", Kind: NodeKindTable, StartByte: 0, EndByte: len("| 列 |\n| 值 |"),
			BlockIDs: []string{"table-block"}, TableRef: "table-1",
		}},
		SourceMap: []SourceSpan{{
			StartByte: 0, EndByte: len("| 列 |\n| 值 |"),
			Sources: []SourceRef{{BlockID: "table-block", TableRef: "table-1", Page: 2}},
		}},
	}

	spans := SelectChunkSourceSpans(doc, "章节\n（第 1-1 行）\n生成的表格文本", []string{"table-block"}, []string{"table-1"}, nil)
	if len(spans) != 1 || spans[0].Sources[0].TableRef != "table-1" {
		t.Fatalf("表格对象级来源回退失败: %+v", spans)
	}
}
