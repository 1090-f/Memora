package chunking

import (
	"context"
	"fmt"

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
	switch strategy {
	case StrategyStructured:
	case StrategyParagraph:
		input = flattenCanonical(doc, false)
	case StrategyRecursive:
		input = flattenCanonical(doc, true)
	default:
		return nil, fmt.Errorf("不支持的 canonical 分块策略 %q", strategy)
	}
	chunks, err := c.ChunkCanonical(ctx, input, opts)
	if err != nil {
		return nil, err
	}
	for i := range chunks {
		chunks[i].Strategy = strategy
		chunks[i].StrategyVersion = opts.StrategyVersion
	}
	return chunks, nil
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
