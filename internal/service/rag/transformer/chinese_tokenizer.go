package transformer

import (
	"context"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/1090-f/Memora/internal/service/rag/tokenizer"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// ChineseTokenizerTransformer 是中文分词 Transformer：将 Chunk 内容分词后
// 以空格分隔写入 MetaData 的 fts_tokens 键，供持久化节点写入 document_chunks.fts_tokens。
// 分词内核可替换（tokenizer.Tokenizer 接口）。
type ChineseTokenizerTransformer struct {
	core tokenizer.Tokenizer
}

// NewChineseTokenizerTransformer 构造中文分词 Transformer。
func NewChineseTokenizerTransformer(core tokenizer.Tokenizer) *ChineseTokenizerTransformer {
	if core == nil {
		core = tokenizer.NewNgramTokenizer(tokenizer.DefaultNgramConfig())
	}
	return &ChineseTokenizerTransformer{core: core}
}

// Transform 实现 Eino document.Transformer。
// 零 Token 的 Chunk（纯符号/标点内容）被过滤，不入库；其余写入 fts_tokens。
func (t *ChineseTokenizerTransformer) Transform(ctx context.Context, docs []*schema.Document, _ ...document.TransformerOption) ([]*schema.Document, error) {
	ctx = callbacks.EnsureRunInfo(ctx, t.GetType(), components.ComponentOfTransformer)
	ctx = callbacks.OnStart(ctx, &document.TransformerCallbackInput{Input: docs})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	out := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		tokens := t.core.Tokenize(doc.Content)
		if len(tokens) == 0 {
			// 纯符号/标点内容没有检索价值，过滤而非失败。
			continue
		}
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]any)
		}
		// 以空格分隔拼接，匹配 PostgreSQL simple 分词配置的检索格式。
		doc.MetaData[einoadapter.MetaFTSTokens] = strings.Join(tokens, " ")
		out = append(out, doc)
	}
	_ = callbacks.OnEnd(ctx, &document.TransformerCallbackOutput{Output: out})
	return out, nil
}

// GetType 返回组件类型名。
func (t *ChineseTokenizerTransformer) GetType() string { return "ChineseTokenizerTransformer" }

// IsCallbacksEnabled 启用 Eino Callbacks。
func (t *ChineseTokenizerTransformer) IsCallbacksEnabled() bool { return true }
