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
	return splitWithCounter(text, maxTokens, overlapTokens, t.Count)
}

type countTokensFunc func(string) (int, error)

// splitWithCounter 使用调用方提供的模型计数器切分文本，保证边界策略在
// heuristic 与模型 tokenizer 之间保持一致。
func splitWithCounter(text string, maxTokens, overlapTokens int, count countTokensFunc) ([]string, error) {
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
	total, err := count(trimmed)
	if err != nil {
		return nil, err
	}
	if total <= maxTokens {
		return []string{trimmed}, nil
	}

	units := splitUnits(trimmed)
	var chunks []string
	var current strings.Builder
	var overlapTail string
	flush := func() {
		content := strings.TrimSpace(current.String())
		if content != "" {
			chunks = append(chunks, content)
		}
		current.Reset()
	}

	for _, unit := range units {
		unitTokens, err := count(unit)
		if err != nil {
			return nil, err
		}
		if unitTokens > maxTokens {
			// 单元自身超限：先 flush 当前，再对单元内部做 token 级硬拆。
			flush()
			pieces, err := hardSplitWithCounter(unit, maxTokens, count)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, pieces...)
			overlapTail = ""
			continue
		}
		candidate := unit
		if current.Len() > 0 {
			candidate = current.String() + "\n" + unit
		}
		candidateTokens, err := count(candidate)
		if err != nil {
			return nil, err
		}
		if current.Len() > 0 && candidateTokens > maxTokens {
			tail := overlapTail
			flush()
			// overlap 尾巴进入下一片开头。
			if overlapTokens > 0 && tail != "" {
				current.WriteString(tail)
			}
			overlapTail = ""
		}
		// overlap 加上完整单元仍超限时，优先保留单元并放弃 overlap。
		if current.Len() > 0 {
			candidate = current.String() + "\n" + unit
			candidateTokens, err = count(candidate)
			if err != nil {
				return nil, err
			}
			if candidateTokens > maxTokens {
				current.Reset()
			}
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(unit)
		overlapTail, err = overlapSuffixWithCounter(current.String(), overlapTokens, count)
		if err != nil {
			return nil, err
		}
	}
	flush()
	return chunks, nil
}

// hardSplitWithCounter 在单元内部按 token 预算和 Unicode 字符边界拆分。
func hardSplitWithCounter(text string, maxTokens int, count countTokensFunc) ([]string, error) {
	runes := []rune(text)
	var pieces []string
	for start := 0; start < len(runes); {
		lo, hi := start+1, len(runes)
		best := start
		for lo <= hi {
			mid := lo + (hi-lo)/2
			tokens, err := count(string(runes[start:mid]))
			if err != nil {
				return nil, err
			}
			if tokens <= maxTokens {
				best = mid
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		if best == start {
			required, err := count(string(runes[start : start+1]))
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("单个 Unicode 字符需要 %d tokens，超过当前预算 %d", required, maxTokens)
		}
		piece := strings.TrimSpace(string(runes[start:best]))
		if piece != "" {
			pieces = append(pieces, piece)
		}
		start = best
	}
	return pieces, nil
}

// overlapSuffixWithCounter 返回文本结尾不超过 overlapTokens 的最长字符片段。
func overlapSuffixWithCounter(text string, overlapTokens int, count countTokensFunc) (string, error) {
	if overlapTokens <= 0 {
		return "", nil
	}
	runes := []rune(text)
	lo, hi := 0, len(runes)
	start := len(runes)
	for lo <= hi {
		mid := lo + (hi-lo)/2
		tokens, err := count(string(runes[mid:]))
		if err != nil {
			return "", err
		}
		if tokens <= overlapTokens {
			start = mid
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}
	if start == 0 || start >= len(runes) {
		return "", nil
	}
	suffix := strings.TrimSpace(string(runes[start:]))
	if strings.TrimSpace(string(runes)) == suffix {
		return "", nil
	}
	return suffix, nil
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
