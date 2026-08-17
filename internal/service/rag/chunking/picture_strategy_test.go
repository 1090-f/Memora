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

func pictureDoc(caption string, withDescription bool, withParagraph bool, maxTokens int) (*parser.ParsedDocument, ChunkOptions) {
	metadata := map[string]any{}
	if withDescription {
		metadata["description"] = "系统架构图：包含 API 网关与数据库"
	}
	doc := &parser.ParsedDocument{
		Assets: []parser.Asset{{
			ID: "asset-1", Kind: "picture", Caption: caption, Omitted: false,
			Page: 2, Metadata: metadata,
		}},
		Blocks: []parser.Block{
			headingBlock("h1", "第一章"),
			{
				ID: "pic1", Type: parser.BlockTypePicture, Text: "",
				HeadingPath: []string{"第一章"}, AssetRefs: []string{"asset-1"},
				Source: parser.SourceLocation{Page: 2},
			},
		},
	}
	if withParagraph {
		doc.Blocks = append(doc.Blocks, textBlock("p1", "这是图片附近的正文段落。", "第一章", 2))
	}
	opts := DefaultChunkOptions()
	opts.MaxTokens = maxTokens
	return doc, opts
}

func TestPictureWithCaptionGeneratesChunk(t *testing.T) {
	doc, opts := pictureDoc("图 1 系统架构", false, false, 1000)
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("有 caption 应生成 Chunk，实际 %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "图 1 系统架构") {
		t.Errorf("Chunk 应含 caption: %q", chunks[0].Content)
	}
	if chunks[0].AssetRefs[0] != "asset-1" {
		t.Errorf("AssetRefs = %v", chunks[0].AssetRefs)
	}
	if len(chunks[0].BlockIDs) != 1 || chunks[0].BlockIDs[0] != "pic1" {
		t.Errorf("BlockIDs = %v", chunks[0].BlockIDs)
	}
}

func TestPictureWithoutTextGeneratesNoChunk(t *testing.T) {
	doc, opts := pictureDoc("", false, false, 1000)
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("无文字图片不应生成空 Chunk，实际 %d", len(chunks))
	}
}

func TestPictureWithDescriptionUsesDescription(t *testing.T) {
	doc, opts := pictureDoc("图 1", true, false, 1000)
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("期望 1 个 Chunk，实际 %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "系统架构图") {
		t.Errorf("Chunk 应含 description: %q", chunks[0].Content)
	}
}

func TestPictureAssociatesNearbyParagraph(t *testing.T) {
	doc, opts := pictureDoc("图 1", false, true, 1000)
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("图片应与正文合并，实际 %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "这是图片附近的正文段落。") {
		t.Errorf("Chunk 应含关联正文: %q", chunks[0].Content)
	}
	// 合并后 Block 引用完整。
	foundPic := false
	for _, id := range chunks[0].BlockIDs {
		if id == "pic1" {
			foundPic = true
		}
	}
	if !foundPic {
		t.Errorf("合并后应保留图片 BlockID: %v", chunks[0].BlockIDs)
	}
}

func TestPictureStandsAloneWhenOverLimit(t *testing.T) {
	// 图片文本超限时独立成 Chunk。
	longParagraph := textBlock("p1", strings.Repeat("很长很长的正文段落内容。", 500), "第一章", 2)
	doc, opts := pictureDoc("图 1", false, false, 10)
	doc.Blocks = append(doc.Blocks, longParagraph)
	chunks, err := newChunker().Chunk(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("图片与超长正文应分别成块，实际 %d", len(chunks))
	}
}

// TestPictureTextDedupesOCRAndCaption 验证 caption 与 OCR 文字相同时只保留一份，
// 不同时则依次保留，避免重复文本进入 Chunk。
func TestPictureTextDedupesOCRAndCaption(t *testing.T) {
	tests := []struct {
		name     string
		asset    parser.Asset
		wantText string
	}{
		{
			name: "caption 与 OCR 相同只保留一份",
			asset: parser.Asset{
				Caption:  "发票 金额",
				Metadata: map[string]any{"ocr_text": "发票 金额"},
			},
			wantText: "发票 金额",
		},
		{
			name: "caption 与 OCR 不同则都保留",
			asset: parser.Asset{
				Caption:  "图 1 系统架构",
				Metadata: map[string]any{"ocr_text": "API 网关"},
			},
			wantText: "API 网关\n图 1 系统架构",
		},
		{
			name: "caption 为空仅保留 OCR",
			asset: parser.Asset{
				Metadata: map[string]any{"ocr_text": "发票 金额"},
			},
			wantText: "发票 金额",
		},
		{
			name:     "仅 caption 时保持原行为",
			asset:    parser.Asset{Caption: "图 1 系统架构"},
			wantText: "图 1 系统架构",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pictureText(tt.asset); got != tt.wantText {
				t.Errorf("pictureText() = %q, 期望 %q", got, tt.wantText)
			}
		})
	}
}
