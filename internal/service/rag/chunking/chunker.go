// Package chunking 提供结构感知分块（StructureAwareChunker）。
// 输入统一为 parser.ParsedDocument；输出 ParsedChunk（无 Embedding/向量概念）。
package chunking

import (
	"context"

	"github.com/1090-f/Memora/internal/service/rag/canonical"
	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// ChunkOptions 是分块配置（进入 chunk_config_hash，不进入 parse_config_hash）。
type ChunkOptions struct {
	// MaxTokens 是单个 Chunk 的 token 上限。
	MaxTokens int
	// MinTokens 是低于该值的相邻 Chunk 可合并阈值。
	MinTokens int
	// OverlapTokens 是长文本内部拆分时的重叠 token 数。
	OverlapTokens int
	// RepeatTableHead 是大表按行拆分时子 Chunk 是否重复表头。
	RepeatTableHead bool
	// StrategyVersion 是分块策略版本描述。
	StrategyVersion string
}

// DefaultChunkOptions 返回保守默认值。
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		MaxTokens:       1000,
		MinTokens:       100,
		OverlapTokens:   100,
		RepeatTableHead: true,
		StrategyVersion: "structure-v1",
	}
}

// ParsedChunk 是分块产物；与 entity.DocumentChunk 的转换在 pipeline 完成。
type ParsedChunk struct {
	Content         string
	HeadingPath     []string
	SourceLocation  parser.SourceLocation
	ContentTypes    []string
	BlockIDs        []string
	TableRefs       []string
	AssetRefs       []string
	SourceSpans     []canonical.SourceSpan
	Strategy        string
	StrategyVersion string
	TokenCount      int
}

// Tokenizer 是分块/计数共用的 token 计量接口。
// 必须与 Embedding 模型对齐；Embedding 模型改变时重新选择 tokenizer 并重新 Chunk。
type Tokenizer interface {
	// Name 返回 tokenizer 身份（参与 chunk_config_hash）。
	Name() string
	// Count 计算文本 token 数。
	Count(text string) (int, error)
	// Split 将超长文本按 token 预算拆分（带 overlap）；返回的片段合计覆盖原文本。
	Split(text string, maxTokens, overlapTokens int) ([]string, error)
}

// Chunker 将 ParsedDocument 分块。
type Chunker interface {
	Chunk(ctx context.Context, doc *parser.ParsedDocument, opts ChunkOptions) ([]ParsedChunk, error)
}

// CanonicalChunker consumes the stable canonical contract. Implementations must
// use typed nodes and references rather than reparsing the Markdown view.
type CanonicalChunker interface {
	ChunkCanonical(ctx context.Context, doc *canonical.CanonicalDocument, opts ChunkOptions) ([]ParsedChunk, error)
}
