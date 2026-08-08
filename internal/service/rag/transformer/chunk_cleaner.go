package transformer

import (
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/chunking"
)

// ChunkCleaner 在分块后、Embedding 前执行：
//   - trim 与最终空白规范；
//   - 移除分块序列化产生的重复分隔符；
//   - 检查 Chunk 内容非空；
//   - 保留标题、表头、caption 等结构上下文；
//   - 不改变 Chunk 边界，不做 token-aware split，不删除 source location 与引用。
type ChunkCleaner struct{}

// NewChunkCleaner 构造分块后清理器。
func NewChunkCleaner() *ChunkCleaner { return &ChunkCleaner{} }

// Clean 就地清理 Chunk 列表，返回清理后的列表（数量不变）。
// 清理后 Chunk 必须仍然不超过 maxTokens；超过说明 Chunker 有 bug，
// 直接返回错误而不是补救性切割。
func (c *ChunkCleaner) Clean(chunks []chunking.ParsedChunk, maxTokens int, count func(string) (int, error)) ([]chunking.ParsedChunk, error) {
	for i := range chunks {
		chunk := &chunks[i]
		cleaned, err := cleanChunkContent(chunk.Content)
		if err != nil {
			return nil, err
		}
		chunk.Content = cleaned
		if chunk.Content == "" {
			return nil, fmt.Errorf("Chunk %d 清理后内容为空（Chunker 输出空 Chunk）", i)
		}
		if maxTokens > 0 {
			tokens, err := count(chunk.Content)
			if err != nil {
				return nil, fmt.Errorf("统计 Chunk %d token 数失败: %w", i, err)
			}
			chunk.TokenCount = tokens
			if tokens > maxTokens {
				return nil, fmt.Errorf("Chunk %d 清理后 %d tokens 超过上限 %d（Chunker bug，禁止补救性切割）", i, tokens, maxTokens)
			}
		}
	}
	return chunks, nil
}

// cleanChunkContent 清理单个 Chunk 文本。
func cleanChunkContent(content string) (string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	// 移除零宽字符。
	content = strings.NewReplacer("\u200b", "", "\ufeff", "").Replace(content)
	lines := strings.Split(content, "\n")
	var builder strings.Builder
	blankRun := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			blankRun++
			if blankRun > 1 {
				continue
			}
			builder.WriteString("\n")
			continue
		}
		blankRun = 0
		builder.WriteString(strings.TrimRight(line, " \t"))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String()), nil
}
