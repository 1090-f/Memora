package chunking

import (
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// pictureStrategy 处理图片 Block：
//   - 使用 caption 与可选的 OCR/Vision description 生成检索文本；
//   - 与同一标题路径下最近的前后正文关联；
//   - 关联后超出 MaxTokens 则独立成 Chunk；
//   - 无任何文字信息的图片只保存资产和 source reference，不生成空 Chunk；
//   - 图片二进制不进入 Chunk。
type pictureStrategy struct {
	tokenizer Tokenizer
}

// ocrText 从资产元数据提取图片 OCR 文字（OCR 节点写入）。
func ocrTextOf(asset parser.Asset) string {
	if asset.Metadata == nil {
		return ""
	}
	value, _ := asset.Metadata["ocr_text"].(string)
	return strings.TrimSpace(value)
}

// description 从资产元数据提取增强描述（AssetEnricher 写入）。
func descriptionOf(asset parser.Asset) string {
	if asset.Metadata == nil {
		return ""
	}
	value, _ := asset.Metadata["description"].(string)
	return strings.TrimSpace(value)
}

// pictureText 生成图片检索文本（OCR 文字优先，其次 caption，最后增强描述）。
// caption 与 OCR 文字相同时只保留一份，避免重复文本进入 Chunk。
func pictureText(asset parser.Asset) string {
	var parts []string
	ocr := ocrTextOf(asset)
	caption := strings.TrimSpace(asset.Caption)
	if ocr != "" {
		parts = append(parts, ocr)
		if caption != "" && caption != ocr {
			parts = append(parts, caption)
		}
	} else if caption != "" {
		parts = append(parts, caption)
	}
	if desc := descriptionOf(asset); desc != "" {
		parts = append(parts, desc)
	}
	return strings.Join(parts, "\n")
}

// toUnit 将图片 Block 转为单元。
//
// contextUnit 是与图片关联的相邻正文（最近的前/后段落，同标题路径）。
// 返回 nil 表示无任何文字信息，不生成空 Chunk。
func (p *pictureStrategy) toUnit(block parser.Block, assets []parser.Asset, contextUnit *unit, opts ChunkOptions) (*unit, error) {
	var textParts []string
	var assetRefs []string
	for _, asset := range assets {
		if asset.Omitted {
			continue
		}
		textParts = append(textParts, pictureText(asset))
		assetRefs = append(assetRefs, asset.ID)
	}
	text := strings.Join(textParts, "\n")

	u := &unit{
		blockIDs:    []string{block.ID},
		contentType: parser.BlockTypePicture,
		source:      block.Source,
		headingPath: block.HeadingPath,
		assetRefs:   assetRefs,
	}
	// 无任何文字信息：只保留资产引用，不生成空 Chunk。
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	u.text = text

	if contextUnit != nil {
		combined := text + "\n" + contextUnit.text
		tokens, err := p.tokenizer.Count(combined)
		if err != nil {
			return nil, err
		}
		if tokens <= opts.MaxTokens {
			// 关联后未超限：图片与正文合并为同一单元。
			u.text = combined
			u.merge(contextUnit, "")
			u.contentType = joinUnique(parser.BlockTypePicture, contextUnit.contentType)
			u.mergeable = true
			return u, nil
		}
		// 关联后超限：图片独立成 Chunk，正文保持独立单元。
		u.seal = true
		return u, nil
	}
	u.seal = true
	return u, nil
}
