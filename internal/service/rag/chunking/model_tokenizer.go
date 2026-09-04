package chunking

import (
	"fmt"
	"strings"
	"sync"

	tiktoken "github.com/tiktoken-go/tokenizer"
)

const cl100kTokenizerName = "tiktoken-cl100k_base-v1"

// TokenizerResolution 记录 Embedding 模型到 Tokenizer 的确定性解析结果。
// Exact=false 表示模型词表未知，调用方仍可使用显式标识的启发式回退。
type TokenizerResolution struct {
	Tokenizer Tokenizer
	Provider  string
	Model     string
	Encoding  string
	Exact     bool
	Reason    string
}

// modelTokenizer 使用 Embedding 模型对应的本地 BPE 词表计数，同时复用
// Memora 的段落/句子边界策略，不在运行时访问外部网络。
type modelTokenizer struct {
	name  string
	codec tiktoken.Codec
}

func (t *modelTokenizer) Name() string { return t.name }

func (t *modelTokenizer) Count(text string) (int, error) {
	return t.codec.Count(text)
}

func (t *modelTokenizer) Split(text string, maxTokens, overlapTokens int) ([]string, error) {
	return splitWithCounter(text, maxTokens, overlapTokens, t.Count)
}

var (
	cl100kOnce  sync.Once
	cl100kCodec tiktoken.Codec
	cl100kErr   error
)

func loadCL100K() (tiktoken.Codec, error) {
	cl100kOnce.Do(func() {
		cl100kCodec, cl100kErr = tiktoken.Get(tiktoken.Cl100kBase)
	})
	return cl100kCodec, cl100kErr
}

// ResolveEmbeddingTokenizer 根据真实模型名选择 Tokenizer。当前精确支持
// OpenAI text-embedding-3 与 text-embedding-ada-002；OpenAI-compatible
// 服务若使用自定义部署名无法可靠推断词表，因此明确回退 heuristic-v1。
func ResolveEmbeddingTokenizer(provider, model string) (TokenizerResolution, error) {
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	resolution := TokenizerResolution{Provider: normalizedProvider, Model: normalizedModel}

	switch normalizedModel {
	case "text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002":
		codec, err := loadCL100K()
		if err != nil {
			return TokenizerResolution{}, fmt.Errorf("加载 cl100k_base tokenizer 失败: %w", err)
		}
		resolution.Tokenizer = &modelTokenizer{name: cl100kTokenizerName, codec: codec}
		resolution.Encoding = string(tiktoken.Cl100kBase)
		resolution.Exact = true
		resolution.Reason = "recognized OpenAI embedding model"
		return resolution, nil
	default:
		resolution.Tokenizer = NewHeuristicTokenizer()
		resolution.Encoding = resolution.Tokenizer.Name()
		resolution.Reason = "unknown embedding tokenizer; using deterministic fallback"
		return resolution, nil
	}
}
