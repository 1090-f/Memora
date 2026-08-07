package transformer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// EnrichConfig 定义 ChunkEnricher 的配置。
type EnrichConfig struct {
	// ChunkConfig 是分段配置的稳定序列化描述，用于计算 chunk_config_hash。
	ChunkConfig string
}

// ChunkEnricher 对分段后的 Chunk 补充稳定元数据：
// chunk_no、char_count、token_count、heading_path、context_title、source_location、chunk_config_hash。
// 空白片段被过滤。
type ChunkEnricher struct {
	chunkConfigHash string
}

// NewChunkEnricher 构造 Enricher，根据配置计算稳定 chunk_config_hash。
func NewChunkEnricher(cfg EnrichConfig) *ChunkEnricher {
	sum := sha256.Sum256([]byte(cfg.ChunkConfig))
	return &ChunkEnricher{chunkConfigHash: hex.EncodeToString(sum[:])}
}

// Transform 实现 Eino document.Transformer。
func (e *ChunkEnricher) Transform(ctx context.Context, docs []*schema.Document, _ ...document.TransformerOption) ([]*schema.Document, error) {
	ctx = callbacks.EnsureRunInfo(ctx, e.GetType(), components.ComponentOfTransformer)
	ctx = callbacks.OnStart(ctx, &document.TransformerCallbackInput{Input: docs})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	out := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		// 去首尾空白；全空白片段无检索价值，直接过滤且不占 chunk_no。
		content := strings.TrimSpace(doc.Content)
		if content == "" {
			continue
		}
		doc.Content = content
		// chunk_no 使用过滤后的连续序号（与 ID 保持一致，避免空洞）。
		chunkNo := len(out)
		doc.ID = fmt.Sprintf("%s#chunk-%d", einoadapter.GetMetaString(doc.MetaData, einoadapter.MetaDocumentID), chunkNo)
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]any)
		}
		einoadapter.SetMetaInt(doc, einoadapter.MetaChunkNo, chunkNo)
		// 按 rune 统计字符数，中文不再按多字节放大计数。
		einoadapter.SetMetaInt(doc, einoadapter.MetaCharCount, utf8.RuneCountInString(content))
		einoadapter.SetMetaInt(doc, einoadapter.MetaTokenCount, estimateTokens(content))
		einoadapter.SetMetaString(doc, einoadapter.MetaChunkConfigHash, e.chunkConfigHash)
		// context_title 取 heading_path 的最后一级或文档标题。
		headingPath := headingPathOf(doc)
		if len(headingPath) > 0 {
			einoadapter.SetMetaString(doc, einoadapter.MetaContextTitle, headingPath[len(headingPath)-1])
		} else if title := einoadapter.GetMetaString(doc.MetaData, einoadapter.MetaDocumentTitle); title != "" {
			einoadapter.SetMetaString(doc, einoadapter.MetaContextTitle, title)
		}
		out = append(out, doc)
	}
	_ = callbacks.OnEnd(ctx, &document.TransformerCallbackOutput{Output: out})
	return out, nil
}

// GetType 返回组件类型名。
func (e *ChunkEnricher) GetType() string { return "ChunkEnricher" }

// IsCallbacksEnabled 启用 Eino Callbacks。
func (e *ChunkEnricher) IsCallbacksEnabled() bool { return true }

// ChunkConfigHash 返回当前配置哈希。
func (e *ChunkEnricher) ChunkConfigHash() string { return e.chunkConfigHash }

// headingPathOf 从 metadata 读取 heading_path（可能为 []string 或 jsonb 字节）。
func headingPathOf(doc *schema.Document) []string {
	// heading_path 可能来自 Eino 元数据（[]string）或 JSON 序列化结果（string/[]byte）。
	value := einoadapter.GetMetaAny(doc.MetaData, einoadapter.MetaHeadingPath)
	switch typed := value.(type) {
	case []string:
		return typed
	case string:
		var out []string
		if err := json.Unmarshal([]byte(typed), &out); err == nil {
			return out
		}
	case []byte:
		var out []string
		if err := json.Unmarshal(typed, &out); err == nil {
			return out
		}
	}
	return nil
}

// estimateTokens 以保守的字符/4 估算 Token 数（中英文混合保守值）。
func estimateTokens(content string) int {
	count := 0
	// 保守估算：ASCII 按 4 字符记 1 Token，中文等非 ASCII 按 1 字符记 1 Token。
	for _, r := range content {
		if r >= 0x4E00 && r <= 0x9FFF {
			count++
		} else if r > 0x7F {
			count++
		} else {
			count += 4
		}
	}
	if count == 0 {
		return 0
	}
	return (count + 3) / 4
}
