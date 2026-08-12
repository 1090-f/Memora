package parser

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
)

// MarkdownParser 将 Markdown 解析为统一 ParsedDocument：
//   - 标题（# 1-6 级）转换为 heading Block 并维护 heading_path；
//   - 围栏代码块、列表项、段落转换为对应 Block 类型；
//   - 图片引用（![](...)，http(s)/附件相对路径/data URI）提取为 picture
//     Block + Asset；无法解析的引用标记 warning 且不阻断正文；
//   - 输出与 PDF/DOCX 完全一致的 ParsedDocument 协议，保证统一分块入口。
type MarkdownParser struct {
	// maxBytes 是输入大小上限；超过视为输入错误。
	maxBytes int64
}

// NewMarkdownParser 构造 Markdown 解析器。
func NewMarkdownParser(maxBytes int64) *MarkdownParser {
	if maxBytes <= 0 {
		maxBytes = 64 * 1024 * 1024
	}
	return &MarkdownParser{maxBytes: maxBytes}
}

// Parse 实现 Parser。
func (m *MarkdownParser) Parse(ctx context.Context, input ParseInput) (*ParsedDocument, error) {
	content, err := io.ReadAll(io.LimitReader(input.Content, m.maxBytes+1))
	if err != nil {
		return nil, ParseErrorf(ParseErrorInternal, "读取文本失败: %v", err)
	}
	if int64(len(content)) > m.maxBytes {
		return nil, ParseErrorf(ParseErrorInvalidInput, "文件超过大小限制 %d 字节", m.maxBytes)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return nil, ParseErrorf(ParseErrorInvalidInput, "空文件或全空白文件")
	}

	title := titleOf(input.FileName, string(content))

	var assets []Asset
	var warnings []string
	blocks := parseMarkdownBlocks(ctx, string(content), input.Options.IncludeBBoxes, input.AssetLoader, &assets, &warnings)

	digest := sha256.Sum256(content)
	markdown := normalizeNewlines(string(content))
	return &ParsedDocument{
		SchemaVersion: SchemaVersion,
		Parser: ParserInfo{
			Name:           ParserNameGoMarkdown,
			Version:        "1.0",
			AdapterVersion: AdapterVersion,
		},
		Source: SourceInfo{
			FileName: input.FileName,
			Format:   "markdown",
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
		Assets:   assets,
		Warnings: warnings,
	}, nil
}

// parseMarkdownBlocks 将 Markdown 按行解析为 Block 序列（保持阅读顺序）：
// 标题 / 列表 / 代码块 / 图片引用 / 段落。
func parseMarkdownBlocks(ctx context.Context, content string, includeBBoxes bool, loader AssetLoader, assets *[]Asset, warnings *[]string) []Block {
	lines := strings.Split(content, "\n")
	var blocks []Block
	headingPath := make([]string, 0, 4)
	buffer := strings.Builder{}
	definitions := collectReferenceDefinitions(content)

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

		if strings.HasPrefix(strings.TrimLeft(line, " "), "```") {
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
			flushParagraph()
			continue
		}

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
		// 独占整行图片（行内式/引用式，支持 title）→ picture Block。
		if alt, ref, ok := standaloneImageLine(trimmed, definitions); ok {
			flushParagraph()
			asset, warn := loadMarkdownImage(ctx, alt, ref, loader)
			if asset != nil {
				*assets = append(*assets, *asset)
				blocks = append(blocks, Block{
					ID:          fmt.Sprintf("block-%06d", len(blocks)+1),
					Type:        BlockTypePicture,
					Text:        "",
					Markdown:    "",
					HeadingPath: append([]string(nil), headingPath...),
					Source:      SourceLocation{Page: 0, BBox: nil},
					AssetRefs:   []string{asset.ID},
				})
			} else if warn != "" {
				*warnings = append(*warnings, warn)
			}
			continue
		}
		// 引用式图片定义行（[ref]: path）不进入正文。
		if markdownRefDefRe.MatchString(trimmed) {
			continue
		}
		// 行内图片：提取资产并从文本中移除语法（保留周围文字）。
		line = extractInlineImages(ctx, line, definitions, loader, assets, warnings)

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

// standaloneImageLine 判断整行是否只包含一张图片（行内式或引用式），返回 alt 与解析后的引用。
func standaloneImageLine(line string, definitions map[string]string) (alt, ref string, ok bool) {
	if m := markdownInlineImageRe.FindStringSubmatch(line); m != nil &&
		strings.TrimSpace(markdownInlineImageRe.ReplaceAllString(line, "")) == "" {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), true
	}
	if m := markdownRefImageRe.FindStringSubmatch(line); m != nil &&
		strings.TrimSpace(markdownRefImageRe.ReplaceAllString(line, "")) == "" {
		refKey := strings.TrimSpace(m[2])
		if path, found := definitions[strings.ToLower(refKey)]; found {
			return strings.TrimSpace(m[1]), path, true
		}
	}
	return "", "", false
}

// extractInlineImages 提取行内图片引用（行内式 + 引用式），返回移除图片语法后的行。
// 图片资产写入 assets；无法解析的引用记录 warning 并保持原语法（便于预览排查）。
func extractInlineImages(ctx context.Context, line string, definitions map[string]string, loader AssetLoader, assets *[]Asset, warnings *[]string) string {
	extract := func(alt, ref, original string) string {
		asset, warn := loadMarkdownImage(ctx, alt, ref, loader)
		if asset != nil {
			*assets = append(*assets, *asset)
			return ""
		}
		if warn != "" {
			*warnings = append(*warnings, warn)
		}
		// unresolved：保留原语法，正文不受影响。
		return original
	}
	line = markdownInlineImageRe.ReplaceAllStringFunc(line, func(match string) string {
		sub := markdownInlineImageRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		return extract(strings.TrimSpace(sub[1]), strings.TrimSpace(sub[2]), match)
	})
	line = markdownRefImageRe.ReplaceAllStringFunc(line, func(match string) string {
		sub := markdownRefImageRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		ref := strings.TrimSpace(sub[2])
		path, ok := definitions[strings.ToLower(ref)]
		if !ok {
			// 引用式定义缺失：保留原语法。
			return match
		}
		return extract(strings.TrimSpace(sub[1]), path, match)
	})
	return line
}

// loadMarkdownImage 读取图片引用生成 Asset；无法解析时返回 nil 与 warning。
// 支持：data URI、http(s) URL（走 AssetLoader）、附件相对路径（走 AssetLoader）。
// 本机绝对路径（如 C:\...）同样交给 AssetLoader 按文件名（basename）匹配附件，
// 匹配不到才视为 unresolved（正文导入不受影响）。
func loadMarkdownImage(ctx context.Context, alt, ref string, loader AssetLoader) (*Asset, string) {
	if dataURI, mime, ok := parseDataURI(ref); ok {
		if len(dataURI) > maxMarkdownImageBytes {
			return nil, fmt.Sprintf("图片 %q 超过大小限制 %d 字节", ref, maxMarkdownImageBytes)
		}
		return makeMarkdownAsset(alt, ref, mime, dataURI), ""
	}

	if loader == nil {
		if isWindowsAbsolutePath(ref) {
			return nil, fmt.Sprintf("图片 %q 为本机路径，导入时未随文档上传，已跳过（unresolved）", ref)
		}
		return nil, fmt.Sprintf("图片 %q 未解析（缺少资源加载器）", ref)
	}
	reader, contentType, err := loader.Open(ctx, ref)
	if err != nil {
		if isWindowsAbsolutePath(ref) {
			return nil, fmt.Sprintf("图片 %q 为本机路径，导入时未随文档上传，已跳过（unresolved）", ref)
		}
		return nil, fmt.Sprintf("图片 %q 未解析: %v", ref, err)
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, maxMarkdownImageBytes+1))
	if err != nil {
		return nil, fmt.Sprintf("图片 %q 读取失败: %v", ref, err)
	}
	if int64(len(data)) > maxMarkdownImageBytes {
		return nil, fmt.Sprintf("图片 %q 超过大小限制 %d 字节", ref, maxMarkdownImageBytes)
	}
	if len(data) == 0 {
		return nil, fmt.Sprintf("图片 %q 内容为空", ref)
	}
	mime := contentType
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		mime = mimeForExtension(path.Ext(ref))
	}
	return makeMarkdownAsset(alt, ref, mime, data), ""
}

