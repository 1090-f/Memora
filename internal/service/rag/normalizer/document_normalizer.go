// Package normalizer 提供分块前的文档规范化（DocumentNormalizer）。
// 输入输出都是 parser.ParsedDocument，不生成 Chunk，不按 token 拆分。
package normalizer

import (
	"context"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// DocumentNormalizer 规范化 ParsedDocument：
//   - Unicode 与换行规范化、移除不可见字符与异常空白；
//   - 识别/移除重复页眉页脚；
//   - 规范 Block 类型、空 Block 与顺序；
//   - 规范 heading path；
//   - 修复安全可确定的引用（移除悬空引用并记录 warning）；
//   - 表格单元格文本规范化但不拆表；
//   - 保留 page、bbox、table_ref、asset_ref。
type DocumentNormalizer struct{}

// NewDocumentNormalizer 构造规范化器。
func NewDocumentNormalizer() *DocumentNormalizer { return &DocumentNormalizer{} }

// Normalize 就地规范化 ParsedDocument。
func (n *DocumentNormalizer) Normalize(_ context.Context, doc *parser.ParsedDocument) error {
	tableIDs := make(map[string]struct{}, len(doc.Tables))
	for _, table := range doc.Tables {
		tableIDs[table.ID] = struct{}{}
	}
	assetIDs := make(map[string]struct{}, len(doc.Assets))
	for _, asset := range doc.Assets {
		assetIDs[asset.ID] = struct{}{}
	}

	// 页眉页脚去重：记录已见过的页眉/页脚文本。
	seenHeaders := make(map[string]int)

	out := make([]parser.Block, 0, len(doc.Blocks))
	for _, block := range doc.Blocks {
		block.Type = strings.ToLower(strings.TrimSpace(block.Type))
		if block.Type == "" {
			block.Type = parser.BlockTypeUnknown
		}
		block.Text = normalizeText(block.Text)
		block.Markdown = normalizeText(block.Markdown)
		if block.Text == "" && block.Type != parser.BlockTypePicture {
			// 空正文 Block 无检索价值，直接移除（图片 Block 保留引用）。
			continue
		}

		switch block.Type {
		case parser.BlockTypePageHeader, parser.BlockTypePageFooter:
			if seenHeaders[block.Text] > 0 {
				// 重复页眉页脚：只保留首次出现。
				seenHeaders[block.Text]++
				continue
			}
			seenHeaders[block.Text]++
		case parser.BlockTypeHeading:
			if _, ok := tableIDs[block.TableRef]; ok {
				block.TableRef = ""
			}
		}

		// heading path 规范化：逐项 trim、去空、去相邻重复。
		block.HeadingPath = normalizeHeadingPath(block.HeadingPath)

		// 修复悬空引用：引用已不存在的表/图时移除并记录 warning。
		if block.TableRef != "" {
			if _, ok := tableIDs[block.TableRef]; !ok {
				doc.Warnings = append(doc.Warnings, "normalizer: 移除悬空 table_ref="+block.TableRef)
				block.TableRef = ""
			}
		}
		kept := block.AssetRefs[:0]
		for _, ref := range block.AssetRefs {
			if _, ok := assetIDs[ref]; ok {
				kept = append(kept, ref)
			} else {
				doc.Warnings = append(doc.Warnings, "normalizer: 移除悬空 asset_ref="+ref)
			}
		}
		block.AssetRefs = kept
		out = append(out, block)
	}
	doc.Blocks = out

	// 表格单元格文本规范化：统一换行与多余空白，不改变行列结构。
	for i := range doc.Tables {
		table := &doc.Tables[i]
		for r := range table.Headers {
			for c := range table.Headers[r] {
				table.Headers[r][c] = normalizeCellText(table.Headers[r][c])
			}
		}
		for r := range table.Rows {
			for c := range table.Rows[r] {
				table.Rows[r][c] = normalizeCellText(table.Rows[r][c])
			}
		}
		for c := range table.Cells {
			table.Cells[c].Text = normalizeCellText(table.Cells[c].Text)
		}
		table.Caption = normalizeText(table.Caption)
		table.Markdown = normalizeText(table.Markdown)
	}

	// 文档级文本规范化。
	doc.Document.Title = normalizeText(doc.Document.Title)
	doc.Document.Markdown = normalizeText(doc.Document.Markdown)

	// 资产 caption 规范化。
	for i := range doc.Assets {
		doc.Assets[i].Caption = normalizeText(doc.Assets[i].Caption)
	}
	return nil
}

// normalizeText 规范化正文：移除零宽字符/BOM、统一换行、压缩连续空行、清理行尾空白。
func normalizeText(content string) string {
	replacer := strings.NewReplacer(
		"\u200b", "", "\ufeff", "", "\r\n", "\n", "\r", "\n",
	)
	content = replacer.Replace(content)
	lines := strings.Split(content, "\n")
	var builder strings.Builder
	blankRun := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankRun++
			if blankRun > 1 {
				continue
			}
			builder.WriteString("\n")
			continue
		}
		blankRun = 0
		builder.WriteString(strings.TrimRight(line, " \t"))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

// normalizeCellText 规范化表格单元格：换行在拉丁字母间转为空格，其余情况移除。
func normalizeCellText(text string) string {
	text = strings.NewReplacer("\u200b", "", "\ufeff", "", "\r\n", "\n", "\r", "\n").Replace(text)
	runes := []rune(text)
	builder := strings.Builder{}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '\n' {
			builder.WriteRune(r)
			continue
		}
		prev := rune(0)
		if i > 0 {
			prev = runes[i-1]
		}
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if isLatin(prev) && isLatin(next) {
			builder.WriteRune(' ')
		}
	}
	return strings.TrimSpace(builder.String())
}

// isLatin 判断字符是否为 ASCII 字母数字。
func isLatin(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// normalizeHeadingPath 逐项 trim、去空与相邻重复。
func normalizeHeadingPath(path []string) []string {
	out := make([]string, 0, len(path))
	for _, item := range path {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1] == item {
			continue
		}
		out = append(out, item)
	}
	return out
}
