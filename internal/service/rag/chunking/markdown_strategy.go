package chunking

import "github.com/1090-f/Memora/internal/service/rag/parser"

// markdownStrategy 处理 code/formula Block：
// 优先保持完整（使用 Markdown 围栏文本），超限才交给 tokenizer 拆分。
type markdownStrategy struct{}

// toUnit 将 code/formula Block 转为单元。
func (m *markdownStrategy) toUnit(block parser.Block) *unit {
	content := block.Markdown
	if content == "" {
		content = block.Text
	}
	return &unit{
		text:        content,
		blockIDs:    []string{block.ID},
		contentType: block.Type,
		source:      block.Source,
		headingPath: block.HeadingPath,
		mergeable:   true,
	}
}
