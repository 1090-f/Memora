package chunking

import (
	"context"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/canonical"
)

// ChunkCanonicalStrategy executes one deterministic strategy over typed nodes.
// Structured preserves heading paths; paragraph/recursive flatten unreliable
// headings while retaining atomic table/picture/code/formula nodes.
func (c *StructureAwareChunker) ChunkCanonicalStrategy(ctx context.Context, doc *canonical.CanonicalDocument, opts ChunkOptions, strategy string) ([]ParsedChunk, error) {
	if doc == nil {
		return nil, fmt.Errorf("CanonicalDocument 不能为空")
	}
	input := doc
	var chunks []ParsedChunk
	var err error
	switch strategy {
	case StrategyStructured:
	case StrategyParagraph:
		input = flattenCanonical(doc, false)
	case StrategyRecursive:
		chunks, err = c.chunkRecursiveCanonical(ctx, doc, opts)
	default:
		return nil, fmt.Errorf("不支持的 canonical 分块策略 %q", strategy)
	}
	if strategy != StrategyRecursive {
		chunks, err = c.ChunkCanonical(ctx, input, opts)
	}
	if err != nil {
		return nil, err
	}
	for i := range chunks {
		chunks[i].Strategy = strategy
		chunks[i].StrategyVersion = strategyVersion(strategy, opts.StrategyVersion)
		chunks[i].SourceSpans = canonical.SelectChunkSourceSpans(
			doc, chunks[i].Content, chunks[i].BlockIDs, chunks[i].TableRefs, chunks[i].AssetRefs,
		)
	}
	MarkOverlapSpans(chunks)
	return chunks, nil
}

func strategyVersion(strategy, structuredVersion string) string {
	switch strategy {
	case StrategyParagraph:
		return ParagraphVersion
	case StrategyRecursive:
		return RecursiveVersion
	default:
		return structuredVersion
	}
}

// chunkRecursiveCanonical 对弱结构文档按连续文本 run 递归回退切分，同时把
// table/picture/code/formula 等 Atomic Node 交回专用 splitter，避免退化为纯字符串扫描。
func (c *StructureAwareChunker) chunkRecursiveCanonical(ctx context.Context, doc *canonical.CanonicalDocument, opts ChunkOptions) ([]ParsedChunk, error) {
	var chunks []ParsedChunk
	var run []canonical.CanonicalNode
	flushRun := func() error {
		if len(run) == 0 {
			return nil
		}
		textParts := make([]string, 0, len(run))
		refs := ParsedChunk{Strategy: StrategyRecursive, StrategyVersion: RecursiveVersion}
		for _, node := range run {
			text := strings.TrimSpace(node.Text)
			if text == "" {
				text = strings.TrimSpace(node.Markdown)
			}
			if text != "" {
				textParts = append(textParts, text)
			}
			refs.BlockIDs = append(refs.BlockIDs, node.BlockIDs...)
			refs.TableRefs = appendNonEmpty(refs.TableRefs, node.TableRef)
			refs.AssetRefs = append(refs.AssetRefs, node.AssetRefs...)
			refs.ContentTypes = appendUniqueString(refs.ContentTypes, canonicalContentType(node.Kind))
			if refs.SourceLocation.Page == 0 {
				refs.SourceLocation = primarySource(node.Sources)
			}
		}
		text := strings.Join(textParts, "\n\n")
		pieces, splitErr := c.tokenizer.Split(text, opts.MaxTokens, opts.OverlapTokens)
		if splitErr != nil {
			return splitErr
		}
		for _, piece := range pieces {
			chunk := refs
			chunk.Content = piece
			count, countErr := c.tokenizer.Count(piece)
			if countErr != nil {
				return countErr
			}
			chunk.TokenCount = count
			chunks = append(chunks, chunk)
		}
		run = nil
		return nil
	}

	for _, node := range doc.Nodes {
		if (node.Kind == canonical.NodeKindPageHeader || node.Kind == canonical.NodeKindPageFooter) && strings.TrimSpace(node.Markdown) == "" {
			continue
		}
		if !node.Atomic && !isCanonicalAtomicKind(node.Kind) {
			run = append(run, node)
			continue
		}
		if err := flushRun(); err != nil {
			return nil, err
		}
		subdoc := *doc
		subdoc.Nodes = []canonical.CanonicalNode{node}
		atomicChunks, err := c.ChunkCanonical(ctx, &subdoc, opts)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, atomicChunks...)
	}
	if err := flushRun(); err != nil {
		return nil, err
	}
	return chunks, nil
}

func isCanonicalAtomicKind(kind canonical.NodeKind) bool {
	switch kind {
	case canonical.NodeKindTable, canonical.NodeKindPicture, canonical.NodeKindCode, canonical.NodeKindFormula:
		return true
	default:
		return false
	}
}

func appendNonEmpty(values []string, value string) []string {
	if value == "" {
		return values
	}
	return append(values, value)
}

func appendUniqueString(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func flattenCanonical(doc *canonical.CanonicalDocument, recursive bool) *canonical.CanonicalDocument {
	copyDoc := *doc
	copyDoc.Nodes = make([]canonical.CanonicalNode, len(doc.Nodes))
	for i, node := range doc.Nodes {
		copyDoc.Nodes[i] = node
		copyDoc.Nodes[i].HeadingPath = nil
		switch node.Kind {
		case canonical.NodeKindTable, canonical.NodeKindPicture,
			canonical.NodeKindCode, canonical.NodeKindFormula, canonical.NodeKindCaption:
			// Atomic nodes retain their specialized type.
		case canonical.NodeKindPageHeader, canonical.NodeKindPageFooter:
			// Default renderer leaves these nodes empty; keep their type.
		default:
			// In paragraph/recursive modes headings are searchable text nodes rather
			// than structural boundaries. Recursive currently shares atomic-aware
			// aggregation but is explicitly versioned for future separator tuning.
			copyDoc.Nodes[i].Kind = canonical.NodeKindParagraph
			if recursive {
				copyDoc.Nodes[i].Metadata = mergeMetadata(node.Metadata, map[string]any{"fallback_recursive": true})
			}
		}
	}
	return &copyDoc
}

func mergeMetadata(base, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}
