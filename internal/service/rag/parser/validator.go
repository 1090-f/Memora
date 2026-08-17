package parser

import (
	"encoding/base64"
	"strings"
)

// ValidateLimits 定义 ParsedDocument 的资源限制。
type ValidateLimits struct {
	// MaxPageCount 是页数上限；0 表示不限制。
	MaxPageCount int
	// MaxBlocks 是 Block 数量上限。
	MaxBlocks int
	// MaxTables 是 Table 数量上限。
	MaxTables int
	// MaxAssets 是 Asset 数量上限。
	MaxAssets int
	// MaxAssetBytes 是单个 Asset base64 解码后的最大字节数；0 表示不限制。
	MaxAssetBytes int64
	// MaxTotalAssetBytes 是所有资产 base64 解码后的总大小上限。
	MaxTotalAssetBytes int64
	// MaxWarnings 是 warning 数量上限（防止响应膨胀）。
	MaxWarnings int
}

// DefaultValidateLimits 返回保守默认值（与 Python 服务限制一致或更严格）。
func DefaultValidateLimits() ValidateLimits {
	return ValidateLimits{
		MaxPageCount:       500,
		MaxBlocks:          100000,
		MaxTables:          10000,
		MaxAssets:          100,
		MaxAssetBytes:      32 * 1024 * 1024,
		MaxTotalAssetBytes: 64 * 1024 * 1024,
		MaxWarnings:        1000,
	}
}

// ValidateParsedDocument 校验 ParsedDocument：
//   - schema 版本受支持；
//   - source 哈希与期望一致（期望为空时跳过）；
//   - Block/Table/Asset 引用完整；
//   - 资源限制（页数、数量、资产大小）不超限；
//   - 资产 base64 合法。
//
// 任一校验失败返回 *ParseError（分类 ParseErrorInvalidResponse）。
func ValidateParsedDocument(doc *ParsedDocument, expectedSourceSHA256 string, limits ValidateLimits) error {
	if err := CheckSchemaVersion(doc.SchemaVersion); err != nil {
		return ParseErrorf(ParseErrorInvalidResponse, "schema 校验失败: %v", err)
	}
	if doc.Parser.Name == "" || doc.Parser.Version == "" {
		return ParseErrorf(ParseErrorInvalidResponse, "ParsedDocument 缺少 parser.name/version")
	}
	if doc.Source.SHA256 == "" {
		return ParseErrorf(ParseErrorInvalidResponse, "ParsedDocument 缺少 source.sha256")
	}
	if expectedSourceSHA256 != "" && !strings.EqualFold(doc.Source.SHA256, expectedSourceSHA256) {
		return ParseErrorf(ParseErrorInvalidResponse,
			"source sha256 不一致：期望 %s，实际 %s", expectedSourceSHA256, doc.Source.SHA256)
	}

	if limits.MaxPageCount > 0 && doc.Document.PageCount > limits.MaxPageCount {
		return ParseErrorf(ParseErrorInvalidResponse, "页数 %d 超过限制 %d", doc.Document.PageCount, limits.MaxPageCount)
	}
	if limits.MaxBlocks > 0 && len(doc.Blocks) > limits.MaxBlocks {
		return ParseErrorf(ParseErrorInvalidResponse, "Block 数量 %d 超过限制 %d", len(doc.Blocks), limits.MaxBlocks)
	}
	if limits.MaxTables > 0 && len(doc.Tables) > limits.MaxTables {
		return ParseErrorf(ParseErrorInvalidResponse, "Table 数量 %d 超过限制 %d", len(doc.Tables), limits.MaxTables)
	}
	if limits.MaxAssets > 0 && len(doc.Assets) > limits.MaxAssets {
		return ParseErrorf(ParseErrorInvalidResponse, "Asset 数量 %d 超过限制 %d", len(doc.Assets), limits.MaxAssets)
	}
	if limits.MaxWarnings > 0 && len(doc.Warnings) > limits.MaxWarnings {
		return ParseErrorf(ParseErrorInvalidResponse, "Warning 数量 %d 超过限制 %d", len(doc.Warnings), limits.MaxWarnings)
	}

	if err := validateReferences(doc); err != nil {
		return err
	}
	if err := validateAssets(doc, limits); err != nil {
		return err
	}
	return nil
}

// validateReferences 验证所有 Block/Table/Asset 引用存在。
func validateReferences(doc *ParsedDocument) error {
	tableIDs := make(map[string]struct{}, len(doc.Tables))
	for _, table := range doc.Tables {
		if table.ID == "" {
			return ParseErrorf(ParseErrorInvalidResponse, "Table 缺少 id")
		}
		tableIDs[table.ID] = struct{}{}
	}
	assetIDs := make(map[string]struct{}, len(doc.Assets))
	for _, asset := range doc.Assets {
		if asset.ID == "" {
			return ParseErrorf(ParseErrorInvalidResponse, "Asset 缺少 id")
		}
		assetIDs[asset.ID] = struct{}{}
	}
	blockIDs := make(map[string]struct{}, len(doc.Blocks))
	for _, block := range doc.Blocks {
		if block.ID == "" {
			return ParseErrorf(ParseErrorInvalidResponse, "Block 缺少 id")
		}
		blockIDs[block.ID] = struct{}{}
	}
	for _, block := range doc.Blocks {
		if block.TableRef != "" {
			if _, ok := tableIDs[block.TableRef]; !ok {
				return ParseErrorf(ParseErrorInvalidResponse,
					"Block %q 引用不存在的 table_ref=%q", block.ID, block.TableRef)
			}
		}
		for _, ref := range block.AssetRefs {
			if _, ok := assetIDs[ref]; !ok {
				return ParseErrorf(ParseErrorInvalidResponse,
					"Block %q 引用不存在的 asset_ref=%q", block.ID, ref)
			}
		}
	}
	return nil
}

// validateAssets 校验资产 base64 合法性与大小限制。
// 解码前先用 DecodedLen 预检单图与总量上限，避免恶意超大 base64 先完整解码撑爆内存。
func validateAssets(doc *ParsedDocument, limits ValidateLimits) error {
	var total int64
	for _, asset := range doc.Assets {
		if asset.Omitted {
			continue
		}
		if asset.DataBase64 == "" && asset.ObjectKey == "" {
			return ParseErrorf(ParseErrorInvalidResponse,
				"Asset %q 既无 data_base64 也无 object_key", asset.ID)
		}
		if asset.DataBase64 == "" {
			continue
		}
		decodedLen := int64(base64.StdEncoding.DecodedLen(len(asset.DataBase64)))
		if limits.MaxAssetBytes > 0 && decodedLen > limits.MaxAssetBytes {
			return ParseErrorf(ParseErrorInvalidResponse,
				"Asset %q 大小 %d 字节超过单图限制 %d", asset.ID, decodedLen, limits.MaxAssetBytes)
		}
		data, err := base64.StdEncoding.DecodeString(asset.DataBase64)
		if err != nil {
			return ParseErrorf(ParseErrorInvalidResponse, "Asset %q base64 解码失败: %v", asset.ID, err)
		}
		total += int64(len(data))
	}
	if limits.MaxTotalAssetBytes > 0 && total > limits.MaxTotalAssetBytes {
		return ParseErrorf(ParseErrorInvalidResponse,
			"资产总量 %d 字节超过限制 %d", total, limits.MaxTotalAssetBytes)
	}
	return nil
}
