package chunking

import (
	"fmt"
	"strings"
)

// HeuristicTokenizer 是确定性启发式 token 计数器：
//   - 中文/非 ASCII 字符按 1 token 计；
//   - ASCII 字符按 4 字符 1 token 计（不足 4 个也保守计 1）；
//   - 显式标记为 heuristic-v1，参与 chunk_config_hash；
//   - 接入 Embedding 模型后必须替换为与模型对齐的 tokenizer。
//
// 禁止用字符数代替 token 数：本实现是明确的计量模型，
// 分块与后置 TokenCounter 使用同一实例，保证计数一致。
type HeuristicTokenizer struct{}

// NewHeuristicTokenizer 构造启发式 tokenizer。
func NewHeuristicTokenizer() *HeuristicTokenizer { return &HeuristicTokenizer{} }

// Name 返回 tokenizer 身份。
func (t *HeuristicTokenizer) Name() string { return "heuristic-v1" }

// Count 计算文本 token 数。
func (t *HeuristicTokenizer) Count(text string) (int, error) {
	return tokenCount([]rune(text)), nil
}

// tokenCount 计算 rune 序列的 token 数。
func tokenCount(runes []rune) int {
	tokens := 0
	ascii := 0
	for _, r := range runes {
		if r > 0x7F {
			tokens++
			continue
		}
		ascii++
		if ascii == 4 {
			tokens++
			ascii = 0
		}
	}
	if ascii > 0 {
		tokens++
	}
	return tokens
}

// Split 将超长文本按 token 预算拆分。
//
// 算法：
//  1. 先按段落/句子边界切分为“单元”；
//  2. 贪心打包到 maxTokens，超出的单元触发换片；
//  3. overlap 只发生在被拆分的长文本内部，不跨结构边界复制。
func (t *HeuristicTokenizer) Split(text string, maxTokens, overlapTokens int) ([]string, error) {
	if maxTokens <= 0 {
		return nil, fmt.Errorf("maxTokens 必须为正数，实际 %d", maxTokens)
	}
	if overlapTokens < 0 {
		overlapTokens = 0
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, nil
	}
	if tokenCount([]rune(trimmed)) <= maxTokens {
		return []string{trimmed}, nil
	}

	units := splitUnits(trimmed)
	var chunks []string
	var current strings.Builder
	currentTokens := 0
	var overlapTail string
	flush := func() {
		content := strings.TrimSpace(current.String())
		if content != "" {
			chunks = append(chunks, content)
		}
		current.Reset()
		currentTokens = 0
	}

	for _, unit := range units {
		unitTokens := tokenCount([]rune(unit))
		if unitTokens > maxTokens {
			// 单元自身超限：先 flush 当前，再对单元内部做 token 级硬拆。
			flush()
			chunks = append(chunks, t.hardSplit(unit, maxTokens)...)
			overlapTail = ""
			continue
		}
		if currentTokens > 0 && currentTokens+unitTokens > maxTokens {
			flush()
			// overlap 尾巴进入下一片开头。
			if overlapTokens > 0 && overlapTail != "" {
				current.WriteString(overlapTail)
				currentTokens = tokenCount([]rune(overlapTail))
			}
			overlapTail = ""
		}
		if currentTokens > 0 {
			current.WriteString("\n")
		}
		current.WriteString(unit)
		currentTokens += unitTokens
		overlapTail = t.overlapSuffix(current.String(), overlapTokens)
	}
	flush()
	return chunks, nil
}

// hardSplit 在单元内部按 token 预算硬拆（确定性，按字符边界）。
func (t *HeuristicTokenizer) hardSplit(text string, maxTokens int) []string {
	runes := []rune(text)
	var pieces []string
	start := 0
	tokens := 0
	ascii := 0
	for i, r := range runes {
		weight := 0
		if r > 0x7F {
			weight = 1
		} else {
			ascii++
			if ascii == 4 {
				weight = 1
				ascii = 0
			}
		}
		if tokens+weight > maxTokens && i > start {
			pieces = append(pieces, strings.TrimSpace(string(runes[start:i])))
			start = i
			tokens = 0
		}
		tokens += weight
	}
	if start < len(runes) {
		piece := strings.TrimSpace(string(runes[start:]))
		if piece != "" {
			pieces = append(pieces, piece)
		}
	}
	return pieces
}

// overlapSuffix 返回文本结尾约 overlapTokens 的尾巴。
func (t *HeuristicTokenizer) overlapSuffix(text string, overlapTokens int) string {
	if overlapTokens <= 0 {
		return ""
	}
	runes := []rune(text)
	tokens := 0
	ascii := 0
	start := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		weight := 0
		if runes[i] > 0x7F {
			weight = 1
		} else {
			ascii++
			if ascii == 4 {
				weight = 1
				ascii = 0
			}
		}
		if tokens+weight > overlapTokens {
			break
		}
		tokens += weight
		start = i
	}
	if start == 0 || start >= len(runes) {
		return ""
	}
	suffix := strings.TrimSpace(string(runes[start:]))
	if strings.TrimSpace(string(runes)) == suffix {
		return ""
	}
	return suffix
}

// splitUnits 按段落与句子边界切分文本。
func splitUnits(text string) []string {
	var units []string
	for _, paragraph := range strings.Split(text, "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		units = append(units, splitSentences(paragraph)...)
	}
	return units
}

// splitSentences 按句末标点切句（保留标点，避免语义割裂）。
func splitSentences(paragraph string) []string {
	runes := []rune(paragraph)
	var sentences []string
	start := 0
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '。', '！', '？', '!', '?', '；', ';', '.':
			if i+1 < len(runes) && runes[i+1] == '.' {
				continue // 省略号/小数不切分
			}
			sentences = append(sentences, strings.TrimSpace(string(runes[start:i+1])))
			start = i + 1
		}
	}
	if start < len(runes) {
		sentences = append(sentences, strings.TrimSpace(string(runes[start:])))
	}
	if len(sentences) == 0 {
		return []string{strings.TrimSpace(paragraph)}
	}
	return sentences
}
