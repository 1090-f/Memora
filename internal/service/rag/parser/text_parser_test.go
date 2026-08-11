package parser

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// memoryAssetLoader 是测试用 AssetLoader：网络图片直接返回字节，附件按路径映射。
type memoryAssetLoader struct {
	attachments map[string][]byte
}

func (l *memoryAssetLoader) Open(_ context.Context, ref string) (io.ReadCloser, string, error) {
	if data, ok := l.attachments[ref]; ok {
		return io.NopCloser(bytes.NewReader(data)), "image/png", nil
	}
	return nil, "", io.ErrUnexpectedEOF
}

func parseMarkdown(t *testing.T, content string, loader AssetLoader) *ParsedDocument {
	t.Helper()
	parser := NewTextParser(64 * 1024 * 1024)
	doc, err := parser.Parse(context.Background(), ParseInput{
		FileName:    "test.md",
		Content:     strings.NewReader(content),
		Options:     DefaultParseOptions(),
		AssetLoader: loader,
	})
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	return doc
}

func TestMarkdownImageExtractedFromAttachment(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x01, 0x02, 0x03}
	loader := &memoryAssetLoader{attachments: map[string][]byte{"img/logo.png": png}}
	doc := parseMarkdown(t, "# 标题\n\n![Logo](img/logo.png)\n\n正文", loader)

	if len(doc.Assets) != 1 {
		t.Fatalf("期望 1 个 Asset，实际 %d", len(doc.Assets))
	}
	asset := doc.Assets[0]
	if asset.Kind != "picture" || asset.MIMEType != "image/png" {
		t.Errorf("Asset 元信息不正确: %+v", asset)
	}
	if asset.Caption != "Logo" || asset.SourceRef != "img/logo.png" {
		t.Errorf("Caption/SourceRef 不正确: caption=%q source_ref=%q", asset.Caption, asset.SourceRef)
	}
	if asset.DataBase64 == "" || asset.SHA256 == "" {
		t.Error("Asset 缺少 base64 或哈希")
	}
	if len(doc.Blocks) != 3 || doc.Blocks[1].Type != BlockTypePicture || doc.Blocks[1].AssetRefs[0] != asset.ID {
		t.Errorf("picture Block 生成不正确: %+v", doc.Blocks)
	}
	if len(doc.Warnings) != 0 {
		t.Errorf("不应有 warning: %v", doc.Warnings)
	}
}

func TestMarkdownImageDataURIExtracted(t *testing.T) {
	doc := parseMarkdown(t, "![内嵌](data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2Q==)", nil)

	if len(doc.Assets) != 1 {
		t.Fatalf("期望 1 个 Asset，实际 %d", len(doc.Assets))
	}
	if doc.Assets[0].MIMEType != "image/jpeg" || doc.Assets[0].Caption != "内嵌" {
		t.Errorf("data URI Asset 不正确: %+v", doc.Assets[0])
	}
}

func TestMarkdownImageWindowsPathUnresolved(t *testing.T) {
	doc := parseMarkdown(t, "![图](C:\\Users\\foo\\Desktop\\pic.jpg)", nil)

	if len(doc.Assets) != 0 {
		t.Fatalf("本机路径不应生成 Asset，实际 %d", len(doc.Assets))
	}
	if len(doc.Warnings) != 1 || !strings.Contains(doc.Warnings[0], "unresolved") {
		t.Errorf("应记录 unresolved warning: %v", doc.Warnings)
	}
	if len(doc.Blocks) != 0 {
		t.Errorf("unresolved 图片不应生成 Block: %+v", doc.Blocks)
	}
}

func TestMarkdownImageMissingAttachmentUnresolved(t *testing.T) {
	doc := parseMarkdown(t, "![图](missing.png)", &memoryAssetLoader{attachments: map[string][]byte{}})

	if len(doc.Assets) != 0 || len(doc.Warnings) != 1 {
		t.Errorf("缺失附件应 warning：assets=%d warnings=%v", len(doc.Assets), doc.Warnings)
	}
}

func TestMarkdownImageInlineKeptAsText(t *testing.T) {
	doc := parseMarkdown(t, "正文里有一张 ![图](a.png) 行内引用", nil)

	if len(doc.Assets) != 0 || len(doc.Blocks) != 1 {
		t.Fatalf("行内引用不应提取：assets=%d blocks=%d", len(doc.Assets), len(doc.Blocks))
	}
	if !strings.Contains(doc.Blocks[0].Text, "![图](a.png)") {
		t.Errorf("行内引用应保留为正文: %q", doc.Blocks[0].Text)
	}
}

func TestTxtImageSyntaxKeptAsText(t *testing.T) {
	doc := parseMarkdown(t, "![图](a.png)", nil)
	if len(doc.Assets) != 0 || len(doc.Warnings) != 1 {
		t.Fatalf("md 无 loader 时应 warning：assets=%d warnings=%v", len(doc.Assets), doc.Warnings)
	}
}
