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
// sheetMode 是电子表格模式（xlsx）：小表也独立成块（seal），不与正文合并，
// 保证每个 Sheet/Table 的检索完整性。
func (t *tableStrategy) toUnits(block parser.Block, table parser.Table, opts ChunkOptions, sheetMode bool) ([]*unit, error) {
	full := t.buildText(block.HeadingPath, table, -1, -1)
	tokens, err := t.tokenizer.Count(full)
	if err != nil {
		return nil, err
	}
	if tokens <= opts.MaxTokens {
		// 小表整体：可合并单元（caption 与表格同块）；xlsx 模式独立成块。
		return []*unit{{
			text:        full,
			blockIDs:    []string{block.ID},
			contentType: parser.BlockTypeTable,
			source:      block.Source,
			tableRefs:   []string{table.ID},
			mergeable:   !sheetMode,
			seal:        sheetMode,
		}}, nil
	}

	// 大表按行拆分：标题路径与 caption 始终保留；表头在首块保留，
	// RepeatTableHead=true 时在后续子块重复。
	headerBlock := buildHeaders(table.Headers)
	basePrefix := buildTablePrefix(block.HeadingPath, table.Caption)

	units := make([]*unit, 0, 1+len(table.Rows)/8)
	batch := make([]string, 0, 8)
	rowRangeStart := 0
	chunkPrefix := func(first bool) string {
		if headerBlock != "" && (first || opts.RepeatTableHead) {
			return joinNonEmptyLines(basePrefix, headerBlock)
		}
		return basePrefix
	}
	flush := func(rowRangeEnd int) {
		if len(batch) == 0 {
			return
		}
		// 行范围（1 起始）：第 a-b 行。
		rangeText := fmt.Sprintf("（第 %d-%d 行）", rowRangeStart+1, rowRangeEnd)
		text := joinNonEmptyLines(chunkPrefix(len(units) == 0), rangeText, strings.Join(batch, "\n"))
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
		parts := []string{chunkPrefix(len(units) == 0), rangeText}
		if len(batch) > 0 {
			parts = append(parts, strings.Join(batch, "\n"))
		}
		parts = append(parts, rowText)
		text := joinNonEmptyLines(parts...)
		return t.tokenizer.Count(text)
	}

	for i, row := range table.Rows {
		rowText := buildRow(row)
		rowTokens, err := t.tokenizer.Count(rowText)
		if err != nil {
			return nil, err
		}
		prefix := joinNonEmptyLines(
			chunkPrefix(len(units) == 0),
			fmt.Sprintf("（第 %d-%d 行）", i+1, i+1),
		)
		prefixForBudget := prefix
		if prefixForBudget != "" {
			prefixForBudget += "\n"
		}
		prefixTokens, err := t.tokenizer.Count(prefixForBudget)
		if err != nil {
			return nil, err
		}
		if rowTokens+prefixTokens > opts.MaxTokens {
			// 单行超限：单元格文本内部安全拆分。
			flush(i)
			prefix = joinNonEmptyLines(
				chunkPrefix(len(units) == 0),
				fmt.Sprintf("（第 %d-%d 行）", i+1, i+1),
			)
			prefixForBudget = prefix
			if prefixForBudget != "" {
				prefixForBudget += "\n"
			}
			prefixTokens, err = t.tokenizer.Count(prefixForBudget)
			if err != nil {
				return nil, err
			}
			budget := opts.MaxTokens - prefixTokens
			if budget < 1 {
				return nil, fmt.Errorf("表格 %s 的标题/caption/表头前缀已超过 MaxTokens", table.ID)
			}
			pieces, err := t.tokenizer.Split(rowText, budget, 0)
			if err != nil {
				return nil, err
			}
			for _, piece := range pieces {
				piecePrefix := joinNonEmptyLines(
					chunkPrefix(len(units) == 0),
					fmt.Sprintf("（第 %d-%d 行）", i+1, i+1),
				)
				units = append(units, &unit{
					text:        joinNonEmptyLines(piecePrefix, piece),
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

// joinNonEmptyLines 以换行连接非空文本，避免空 caption/header 产生多余空行。
func joinNonEmptyLines(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			filtered = append(filtered, value)
		}
	}
	return strings.Join(filtered, "\n")
}
