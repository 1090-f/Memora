package chunking

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// TestAttachPicturesManyPicturesNoPanic 回归测试：大量图片与正文交替时，
// attachPictures 的合并移除操作会不断缩短单元列表，旧实现缓存的图片下标
// 会越界 panic（PPTX 多图场景）。合并必须逐位顺序处理。
func TestAttachPicturesManyPicturesNoPanic(t *testing.T) {
	var blocks []parser.Block
	var assets []parser.Asset
	for i := 0; i < 40; i++ {
		blocks = append(blocks,
			parser.Block{ID: fmt.Sprintf("pic-%d", i), Type: parser.BlockTypePicture, Text: "", AssetRefs: []string{fmt.Sprintf("asset-%d", i)}},
			parser.Block{ID: fmt.Sprintf("para-%d", i), Type: parser.BlockTypeParagraph, Text: fmt.Sprintf("第 %d 段相关正文内容。", i)},
		)
		assets = append(assets, parser.Asset{ID: fmt.Sprintf("asset-%d", i), Kind: "picture", Caption: fmt.Sprintf("图片 %d 说明", i), Metadata: map[string]any{}})
	}
	doc := &parser.ParsedDocument{
		Source: parser.SourceInfo{Format: "pptx"},
		Blocks: blocks,
		Assets: assets,
	}

	chunker := NewStructureAwareChunker(NewHeuristicTokenizer(), "structure-v1")
	chunks, err := chunker.Chunk(context.Background(), doc, ChunkOptions{MaxTokens: 2000, MinTokens: 0, OverlapTokens: 0})
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("未产生 Chunk")
	}
	// 每张图片都应并入相邻正文（文本量小、未超限），图片说明文字应出现在 Chunk 内容中。
	allContent := strings.Join(func() []string {
		contents := make([]string, len(chunks))
		for i, chunk := range chunks {
			contents[i] = chunk.Content
		}
		return contents
	}(), "\n")
	for i := 0; i < 40; i++ {
		caption := fmt.Sprintf("图片 %d 说明", i)
		if !strings.Contains(allContent, caption) {
			t.Errorf("图片 %d 说明未出现在任何 Chunk 中", i)
		}
	}
}

// TestAttachPicturesOverLimitKeepsPictureIndependent 验证关联后超出 MaxTokens 时
// 图片保持独立，不因移除操作影响其它单元。
func TestAttachPicturesOverLimitKeepsPictureIndependent(t *testing.T) {
	longText := strings.Repeat("这是一段非常长的正文内容。", 200)
	blocks := []parser.Block{
		{ID: "pic-1", Type: parser.BlockTypePicture, AssetRefs: []string{"asset-1"}},
		{ID: "para-1", Type: parser.BlockTypeParagraph, Text: longText},
		{ID: "para-2", Type: parser.BlockTypeParagraph, Text: "短正文。"},
	}
	assets := []parser.Asset{{ID: "asset-1", Kind: "picture", Caption: "独立图片说明"}}
	doc := &parser.ParsedDocument{Blocks: blocks, Assets: assets}

	chunker := NewStructureAwareChunker(NewHeuristicTokenizer(), "structure-v1")
	chunks, err := chunker.Chunk(context.Background(), doc, ChunkOptions{MaxTokens: 1000, MinTokens: 0, OverlapTokens: 0})
	if err != nil {
		t.Fatalf("Chunk 失败: %v", err)
	}
	found := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.Content, "独立图片说明") {
			found = true
		}
	}
	if !found {
		t.Error("超限图片应独立成 Chunk 并保留图片说明")
	}
}
