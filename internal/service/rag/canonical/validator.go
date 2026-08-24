package canonical

import (
	"fmt"
	"unicode/utf8"
)

// DocumentValidator validates the stable canonical contract.
type DocumentValidator struct{}

// NewValidator creates the default validator.
func NewValidator() *DocumentValidator { return &DocumentValidator{} }

// Validate checks schema, byte boundaries, node slices, source mappings and hash.
func (v *DocumentValidator) Validate(doc *CanonicalDocument) error {
	if doc == nil {
		return fmt.Errorf("CanonicalDocument 不能为空")
	}
	if doc.SchemaVersion != SchemaVersion {
		return fmt.Errorf("CanonicalDocument schema_version=%q，期望 %q", doc.SchemaVersion, SchemaVersion)
	}
	if doc.RendererVersion == "" {
		return fmt.Errorf("CanonicalDocument renderer_version 不能为空")
	}
	if !utf8.ValidString(doc.Markdown) {
		return fmt.Errorf("CanonicalDocument markdown 不是合法 UTF-8")
	}
	seenIDs := make(map[string]struct{}, len(doc.Nodes))
	lastNodeEnd := 0
	for i, node := range doc.Nodes {
		if node.ID == "" {
			return fmt.Errorf("CanonicalNode %d ID 不能为空", i)
		}
		if _, exists := seenIDs[node.ID]; exists {
			return fmt.Errorf("CanonicalNode ID 重复: %s", node.ID)
		}
		seenIDs[node.ID] = struct{}{}
		if node.StartByte < lastNodeEnd || node.StartByte < 0 || node.EndByte < node.StartByte || node.EndByte > len(doc.Markdown) {
			return fmt.Errorf("CanonicalNode %s byte 区间非法 [%d,%d)", node.ID, node.StartByte, node.EndByte)
		}
		if !isUTF8Boundary(doc.Markdown, node.StartByte) || !isUTF8Boundary(doc.Markdown, node.EndByte) {
			return fmt.Errorf("CanonicalNode %s byte 区间未落在 UTF-8 边界", node.ID)
		}
		if node.Markdown != doc.Markdown[node.StartByte:node.EndByte] {
			return fmt.Errorf("CanonicalNode %s markdown 与文档区间不一致", node.ID)
		}
		lastNodeEnd = node.EndByte
	}
	lastSpanEnd := 0
	for i, span := range doc.SourceMap {
		if span.StartByte < lastSpanEnd || span.StartByte < 0 || span.EndByte <= span.StartByte || span.EndByte > len(doc.Markdown) {
			return fmt.Errorf("SourceSpan %d byte 区间非法 [%d,%d)", i, span.StartByte, span.EndByte)
		}
		if !isUTF8Boundary(doc.Markdown, span.StartByte) || !isUTF8Boundary(doc.Markdown, span.EndByte) {
			return fmt.Errorf("SourceSpan %d 未落在 UTF-8 边界", i)
		}
		if !span.Generated && len(span.Sources) == 0 {
			return fmt.Errorf("SourceSpan %d 非生成文本却没有来源", i)
		}
		lastSpanEnd = span.EndByte
	}
	hash, err := HashDocument(doc)
	if err != nil {
		return fmt.Errorf("计算 CanonicalDocument hash 失败: %w", err)
	}
	if doc.ContentHash != hash {
		return fmt.Errorf("CanonicalDocument content_hash 不匹配")
	}
	return nil
}

func isUTF8Boundary(value string, offset int) bool {
	if offset == 0 || offset == len(value) {
		return true
	}
	return offset > 0 && offset < len(value) && utf8.RuneStart(value[offset])
}
