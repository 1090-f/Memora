package parser

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// memoryAssetLoader 是测试用 AssetLoader：网络图片直接返回字节，附件按路径映射
// （精确匹配优先，其次按文件名 basename 匹配，与 MarkdownAssetLoader 行为一致）。
type memoryAssetLoader struct {
	attachments map[string][]byte
}

func (l *memoryAssetLoader) Open(_ context.Context, ref string) (io.ReadCloser, string, error) {
	if data, ok := l.attachments[ref]; ok {
		return io.NopCloser(bytes.NewReader(data)), "image/png", nil
	}
	for path, data := range l.attachments {
		if strings.EqualFold(fileBaseName(path), fileBaseName(ref)) {
			return io.NopCloser(bytes.NewReader(data)), "image/png", nil
		}
	}
	return nil, "", io.ErrUnexpectedEOF
}

// fileBaseName 返回路径最后一段，兼容 / 与 \ 分隔符。
func fileBaseName(p string) string {
	normalized := strings.ReplaceAll(p, "\\", "/")
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		return normalized[idx+1:]
	}
	return normalized
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

func TestMarkdownImageWindowsPathMatchedByBaseName(t *testing.T) {
	// 绝对路径引用 + zip 附件按文件名（basename）匹配成功。
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	loader := &memoryAssetLoader{attachments: map[string][]byte{"pic.jpg": png}}
	doc := parseMarkdown(t, "![图](C:\\Users\\foo\\Desktop\\pic.jpg)", loader)

	if len(doc.Assets) != 1 {
		t.Fatalf("basename 匹配应生成 Asset，实际 %d", len(doc.Assets))
	}
	if doc.Assets[0].SourceRef != `C:\Users\foo\Desktop\pic.jpg` {
		t.Errorf("Asset 不正确: %+v", doc.Assets[0])
	}
	if len(doc.Warnings) != 0 {
		t.Errorf("不应有 warning: %v", doc.Warnings)
	}
}

func TestMarkdownImageMissingAttachmentUnresolved(t *testing.T) {
	doc := parseMarkdown(t, "![图](missing.png)", &memoryAssetLoader{attachments: map[string][]byte{}})

	if len(doc.Assets) != 0 || len(doc.Warnings) != 1 {
		t.Errorf("缺失附件应 warning：assets=%d warnings=%v", len(doc.Assets), doc.Warnings)
	}
}

func TestMarkdownImageInlineKeptAsText(t *testing.T) {
	// 行内图片：提取资产，文本移除语法但保留周围文字。
	png := []byte{0x89, 'P', 'N', 'G'}
	loader := &memoryAssetLoader{attachments: map[string][]byte{"a.png": png}}
	doc := parseMarkdown(t, "正文里有一张 ![图](a.png) 行内引用", loader)

	if len(doc.Assets) != 1 {
		t.Fatalf("行内引用应提取资产，实际 %d", len(doc.Assets))
	}
	if len(doc.Blocks) != 1 || strings.Contains(doc.Blocks[0].Text, "![图](a.png)") {
		t.Errorf("行内图片语法应从文本移除: %q", doc.Blocks[0].Text)
	}
	if !strings.Contains(doc.Blocks[0].Text, "正文里有一张") || !strings.Contains(doc.Blocks[0].Text, "行内引用") {
		t.Errorf("周围文字应保留: %q", doc.Blocks[0].Text)
	}
}

func TestMarkdownImageInlineMissingUnresolved(t *testing.T) {
	// 行内图片找不到附件：warning + 语法保留（预览排查用）。
	doc := parseMarkdown(t, "正文 ![图](missing.png) 结尾", &memoryAssetLoader{attachments: map[string][]byte{}})

	if len(doc.Assets) != 0 {
		t.Fatalf("缺失附件不应提取资产，实际 %d", len(doc.Assets))
	}
	if len(doc.Warnings) != 1 {
		t.Errorf("应记录 warning: %v", doc.Warnings)
	}
	if len(doc.Blocks) != 1 || !strings.Contains(doc.Blocks[0].Text, "![图](missing.png)") {
		t.Errorf("unresolved 行内引用应保留语法: %q", doc.Blocks[0].Text)
	}
}

func TestMarkdownImageReferenceStyle(t *testing.T) {
	// 引用式图片：![alt][ref] + [ref]: path。
	png := []byte{0x89, 'P', 'N', 'G'}
	loader := &memoryAssetLoader{attachments: map[string][]byte{"img/logo.png": png}}
	content := strings.Join([]string{
		"![Logo][logo]",
		"",
		"[logo]: img/logo.png",
		"",
	}, "\n")
	doc := parseMarkdown(t, content, loader)

	if len(doc.Assets) != 1 {
		t.Fatalf("引用式图片应提取资产，实际 %d", len(doc.Assets))
	}
	if doc.Assets[0].SourceRef != "img/logo.png" {
		t.Errorf("SourceRef 应为定义路径: %+v", doc.Assets[0])
	}
	if len(doc.Blocks) != 1 || doc.Blocks[0].Type != BlockTypePicture {
		t.Errorf("引用式整行应生成 picture Block: %+v", doc.Blocks)
	}
}

func TestMarkdownImageWithTitle(t *testing.T) {
	// 带 title 的整行图片：![alt](path "title")。
	png := []byte{0x89, 'P', 'N', 'G'}
	loader := &memoryAssetLoader{attachments: map[string][]byte{"b.png": png}}
	doc := parseMarkdown(t, `![图](b.png "图片标题")`, loader)

	if len(doc.Assets) != 1 || doc.Assets[0].Caption != "图" {
		t.Errorf("带 title 图片应提取资产: %+v", doc.Assets)
	}
	if len(doc.Blocks) != 1 || doc.Blocks[0].Type != BlockTypePicture {
		t.Errorf("应生成 picture Block: %+v", doc.Blocks)
	}
}

func TestScanMarkdownImageRefs(t *testing.T) {
	content := strings.Join([]string{
		"# 标题",
		"![网络图](https://example.com/a.png)",
		"正文里的 ![行内](img/b.png) 图片",
		"![引用图][ref1]",
		"",
		"[ref1]: ./images/c.png \"title\"",
		"![data](data:image/png;base64,AAAA)",
		"![缺定义][missing]",
	}, "\n")
	refs := ScanMarkdownImageRefs(content)

	want := []ImageRef{
		{Alt: "网络图", Ref: "https://example.com/a.png"},
		{Alt: "行内", Ref: "img/b.png"},
		{Alt: "引用图", Ref: "./images/c.png"},
		{Alt: "data", Ref: "data:image/png;base64,AAAA"},
		{Alt: "缺定义", Ref: "[missing]"},
	}
	if len(refs) != len(want) {
		t.Fatalf("引用数量 = %d, want %d: %+v", len(refs), len(want), refs)
	}
	for i, item := range want {
		if refs[i] != item {
			t.Errorf("引用[%d] = %+v, want %+v", i, refs[i], item)
		}
	}
}

func TestTxtImageSyntaxKeptAsText(t *testing.T) {
	doc := parseMarkdown(t, "![图](a.png)", nil)
	if len(doc.Assets) != 0 || len(doc.Warnings) != 1 {
		t.Fatalf("md 无 loader 时应 warning：assets=%d warnings=%v", len(doc.Assets), doc.Warnings)
	}
}
