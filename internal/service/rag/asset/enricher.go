// Package asset 提供图片资产二次增强（AssetEnricher）扩展点。
// 增强由 Go 编排；实际 OCR/Vision 推理可调用独立模型服务。
// 默认 NoopEnricher：仅使用 Docling 提取的 caption、page 与位置信息。
package asset

import (
	"context"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// Enricher 对 ParsedDocument.Assets 做二次增强（OCR/Vision description 等）。
// 实现必须：失败时降级为原始 caption 并记录结构化 warning，不阻断分块。
type Enricher interface {
	// Enrich 就地增强 doc.Assets；增强结果写入 asset.Metadata。
	Enrich(ctx context.Context, doc *parser.ParsedDocument) error
}

// NoopEnricher 是默认实现：不做任何增强。
type NoopEnricher struct{}

// NewNoopEnricher 构造 NoopEnricher。
func NewNoopEnricher() *NoopEnricher { return &NoopEnricher{} }

// Enrich 实现 Enricher：空操作。
func (n *NoopEnricher) Enrich(_ context.Context, _ *parser.ParsedDocument) error { return nil }
