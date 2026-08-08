package chunking

import (
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// tableStrategy 处理表格 Block：
//   - 小表整体成为一个可合并单元；
//   - 大表按行分割，每个子单元重复 caption 与表头；
//   - 单行自身超限时在单元格文本内部安全拆分；
//   - 保留 table_ref、行范围、页码与 bbox。
//
// 表格确定性检索文本：标题路径 → Caption → 表头 → 行范围 → 行内容。
type tableStrategy struct {
	tokenizer Tokenizer
}

// toUnits 将表格 Block 转为一个或多个单元（大表拆行）。
func (t *tableStrategy) toUnits(block parser.Block, table parser.Table, opts ChunkOptions) ([]*unit, error) {
	full := t.buildText(block.HeadingPath, table, -1, -1)
	tokens, err := t.tokenizer.Count(full)
	if err != nil {
		return nil, err
	}
	if tokens <= opts.MaxTokens {
		// 小表整体：可合并单元（caption 与表格同块）。
		return []*unit{{
			text:        full,
			blockIDs:    []string{block.ID},
			contentType: parser.BlockTypeTable,
			source:      block.Source,
			tableRefs:   []string{table.ID},
			mergeable:   true,
		}}, nil
	}

	// 大表按行拆分：表头（+caption）作为每个子单元的前缀。
	headerBlock := buildHeaders(table.Headers)
	headerPrefix := buildTablePrefix(block.HeadingPath, table.Caption) + "\n" + headerBlock

	units := make([]*unit, 0, 1+len(table.Rows)/8)
	batch := make([]string, 0, 8)
	rowRangeStart := 0
	flush := func(rowRangeEnd int) {
		if len(batch) == 0 {
			return
		}
		// 行范围（1 起始）：第 a-b 行。
		rangeText := fmt.Sprintf("（第 %d-%d 行）", rowRangeStart+1, rowRangeEnd)
		text := headerPrefix + "\n" + rangeText + "\n" + strings.Join(batch, "\n")
		units = append(units, &unit{
			text:        text,
			blockIDs:    []string{block.ID},
			contentType: parser.BlockTypeTable,
			source:      block.Source,
			tableRefs:   []string{table.ID},
			seal:        true, // 大表子块独立，不与正文合并
		})
		batch = batch[:0]
	}

	// 预测式预算校验：headerPrefix + 行范围 + 当前批次 + 新行 的总 token 数。
	prospective := func(rowText string, rowRangeEnd int) (int, error) {
		rangeText := fmt.Sprintf("（第 %d-%d 行）", rowRangeStart+1, rowRangeEnd)
		text := headerPrefix + "\n" + rangeText + "\n"
		if len(batch) > 0 {
			text += strings.Join(batch, "\n") + "\n"
		}
		text += rowText
		return t.tokenizer.Count(text)
	}

	for i, row := range table.Rows {
		rowText := buildRow(row)
		rowTokens, err := t.tokenizer.Count(rowText)
		if err != nil {
			return nil, err
		}
		if rowTokens > opts.MaxTokens {
			// 单行超限：单元格文本内部安全拆分。
			flush(i)
			pieces, err := t.tokenizer.Split(rowText, opts.MaxTokens, 0)
			if err != nil {
				return nil, err
			}
			for _, piece := range pieces {
				units = append(units, &unit{
					text:        headerPrefix + "\n" + piece,
					blockIDs:    []string{block.ID},
					contentType: parser.BlockTypeTable,
					source:      block.Source,
					tableRefs:   []string{table.ID},
					seal:        true,
				})
			}
			continue
		}
		if len(batch) > 0 {
			tokens, err := prospective(rowText, i+1)
			if err != nil {
				return nil, err
			}
			if tokens > opts.MaxTokens {
				flush(i)
				rowRangeStart = i
			}
		}
		batch = append(batch, rowText)
	}
	flush(len(table.Rows))
	return units, nil
}

// buildText 构建整表检索文本；rowStart/rowEnd < 0 表示全表。
func (t *tableStrategy) buildText(headingPath []string, table parser.Table, rowStart, rowEnd int) string {
	prefix := buildTablePrefix(headingPath, table.Caption)
	var builder strings.Builder
	builder.WriteString(prefix)
	builder.WriteString("\n")
	builder.WriteString(buildHeaders(table.Headers))
	if rowStart < 0 {
		for _, row := range table.Rows {
			builder.WriteString("\n")
			builder.WriteString(buildRow(row))
		}
		return builder.String()
	}
	for _, row := range table.Rows[rowStart:rowEnd] {
		builder.WriteString("\n")
		builder.WriteString(buildRow(row))
	}
	return builder.String()
}

// buildTablePrefix 构建标题路径 + caption 前缀。
func buildTablePrefix(headingPath []string, caption string) string {
	var builder strings.Builder
	if len(headingPath) > 0 {
		builder.WriteString(strings.Join(headingPath, " / "))
	}
	if caption != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(caption)
	}
	return builder.String()
}

// buildHeaders 构建表头文本（保持 markdown 表格风格）。
func buildHeaders(headers [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, row := range headers {
		builder.WriteString("| ")
		builder.WriteString(strings.Join(row, " | "))
		builder.WriteString(" |")
		builder.WriteString("\n")
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

// buildRow 构建单行文本。
func buildRow(row []string) string {
	return "| " + strings.Join(row, " | ") + " |"
}
