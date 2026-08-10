package service

import (
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/tokenizer"
)

// memoryFTSHelper 提供记忆全文检索的辅助功能。
type memoryFTSHelper struct {
	tokenizer tokenizer.Tokenizer
}

// newMemoryFTSHelper 创建记忆 FTS 辅助工具。
func newMemoryFTSHelper() *memoryFTSHelper {
	return &memoryFTSHelper{
		tokenizer: tokenizer.NewNgramTokenizer(tokenizer.DefaultNgramConfig()),
	}
}

// GenerateFTSTokens 将文本分词后生成 fts_tokens 字符串。
// 返回值可直接写入 memories.fts_tokens 字段。
// 示例输入: "Go 语言是一种编程语言"
// 示例输出: "go 语言 编程 语言 编程语 言是 是一 一种 种编 编程"
func (h *memoryFTSHelper) GenerateFTSTokens(content string) string {
	tokens := h.tokenizer.Tokenize(content)
	return strings.Join(tokens, " ")
}

// 全局单例，供 MemoryExtractor / MemoryManager 使用。
var memoryFTS = newMemoryFTSHelper()

// GenerateMemoryFTSTokens 生成记忆内容的 fts_tokens。
// 在创建或更新 Memory 时调用此函数填充 FTSTokens 字段。
func GenerateMemoryFTSTokens(content string) string {
	return memoryFTS.GenerateFTSTokens(content)
}
