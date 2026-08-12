package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
)

// TextParser 将纯文本（TXT）解析为统一 ParsedDocument：
//   - 按空行划分段落，段落转换为 paragraph Block；
//   - 不做 Markdown 语法解释（标题/列表/代码/图片引用由 MarkdownParser 负责）；
//   - 输出与 PDF/DOCX 完全一致的 ParsedDocument 协议，保证统一分块入口。
type TextParser struct {
	// maxBytes 是输入大小上限；超过视为输入错误。
	maxBytes int64
}

// NewTextParser 构造纯文本解析器。
func NewTextParser(maxBytes int64) *TextParser {
	if maxBytes <= 0 {
		maxBytes = 64 * 1024 * 1024
	}
	return &TextParser{maxBytes: maxBytes}
}

// Parse 实现 Parser。
func (t *TextParser) Parse(ctx context.Context, input ParseInput) (*ParsedDocument, error) {
	content, err := io.ReadAll(io.LimitReader(input.Content, t.maxBytes+1))
	if err != nil {
		return nil, ParseErrorf(ParseErrorInternal, "读取文本失败: %v", err)
	}
	if int64(len(content)) > t.maxBytes {
		return nil, ParseErrorf(ParseErrorInvalidInput, "文件超过大小限制 %d 字节", t.maxBytes)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return nil, ParseErrorf(ParseErrorInvalidInput, "空文件或全空白文件")
	}

	title := titleOfTxt(input.FileName)
	blocks := parseTextBlocks(string(content))

	digest := sha256.Sum256(content)
	markdown := normalizeNewlines(string(content))
	return &ParsedDocument{
		SchemaVersion: SchemaVersion,
		Parser: ParserInfo{
			Name:           ParserNameGoText,
			Version:        "1.0",
			AdapterVersion: AdapterVersion,
		},
		Source: SourceInfo{
			FileName: input.FileName,
			Format:   "txt",
			SHA256:   hex.EncodeToString(digest[:]),
			Size:     int64(len(content)),
		},
		Document: DocumentInfo{
			Title:     title,
			Markdown:  markdown,
			PageCount: 0,
			Metadata:  map[string]any{},
		},
		Blocks:   blocks,
		Tables:   nil,
		Assets:   nil,
		Warnings: nil,
	}, nil
}

// parseTextBlocks 将纯文本按空行划分为段落 Block（保持阅读顺序）。
func parseTextBlocks(content string) []Block {
	lines := strings.Split(content, "\n")
	var blocks []Block
	buffer := strings.Builder{}

	flushParagraph := func() {
		if strings.TrimSpace(buffer.String()) == "" {
			buffer.Reset()
			return
		}
		blocks = append(blocks, Block{
			ID:       fmt.Sprintf("block-%06d", len(blocks)+1),
			Type:     BlockTypeParagraph,
			Text:     strings.TrimSpace(buffer.String()),
			Markdown: strings.TrimSpace(buffer.String()),
			Source:   SourceLocation{Page: 0, BBox: nil},
		})
		buffer.Reset()
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		if strings.TrimSpace(line) == "" {
			flushParagraph()
			continue
		}
		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(line)
	}
	flushParagraph()
	return blocks
}

// titleOfTxt 生成纯文本标题：取文件名（去扩展名）。
func titleOfTxt(fileName string) string {
	name := path.Base(fileName)
	if ext := path.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	if name == "" {
		return "未命名文档"
	}
	return name
}

// normalizeNewlines 统一换行并移除零宽字符/BOM。
func normalizeNewlines(content string) string {
	replacer := strings.NewReplacer("\u200b", "", "\ufeff", "", "\r\n", "\n", "\r", "\n")
	return replacer.Replace(content)
}
