package transformer

import (
	"context"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// HeadingEnricher 是 Markdown 标题分段与 Recursive 分段的组合 Transformer。
// 输入文档先按 Markdown 标题切分（标题路径写入 heading_path），
// 再按 Token 上限用 Recursive Splitter 细分，保留标题上下文。
type HeadingEnricher struct {
	headerSplitter document.Transformer
	recursive      document.Transformer
	// maxChunkChars 是 Recursive Splitter 的最大字符数。
	maxChunkChars int
	// overlapChars 是 Recursive Splitter 的重叠字符数。
	overlapChars int
}

// NewHeadingEnricher 构造组合分段器。
func NewHeadingEnricher() (*HeadingEnricher, error) {
	ctx := context.Background()
	headerSplitter, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"#": "h1", "##": "h2", "###": "h3", "####": "h4", "#####": "h5",
		},
		// 标题行从正文移除（标题已进入 heading_path metadata），避免重复进入多个 chunk。
		TrimHeaders: true,
	})
	if err != nil {
		return nil, fmt.Errorf("构造 Markdown Header Splitter 失败: %w", err)
	}
	recursiveSplitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   defaultMaxChunkChars,
		OverlapSize: defaultChunkOverlap,
		LenFunc:     runeCount,
		Separators:  []string{"\n", "。", "！", "？", ".", "!", "?", "；", ";", " "},
	})
	if err != nil {
		return nil, fmt.Errorf("构造 Recursive Splitter 失败: %w", err)
	}
	return &HeadingEnricher{
		headerSplitter: headerSplitter,
		recursive:      recursiveSplitter,
		maxChunkChars:  defaultMaxChunkChars,
		overlapChars:   defaultChunkOverlap,
	}, nil
}

// 分段默认值（保守值，集中定义禁止散落魔法数字）。
const (
	defaultMaxChunkChars = 1000
	defaultChunkOverlap  = 100
)

// Transform 实现 Eino document.Transformer。
func (h *HeadingEnricher) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	ctx = callbacks.EnsureRunInfo(ctx, h.GetType(), components.ComponentOfTransformer)
	ctx = callbacks.OnStart(ctx, &document.TransformerCallbackInput{Input: docs})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	// 第一段：按 Markdown 标题切分，标题进入 heading_path 元数据。
	headerSplits, err := h.headerSplitter.Transform(ctx, docs, opts...)
	if err != nil {
		return nil, fmt.Errorf("Markdown 标题分段失败: %w", err)
	}
	// 将标题键（h1..h5）组合为 heading_path 数组。
	for _, doc := range headerSplits {
		path := headingPath(doc)
		if len(path) > 0 {
			if doc.MetaData == nil {
				doc.MetaData = make(map[string]any)
			}
			doc.MetaData[einoadapter.MetaHeadingPath] = path
			doc.MetaData[einoadapter.MetaContextTitle] = path[len(path)-1]
		}
	}

	// 第二段：超长块按 Recursive 细分，标题上下文随 metadata 保留。
	recursiveSplits, err := h.recursive.Transform(ctx, headerSplits, opts...)
	if err != nil {
		return nil, fmt.Errorf("Recursive 细分失败: %w", err)
	}
	_ = callbacks.OnEnd(ctx, &document.TransformerCallbackOutput{Output: recursiveSplits})
	return recursiveSplits, nil
}

// GetType 返回组件类型名。
func (h *HeadingEnricher) GetType() string { return "HeadingEnricher" }

// IsCallbacksEnabled 启用 Eino Callbacks。
func (h *HeadingEnricher) IsCallbacksEnabled() bool { return true }

// headingPath 从 doc.MetaData 提取 h1..h5 键并按层级排序组合为路径。
func headingPath(doc *schema.Document) []string {
	if doc.MetaData == nil {
		return nil
	}
	levels := []string{"h1", "h2", "h3", "h4", "h5"}
	var path []string
	// 按固定层级顺序拼接并跳过空标题，保证 heading_path 稳定有序。
	for _, key := range levels {
		if value, ok := doc.MetaData[key].(string); ok && strings.TrimSpace(value) != "" {
			path = append(path, strings.TrimSpace(value))
		}
	}
	return path
}

// runeCount 按 rune 计数（中英文一致）。
func runeCount(s string) int { return len([]rune(s)) }
