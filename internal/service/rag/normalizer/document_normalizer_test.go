package normalizer

import (
	"context"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

func testDoc() *parser.ParsedDocument {
	return &parser.ParsedDocument{
		SchemaVersion: parser.SchemaVersion,
		Document: parser.DocumentInfo{
			Title:    "  示例文档  ",
			Markdown: "# 示例\r\n\r\n\r\n正文\u200b",
			Metadata: map[string]any{},
		},
		Blocks: []parser.Block{
			{
				ID: "block-1", Type: "Page_Header", Text: "公司页眉",
				HeadingPath: []string{"  ", "第一章"},
			},
			{
				ID: "block-2", Type: "Page_Header", Text: "公司页眉",
			},
			{
				ID: "block-3", Type: "paragraph", Text: " 正文\u200b内容 ",
				TableRef: "table-000001", AssetRefs: []string{"asset-000001", "asset-999"},
			},
			{
				ID: "block-4", Type: "unknown", Text: "  ",
			},
			{
				ID: "block-5", Type: "picture", Text: "",
				AssetRefs: []string{"asset-000001"},
			},
		},
		Tables: []parser.Table{
			{
				ID:       "table-000001",
				Caption:  " 表 1 ",
				Headers:  [][]string{{" 地区 ", "销\r\n售额"}},
				Rows:     [][]string{{" 华东\t", "100"}},
				Cells:    []parser.TableCell{{Row: 0, Column: 0, Text: " 地区 "}},
				Markdown: "| 地区 |\r\n",
			},
		},
		Assets: []parser.Asset{
			{ID: "asset-000001", Caption: " 图 1 ", MIMEType: "image/png", Omitted: true},
		},
	}
}

func TestNormalizeBlocksAndReferences(t *testing.T) {
	doc := testDoc()
	if err := NewDocumentNormalizer().Normalize(context.Background(), doc); err != nil {
		t.Fatalf("规范化失败: %v", err)
	}
	// 重复页眉去重。
	headers := 0
	for _, b := range doc.Blocks {
		if b.Type == parser.BlockTypePageHeader {
			headers++
		}
	}
	if headers != 1 {
		t.Errorf("页眉去重后数量 = %d，期望 1", headers)
	}
	// 空未知 Block 移除；图片 Block 保留。
	pictures := 0
	for _, b := range doc.Blocks {
		if b.Type == parser.BlockTypePicture {
			pictures++
		}
		if b.Type == parser.BlockTypeUnknown {
			t.Error("空 unknown Block 应被移除")
		}
	}
	if pictures != 1 {
		t.Errorf("图片 Block 数量 = %d，期望 1", pictures)
	}
	// 悬空引用清理。
	for _, b := range doc.Blocks {
		if b.ID == "block-3" {
			if b.TableRef == "" {
				t.Error("table_ref 应保留（表存在）")
			}
			for _, ref := range b.AssetRefs {
				if ref == "asset-999" {
					t.Error("悬空 asset_ref 应被移除")
				}
			}
		}
	}
	// heading path trim + 去空。
	for _, b := range doc.Blocks {
		if b.ID == "block-1" {
			if len(b.HeadingPath) != 1 || b.HeadingPath[0] != "第一章" {
				t.Errorf("heading_path = %v", b.HeadingPath)
			}
		}
	}
	// 零宽字符移除。
	for _, b := range doc.Blocks {
		if b.ID == "block-3" && strings.Contains(b.Text, "\u200b") {
			t.Error("正文仍含零宽字符")
		}
	}
}

func TestNormalizeTableCellText(t *testing.T) {
	doc := testDoc()
	if err := NewDocumentNormalizer().Normalize(context.Background(), doc); err != nil {
		t.Fatalf("规范化失败: %v", err)
	}
	table := doc.Tables[0]
	if table.Headers[0][1] != "销售额" {
		t.Errorf("表头单元格换行未规范化: %q", table.Headers[0][1])
	}
	if table.Rows[0][0] != "华东" {
		t.Errorf("行单元格未 trim: %q", table.Rows[0][0])
	}
	// 行列结构保持。
	if len(table.Rows) != 1 || len(table.Headers[0]) != 2 {
		t.Error("表格行列结构被破坏")
	}
}

func TestNormalizeKeepsWarningsAndCaption(t *testing.T) {
	doc := testDoc()
	if err := NewDocumentNormalizer().Normalize(context.Background(), doc); err != nil {
		t.Fatalf("规范化失败: %v", err)
	}
	found := false
	for _, w := range doc.Warnings {
		if strings.Contains(w, "asset-999") {
			found = true
		}
	}
	if !found {
		t.Error("应记录悬空引用的 warning")
	}
	if doc.Assets[0].Caption != "图 1" {
		t.Errorf("caption 未规范化: %q", doc.Assets[0].Caption)
	}
}
