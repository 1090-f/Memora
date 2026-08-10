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

// TextParser 将 TXT / Markdown 解析为统一 ParsedDocument：
//   - Markdown 标题（# 1-6 级）转换为 heading Block 并维护 heading_path；
//   - 围栏代码块、列表项、段落转换为对应 Block 类型；
//   - 输出与 PDF/DOCX 完全一致的 ParsedDocument 协议，保证统一分块入口。
type TextParser struct {
	// maxBytes 是输入大小上限；超过视为输入错误。
	maxBytes int64
}

// NewTextParser 构造文本解析器。
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

	format := "txt"
	if ext := strings.ToLower(path.Ext(input.FileName)); ext == ".md" || ext == ".markdown" {
		format = "markdown"
	}
	title := titleOf(input.FileName, string(content))

	blocks := parseTextBlocks(string(content), format, input.Options.IncludeBBoxes)

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
			Format:   format,
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

// parseTextBlocks 将文本按行解析为 Block 序列（保持阅读顺序）。
func parseTextBlocks(content, format string, includeBBoxes bool) []Block {
	lines := strings.Split(content, "\n")
	var blocks []Block
	headingPath := make([]string, 0, 4)
	buffer := strings.Builder{}
	flushParagraph := func() {
		if strings.TrimSpace(buffer.String()) == "" {
			buffer.Reset()
			return
		}
		blocks = append(blocks, Block{
			ID:          fmt.Sprintf("block-%06d", len(blocks)+1),
			Type:        BlockTypeParagraph,
			Text:        strings.TrimSpace(buffer.String()),
			Markdown:    strings.TrimSpace(buffer.String()),
			HeadingPath: append([]string(nil), headingPath...),
			Source:      SourceLocation{Page: 0, BBox: nil},
		})
		buffer.Reset()
	}

	inCode := false
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)

		if format == "markdown" && strings.HasPrefix(strings.TrimLeft(line, " "), "```") {
			if inCode {
				buffer.WriteString(line + "\n")
				blocks = append(blocks, Block{
					ID:          fmt.Sprintf("block-%06d", len(blocks)+1),
					Type:        BlockTypeCode,
					Text:        strings.TrimSpace(buffer.String()),
					Markdown:    "```\n" + strings.TrimSpace(buffer.String()) + "\n```",
					HeadingPath: append([]string(nil), headingPath...),
					Source:      SourceLocation{Page: 0, BBox: nil},
				})
				buffer.Reset()
				inCode = false
			} else {
				flushParagraph()
				buffer.Reset()
				inCode = true
			}
			continue
		}

		if trimmed == "" {
			if format == "markdown" {
				flushParagraph()
			}
			continue
		}

		if format == "markdown" {
			if level, text, ok := markdownHeading(trimmed); ok {
				flushParagraph()
				// 标题路径：level 层替换/截断后追加当前标题文本，保留父级路径。
				for len(headingPath) >= level {
					headingPath = headingPath[:len(headingPath)-1]
				}
				headingPath = append(headingPath, text)
				blocks = append(blocks, Block{
					ID:          fmt.Sprintf("block-%06d", len(blocks)+1),
					Type:        BlockTypeHeading,
					Text:        text,
					Markdown:    text,
					HeadingPath: append([]string(nil), headingPath...),
					Source:      SourceLocation{Page: 0, BBox: nil},
				})
				continue
			}
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				flushParagraph()
				blocks = append(blocks, Block{
					ID:          fmt.Sprintf("block-%06d", len(blocks)+1),
					Type:        BlockTypeListItem,
					Text:        trimmed[2:],
					Markdown:    trimmed,
					HeadingPath: append([]string(nil), headingPath...),
					Source:      SourceLocation{Page: 0, BBox: nil},
				})
				continue
			}
		}

		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(line)
	}
	if inCode {
		blocks = append(blocks, Block{
			ID:          fmt.Sprintf("block-%06d", len(blocks)+1),
			Type:        BlockTypeCode,
			Text:        strings.TrimSpace(buffer.String()),
			Markdown:    "```\n" + strings.TrimSpace(buffer.String()) + "\n```",
			HeadingPath: append([]string(nil), headingPath...),
			Source:      SourceLocation{Page: 0, BBox: nil},
		})
		buffer.Reset()
	}
	flushParagraph()
	return blocks
}

// markdownHeading 判断是否 Markdown 标题行；返回级别与文本。
func markdownHeading(line string) (int, string, bool) {
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}
	level := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		level++
		if level > 6 {
			return 0, "", false
		}
	}
	text := strings.TrimSpace(line[level:])
	if text == "" {
		return 0, "", false
	}
	return level, text, true
}

// titleOf 生成标题：优先首个 h1 标题文本，其次文件名。
func titleOf(fileName, content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && len(trimmed) > 2 {
			return strings.TrimSpace(trimmed[2:])
		}
	}
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