// makeMarkdownAsset 由图片字节构造 Asset（含 base64 与哈希）。
func makeMarkdownAsset(alt, ref, mime string, data []byte) *Asset {
	digest := sha256.Sum256(data)
	refDigest := sha256.Sum256([]byte(ref))
	return &Asset{
		ID:         fmt.Sprintf("asset-%08x", refDigest[:4]),
		Kind:       "picture",
		MIMEType:   mime,
		SHA256:     hex.EncodeToString(digest[:]),
		Caption:    alt,
		DataBase64: base64.StdEncoding.EncodeToString(data),
		SourceRef:  ref,
		Metadata:   map[string]any{},
	}
}

// maxMarkdownImageBytes 是单张 Markdown 图片上限（与 Python 侧 MAX_ASSET_BYTES 对齐）。
const maxMarkdownImageBytes = 32 * 1024 * 1024

// parseDataURI 解析 data:image/xxx;base64,.... 内联图片。
func parseDataURI(ref string) (data []byte, mime string, ok bool) {
	lower := strings.ToLower(ref)
	if !strings.HasPrefix(lower, "data:image/") {
		return nil, "", false
	}
	comma := strings.Index(ref, ",")
	if comma < 0 {
		return nil, "", false
	}
	header := strings.ToLower(ref[5:comma])
	if !strings.Contains(header, "base64") {
		return nil, "", false
	}
	mime = strings.TrimSpace(strings.SplitN(header, ";", 2)[0])
	data, err := base64.StdEncoding.DecodeString(ref[comma+1:])
	if err != nil {
		return nil, "", false
	}
	return data, mime, true
}

// mimeForExtension 按扩展名推断图片 MIME。
func mimeForExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/png"
	}
}

// isWindowsAbsolutePath 判断 Windows 本机绝对路径（盘符前缀或 UNC）。
func isWindowsAbsolutePath(ref string) bool {
	if len(ref) >= 2 && ref[1] == ':' {
		c := ref[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return strings.HasPrefix(ref, `\\`) || strings.HasPrefix(ref, "/")
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
