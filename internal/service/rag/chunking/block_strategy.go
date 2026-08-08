package chunking

import (
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// unit 是分块过程中的内容单元：一个或多个 Block 的聚合。
type unit struct {
	text        string
	blockIDs    []string
	contentType string
	source      parser.SourceLocation
	tableRefs   []string
	assetRefs   []string
	headingPath []string
	// seal 表示该单元必须独立成 Chunk（长表子块、独立图片等），不与相邻合并。
	seal bool
	// mergeable 表示可与相邻普通正文 Chunk 合并。
	mergeable bool
}

// merge 将 other 的文本与引用并入当前单元（用于 caption→table/picture 关联）。
func (u *unit) merge(other *unit, sep string) {
	if u.text != "" && other.text != "" {
		u.text += sep
	}
	u.text += other.text
	u.blockIDs = append(u.blockIDs, other.blockIDs...)
	u.tableRefs = append(u.tableRefs, other.tableRefs...)
	u.assetRefs = append(u.assetRefs, other.assetRefs...)
	if u.source.Page == 0 {
		u.source = other.source
	}
	// 合并后类型取复合描述。
	u.contentType = joinUnique(u.contentType, other.contentType)
}

// blockStrategy 将普通文本 Block 转换为内容单元。
// caption 直接并入其后的 table/picture 单元由编排器处理。
type blockStrategy struct{}

// toUnit 将 Block 转为 unit。
func (b *blockStrategy) toUnit(block parser.Block) *unit {
	return &unit{
		text:        block.Text,
		blockIDs:    []string{block.ID},
		contentType: block.Type,
		source:      block.Source,
		headingPath: block.HeadingPath,
		tableRefs:   []string{},
		assetRefs:   []string{},
		mergeable:   true,
	}
}

// caption 判断是否为 caption Block。
func (b *blockStrategy) caption(block parser.Block) bool {
	return block.Type == parser.BlockTypeCaption
}

// joinUnique 保序去重合并（a 在前）。
func joinUnique(a, b string) string {
	seen := make(map[string]bool)
	var out []string
	for _, part := range []string{a, b} {
		for _, item := range strings.Split(part, "+") {
			if !seen[item] {
				seen[item] = true
				out = append(out, item)
			}
		}
	}
	return strings.Join(out, "+")
}
