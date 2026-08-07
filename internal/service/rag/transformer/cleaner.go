// Package transformer 提供文档清洗、分段补充与 Token 计数等 Eino Transformer。
package transformer

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// Cleaner 清洗文档内容：去除不可见字符、规范空白、合并多余空行。
// 保留并扩展 MetaData，不改变 Document ID。
type Cleaner struct{}

// NewCleaner 构造清洗 Transformer。
func NewCleaner() *Cleaner { return &Cleaner{} }

// Transform 实现 Eino document.Transformer。
func (c *Cleaner) Transform(ctx context.Context, docs []*schema.Document, _ ...document.TransformerOption) ([]*schema.Document, error) {
	ctx = callbacks.EnsureRunInfo(ctx, c.GetType(), components.ComponentOfTransformer)
	ctx = callbacks.OnStart(ctx, &document.TransformerCallbackInput{Input: docs})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()
	for _, doc := range docs {
		doc.Content = cleanText(doc.Content)
	}
	_ = callbacks.OnEnd(ctx, &document.TransformerCallbackOutput{Output: docs})
	return docs, nil
}

// GetType 返回组件类型名。
func (c *Cleaner) GetType() string { return "Cleaner" }

// IsCallbacksEnabled 启用 Eino Callbacks。
func (c *Cleaner) IsCallbacksEnabled() bool { return true }

// cleanText 规范化正文：替换零宽字符、统一换行、合并连续空白行。
// 不做行级 TrimSpace——Markdown 缩进代码块与列表缩进无法可靠区分，
// 保留前导空白由分段器处理，避免破坏代码/列表语义。
func cleanText(content string) string {
	// 一次性移除零宽字符/BOM 并统一换行符，避免干扰分段与展示。
	replacer := strings.NewReplacer(
		"\u200b", "", "\ufeff", "", "\r\n", "\n", "\r", "\n",
	)
	content = replacer.Replace(content)
	lines := strings.Split(content, "\n")
	var builder strings.Builder
	// 连续空白行压缩为单个换行，作为段落分隔而不产生多余空行。
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
		// 行尾空白清理（不影响行首缩进）。
		builder.WriteString(strings.TrimRight(line, " \t"))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}
