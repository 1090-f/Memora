package service

import (
	"strings"
	"unicode"
)

// tokenCounter 实现 TokenCounter 接口，使用简单规则估算 Token 数量。
type tokenCounter struct{}

// NewTokenCounter 创建一个新的 Token 计数器。
func NewTokenCounter() *tokenCounter {
	return &tokenCounter{}
}

// Count 计算文本的 Token 数，使用简单规则估算：
// - 中文字符：1 字 ≈ 1.5 token
// - 英文单词：1 词 ≈ 1 token
// - 数字：1 个 ≈ 0.5 token
// - 标点符号：1 个 ≈ 0.5 token
func (c *tokenCounter) Count(text string) int {
	if text == "" {
		return 0
	}

	tokens := 0
	words := strings.Fields(text)

	for _, word := range words {
		// 检查是否包含中文字符
		hasChinese := false
		hasDigit := false
		hasLetter := false

		for _, r := range word {
			if unicode.Is(unicode.Han, r) {
				hasChinese = true
			} else if unicode.IsDigit(r) {
				hasDigit = true
			} else if unicode.IsLetter(r) {
				hasLetter = true
			}
		}

		if hasChinese {
			// 中文字符按字数计算，每个约 1.5 token
			chineseCount := 0
			for _, r := range word {
				if unicode.Is(unicode.Han, r) {
					chineseCount++
				}
			}
			tokens += int(float64(chineseCount) * 1.5)
		} else if hasLetter {
			// 英文单词
			tokens++
		} else if hasDigit {
			// 数字
			tokens++
		}
	}

	if tokens == 0 {
		tokens = 1
	}

	return tokens
}
