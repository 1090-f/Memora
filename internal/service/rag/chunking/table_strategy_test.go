package chunking

import (
	"context"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

func tableDoc(rows [][]string, maxTokens int) (*parser.ParsedDocument, ChunkOptions) {
	table := parser.Table{
		ID: "table-1", Caption: "表 1 销售数据",
		Headers: [][]string{{"地区", "销售额"}}, Rows: rows,
		RowCount: len(rows), ColumnCount: 2,
	}
	doc := &parser.ParsedDocument{
		Tables: []parser.Table{table},
		Blocks: []parser.Block{
			headingBlock("h1", "第一章"),
			{
				ID: "tb1", Type: parser.BlockTypeTable, Text: "| 地区 | 销售额 |",
				TableRef: "table-1", HeadingPath: []string{"第一章"},
				Source: parser.SourceLocation{Page: 1, BBox: []float64{0, 0, 100, 100}},
			},
		},
	}
	opts := DefaultChunkOptions()
	opts.MaxTokens = maxTokens
	return doc, opts
}

func TestTableSmallTableWholeChunk(t *testing.T) {
	doc, opts := tableDoc([][]string{{"华东", "100"}, {"华南", "200"}}, 1000)
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("小表应整体成 Chunk，实际 %d", len(chunks))
	}
	content := chunks[0].Content
	for _, want := range []string{"表 1 销售数据", "| 地区 | 销售额 |", "华东", "华南"} {
		if !strings.Contains(content, want) {
			t.Errorf("表格 Chunk 缺少 %q: %q", want, content)
		}
	}
	if len(chunks[0].TableRefs) != 1 || chunks[0].TableRefs[0] != "table-1" {
		t.Errorf("TableRefs = %v", chunks[0].TableRefs)
	}
	if chunks[0].SourceLocation.Page != 1 {
		t.Errorf("页码丢失: %v", chunks[0].SourceLocation)
	}
	if len(chunks[0].BlockIDs) != 1 || chunks[0].BlockIDs[0] != "tb1" {
		t.Errorf("BlockIDs = %v", chunks[0].BlockIDs)
	}
}

func TestTableBigTableSplitByRows(t *testing.T) {
	rows := make([][]string, 200)
	for i := range rows {
		rows[i] = []string{"地区", "数值数据"}
	}
	doc, opts := tableDoc(rows, 40)
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("大表应按行拆分，实际 %d 个", len(chunks))
	}
	for _, chunk := range chunks {
		if chunk.TokenCount > 40 {
			t.Errorf("子 Chunk %d tokens 超过上限", chunk.TokenCount)
		}
		// 每个子 Chunk 重复 caption 与表头。
		if !strings.Contains(chunk.Content, "表 1 销售数据") || !strings.Contains(chunk.Content, "| 地区 | 销售额 |") {
			t.Errorf("子 Chunk 缺少重复表头/caption: %q", chunk.Content)
		}
		// 行范围标注。
		if !strings.Contains(chunk.Content, "第") || !strings.Contains(chunk.Content, "行") {
			t.Errorf("子 Chunk 缺少行范围: %q", chunk.Content)
		}
		if len(chunk.TableRefs) != 1 {
			t.Errorf("子 Chunk TableRefs = %v", chunk.TableRefs)
		}
	}
}

func TestTableSingleRowOverLimitSplitsSafely(t *testing.T) {
	// 单行超长单元格。
	row := []string{strings.Repeat("很长很长的单元格内容。", 200), "b"}
	doc, opts := tableDoc([][]string{row}, 100)
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) < 1 {
		t.Fatal("应有 Chunk")
	}
	// 超限行拆分后前缀也计入预算，每个 Chunk 严格不超限。
	for _, chunk := range chunks {
		if chunk.TokenCount > 100 {
			t.Errorf("Chunk %d tokens 超限: %q", chunk.TokenCount, chunk.Content)
		}
		if !strings.Contains(chunk.Content, "第 1-1 行") {
			t.Errorf("超长单行子块缺少行范围: %q", chunk.Content)
		}
	}
}

func TestTableRepeatHeaderCanBeDisabled(t *testing.T) {
	rows := make([][]string, 80)
	for i := range rows {
		rows[i] = []string{"地区", "数值数据"}
	}
	doc, opts := tableDoc(rows, 40)
	opts.RepeatTableHead = false
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("大表应拆成多个 Chunk，实际 %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "| 地区 | 销售额 |") {
		t.Error("首块必须保留原始表头")
	}
	for i, chunk := range chunks[1:] {
		if strings.Contains(chunk.Content, "| 地区 | 销售额 |") {
			t.Errorf("RepeatTableHead=false 时后续块 %d 不应重复表头: %q", i+1, chunk.Content)
		}
		if !strings.Contains(chunk.Content, "表 1 销售数据") {
			t.Errorf("后续块仍应保留 caption: %q", chunk.Content)
		}
	}
}
