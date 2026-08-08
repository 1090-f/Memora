package chunking

import (
	"context"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// StructureAwareChunker 按标题路径分组、保持阅读顺序的结构感知分块器。
//
// 规则：
//   - 不跨不相关一级标题合并；
//   - 超长 Block 内部 token-aware 拆分，overlap 只发生在拆分文本内部；
//   - 过短相邻 Chunk 只与结构兼容的相邻 Chunk 合并；
//   - code/formula 优先保持完整，超限才拆；
//   - caption 与其表格/图片尽量保持同一 Chunk；
//   - 每个 Chunk 保留 Block IDs、页码与 bbox。
type StructureAwareChunker struct {
	tokenizer       Tokenizer
	strategyVersion string
}

// NewStructureAwareChunker 构造结构感知分块器。
func NewStructureAwareChunker(tokenizer Tokenizer, strategyVersion string) *StructureAwareChunker {
	if strategyVersion == "" {
		strategyVersion = "structure-v1"
	}
	return &StructureAwareChunker{tokenizer: tokenizer, strategyVersion: strategyVersion}
}

// StrategyVersion 返回策略版本（参与 chunk_config_hash）。
func (c *StructureAwareChunker) StrategyVersion() string { return c.strategyVersion }

// Chunk 实现 Chunker。
func (c *StructureAwareChunker) Chunk(_ context.Context, doc *parser.ParsedDocument, opts ChunkOptions) ([]ParsedChunk, error) {
	if opts.MaxTokens <= 0 {
		return nil, fmt.Errorf("ChunkOptions.MaxTokens 必须为正数，实际 %d", opts.MaxTokens)
	}
	if opts.MinTokens < 0 {
		opts.MinTokens = 0
	}
	if opts.OverlapTokens < 0 {
		opts.OverlapTokens = 0
	}
	if opts.StrategyVersion == "" {
		opts.StrategyVersion = c.strategyVersion
	}

	units, err := c.buildUnits(doc, opts)
	if err != nil {
		return nil, err
	}
	units = c.attachCaptions(units)
	units, err = c.attachPictures(units, doc, opts)
	if err != nil {
		return nil, err
	}
	return c.assemble(units, opts)
}

// ---------------------------------------------------------------- 单元构建

// buildUnits 将 Block 按阅读顺序转为单元（不处理 caption/图片关联）。
func (c *StructureAwareChunker) buildUnits(doc *parser.ParsedDocument, opts ChunkOptions) ([]*unit, error) {
	tablesByID := make(map[string]parser.Table, len(doc.Tables))
	for _, table := range doc.Tables {
		tablesByID[table.ID] = table
	}
	assetsByID := make(map[string]parser.Asset, len(doc.Assets))
	for _, asset := range doc.Assets {
		assetsByID[asset.ID] = asset
	}

	text := &blockStrategy{}
	markdown := &markdownStrategy{}
	tableStrategy := &tableStrategy{tokenizer: c.tokenizer}
	picture := &pictureStrategy{tokenizer: c.tokenizer}

	var units []*unit
	for _, block := range doc.Blocks {
		switch block.Type {
		case parser.BlockTypeHeading, parser.BlockTypeTitle:
			// 标题作为上下文，不进入 Chunk 内容（heading_path 已携带）。
			continue
		case parser.BlockTypeTable:
			table, ok := tablesByID[block.TableRef]
			if !ok {
				continue
			}
			tableUnits, err := tableStrategy.toUnits(block, table, opts)
			if err != nil {
				return nil, err
			}
			units = append(units, tableUnits...)
		case parser.BlockTypePicture:
			var assets []parser.Asset
			for _, ref := range block.AssetRefs {
				if asset, ok := assetsByID[ref]; ok {
					assets = append(assets, asset)
				}
			}
			u, err := picture.toUnit(block, assets, nil, opts)
			if err != nil {
				return nil, err
			}
			if u != nil {
				units = append(units, u)
			}
		case parser.BlockTypeCode, parser.BlockTypeFormula:
			units = append(units, markdown.toUnit(block))
		default:
			if block.Text == "" {
				continue
			}
			units = append(units, text.toUnit(block))
		}
	}
	return units, nil
}

// attachCaptions 将紧跟 table/picture 单元的 caption 单元并入其中。
func (c *StructureAwareChunker) attachCaptions(units []*unit) []*unit {
	out := make([]*unit, 0, len(units))
	for i, u := range units {
		if u.contentType == parser.BlockTypeCaption && i+1 < len(units) && units[i+1].isTableOrPicture() {
			units[i+1].merge(u, "\n")
			units[i+1].contentType = joinUnique(units[i+1].contentType, parser.BlockTypeCaption)
			continue
		}
		out = append(out, u)
	}
	return out
}

// isTableOrPicture 判断单元是否表/图。
func (u *unit) isTableOrPicture() bool {
	return strings.Contains(u.contentType, parser.BlockTypeTable) ||
		strings.Contains(u.contentType, parser.BlockTypePicture)
}

// attachPictures 为图片单元关联同一标题路径下最近的前/后正文。
// 关联后超限则图片独立；正文单元若被合并则移除。
func (c *StructureAwareChunker) attachPictures(units []*unit, doc *parser.ParsedDocument, opts ChunkOptions) ([]*unit, error) {
	// 先按阅读顺序保留非图片单元，记录图片单元位置。
	ordered := make([]*unit, 0, len(units))
	pictureIdx := make([]int, 0, len(units))
	for _, u := range units {
		ordered = append(ordered, u)
		if strings.Contains(u.contentType, parser.BlockTypePicture) && u.seal {
			pictureIdx = append(pictureIdx, len(ordered)-1)
		}
	}
	for _, idx := range pictureIdx {
		pic := ordered[idx]
		// 前向找：图片之后最近的正文；找不到再后向找。
		contextUnit := c.nearestParagraphAfter(ordered, idx)
		if contextUnit == nil {
			contextUnit = c.nearestParagraphBefore(ordered, idx)
		}
		if contextUnit == nil {
			continue
		}
		combined := pic.text + "\n" + contextUnit.text
		tokens, err := c.tokenizer.Count(combined)
		if err != nil {
			return nil, err
		}
		if tokens > opts.MaxTokens {
			continue // 关联后超限：图片保持独立。
		}
		// 合并到正文：图片文本 + 正文，作为普通可合并单元。
		contextUnit.text = combined
		contextUnit.blockIDs = append(contextUnit.blockIDs, pic.blockIDs...)
		contextUnit.assetRefs = append(contextUnit.assetRefs, pic.assetRefs...)
		contextUnit.contentType = joinUnique(contextUnit.contentType, parser.BlockTypePicture)
		if contextUnit.source.Page == 0 {
			contextUnit.source = pic.source
		}
		// 图片单元从列表中移除（后续下标受影响，重新收集）。
		ordered = append(ordered[:idx], ordered[idx+1:]...)
		pictureIdx = nil
		for i, u := range ordered {
			if strings.Contains(u.contentType, parser.BlockTypePicture) && u.seal {
				pictureIdx = append(pictureIdx, i)
			}
		}
	}
	return ordered, nil
}

// nearestParagraphAfter 返回 idx 之后最近的普通正文单元（同一标题路径、页码接近）。
func (c *StructureAwareChunker) nearestParagraphAfter(units []*unit, idx int) *unit {
	for i := idx + 1; i < len(units); i++ {
		if !c.isNearbyParagraph(units[idx], units[i]) {
			continue
		}
		return units[i]
	}
	return nil
}

// nearestParagraphBefore 返回 idx 之前最近的普通正文单元（同一标题路径、页码接近）。
func (c *StructureAwareChunker) nearestParagraphBefore(units []*unit, idx int) *unit {
	for i := idx - 1; i >= 0; i-- {
		if !c.isNearbyParagraph(units[idx], units[i]) {
			continue
		}
		return units[i]
	}
	return nil
}

// isNearbyParagraph 判断候选单元是否可与图片关联。
func (c *StructureAwareChunker) isNearbyParagraph(picture, candidate *unit) bool {
	if !candidate.mergeable || !isPlainText(candidate.contentType) {
		return false
	}
	if !sameTopHeading(picture.headingPath, candidate.headingPath) {
		return false
	}
	// 页码差异过大不关联（跨页正文不关联，最多 2 页）。
	if picture.source.Page != 0 && candidate.source.Page != 0 &&
		abs(candidate.source.Page-picture.source.Page) > 2 {
		return false
	}
	return true
}

// isPlainText 判断单元是否为普通正文类型。
func isPlainText(contentType string) bool {
	switch strings.Split(contentType, "+")[0] {
	case parser.BlockTypeParagraph, parser.BlockTypeListItem,
		parser.BlockTypeFootnote, parser.BlockTypeUnknown,
		parser.BlockTypePageHeader, parser.BlockTypePageFooter:
		return true
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---------------------------------------------------------------- 组装

// assemble 将单元组装为 Chunk：
//   - 普通单元按 token 预算打包（预算扣除标题前缀），超限单元内部拆分；
//   - 独立单元（大表子块、独立图片）直接成块；
//   - 最后合并过短相邻 Chunk（结构兼容、不跨一级标题）。
func (c *StructureAwareChunker) assemble(units []*unit, opts ChunkOptions) ([]ParsedChunk, error) {
	chunks := make([]ParsedChunk, 0, len(units))
	var current []*unit
	var sectionPrefix string
	var prefixTokens int
	flush := func() {
		if len(current) == 0 {
			return
		}
		chunk, err := c.chunkFromUnits(current, sectionPrefix, opts)
		if err == nil {
			chunks = append(chunks, chunk)
		}
		current = nil
	}

	for _, u := range units {
		if strings.TrimSpace(u.text) == "" {
			continue
		}
		if u.seal {
			flush()
			chunk, err := c.chunkFromUnits([]*unit{u}, headingPrefixText(u.headingPath), opts)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, chunk)
			continue
		}
		// 一级标题变化：分区边界，先 flush。
		if len(current) > 0 && !sameTopHeading(current[0].headingPath, u.headingPath) {
			flush()
		}
		// 新分区：计算标题前缀预算。
		if len(current) == 0 {
			sectionPrefix = headingPrefixText(u.headingPath)
			tokens, err := c.tokenizer.Count(sectionPrefix)
			if err != nil {
				return nil, err
			}
			prefixTokens = tokens
		}
		current = append(current, u)
		if c.overBudget(current, prefixTokens, opts) {
			split, err := c.splitUnits(current, sectionPrefix, prefixTokens, opts)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, split...)
			current = nil
		}
	}
	flush()

	// 过短相邻合并：不跨一级标题、非独立类型、合并后不超限。
	merged := make([]ParsedChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if len(merged) == 0 {
			merged = append(merged, chunk)
			continue
		}
		last := &merged[len(merged)-1]
		if canMerge(last, &chunk, opts, c.tokenizer) {
			mergeChunks(last, &chunk)
			continue
		}
		merged = append(merged, chunk)
	}
	return merged, nil
}

// overBudget 判断 current 单元集是否超过预算（正文 token + 标题前缀）。
func (c *StructureAwareChunker) overBudget(current []*unit, prefixTokens int, opts ChunkOptions) bool {
	tokens := prefixTokens
	for _, u := range current {
		count, err := c.tokenizer.Count(u.text)
		if err != nil {
			return true
		}
		tokens += count
	}
	return tokens > opts.MaxTokens
}

// splitUnits 对超限单元集做 token-aware 拆分。
// 拆分预算 = MaxTokens - 标题前缀 token - overlap token；
// overlap 只发生在拆分文本内部，保证最终 Chunk 不超过 MaxTokens。
func (c *StructureAwareChunker) splitUnits(current []*unit, sectionPrefix string, prefixTokens int, opts ChunkOptions) ([]ParsedChunk, error) {
	budget := opts.MaxTokens - prefixTokens - opts.OverlapTokens
	if budget < 1 {
		budget = 1
	}
	refs := &unit{headingPath: current[0].headingPath}
	text := ""
	for _, u := range current {
		if text != "" {
			text += "\n"
		}
		text += u.text
		refs.blockIDs = append(refs.blockIDs, u.blockIDs...)
		refs.tableRefs = append(refs.tableRefs, u.tableRefs...)
		refs.assetRefs = append(refs.assetRefs, u.assetRefs...)
		refs.source = u.source
		refs.contentType = joinUnique(refs.contentType, u.contentType)
	}
	pieces, err := c.tokenizer.Split(text, budget, opts.OverlapTokens)
	if err != nil {
		return nil, err
	}
	var out []ParsedChunk
	for _, piece := range pieces {
		refs.text = piece
		chunk, err := c.chunkFromUnits([]*unit{refs}, sectionPrefix, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk)
	}
	return out, nil
}

// chunkFromUnits 将单元集转换为 ParsedChunk（标题前缀只添加一次）。
func (c *StructureAwareChunker) chunkFromUnits(units []*unit, sectionPrefix string, opts ChunkOptions) (ParsedChunk, error) {
	chunk := ParsedChunk{}
	seenTypes := make(map[string]bool)
	headingPath := units[0].headingPath
	for _, u := range units {
		if chunk.Content != "" {
			chunk.Content += "\n"
		}
		chunk.Content += u.text
		chunk.BlockIDs = append(chunk.BlockIDs, u.blockIDs...)
		chunk.TableRefs = append(chunk.TableRefs, u.tableRefs...)
		chunk.AssetRefs = append(chunk.AssetRefs, u.assetRefs...)
		for _, item := range strings.Split(u.contentType, "+") {
			if !seenTypes[item] {
				seenTypes[item] = true
				chunk.ContentTypes = append(chunk.ContentTypes, item)
			}
		}
		if chunk.SourceLocation.Page == 0 {
			chunk.SourceLocation = u.source
		}
	}
	chunk.HeadingPath = headingPath
	if sectionPrefix != "" && !strings.HasPrefix(chunk.Content, sectionPrefix+"\n") {
		chunk.Content = sectionPrefix + "\n" + chunk.Content
	}
	chunk.Content = strings.TrimSpace(chunk.Content)
	if chunk.Content == "" {
		return chunk, fmt.Errorf("空 Chunk")
	}
	count, err := c.tokenizer.Count(chunk.Content)
	if err != nil {
		return chunk, err
	}
	chunk.TokenCount = count
	return chunk, nil
}

// headingPrefixText 将标题路径渲染为前缀文本。
func headingPrefixText(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return strings.Join(path, " / ")
}

// canMerge 判断 Chunk b 是否可并入相邻 Chunk a：
// b 过短、同属一个一级标题、不含独立类型（大表/独立图片）、合并后不超限。
func canMerge(a, b *ParsedChunk, opts ChunkOptions, tk Tokenizer) bool {
	if b.TokenCount >= opts.MinTokens {
		return false
	}
	if !sameTopHeading(a.HeadingPath, b.HeadingPath) {
		return false
	}
	if hasSealedType(a) || hasSealedType(b) {
		return false
	}
	count, err := tk.Count(a.Content + "\n" + b.Content)
	if err != nil {
		return false
	}
	return count <= opts.MaxTokens
}

// hasSealedType 判断 Chunk 是否含独立类型（大表/独立图片）。
func hasSealedType(chunk *ParsedChunk) bool {
	for _, t := range chunk.ContentTypes {
		switch t {
		case parser.BlockTypeTable, parser.BlockTypePicture:
			return true
		}
	}
	return false
}

// sameTopHeading 判断两个 Chunk 是否属于同一一级标题（或同为无标题区）。
func sameTopHeading(a, b []string) bool {
	topA, topB := "", ""
	if len(a) > 0 {
		topA = a[0]
	}
	if len(b) > 0 {
		topB = b[0]
	}
	return topA == topB
}

// mergeChunks 将 b 并入 a。
func mergeChunks(a, b *ParsedChunk) {
	a.Content = strings.TrimSpace(a.Content) + "\n" + strings.TrimSpace(b.Content)
	a.BlockIDs = append(a.BlockIDs, b.BlockIDs...)
	a.TableRefs = append(a.TableRefs, b.TableRefs...)
	a.AssetRefs = append(a.AssetRefs, b.AssetRefs...)
	a.ContentTypes = append(a.ContentTypes, b.ContentTypes...)
	a.TokenCount += b.TokenCount
	if a.SourceLocation.Page == 0 {
		a.SourceLocation = b.SourceLocation
	}
}
