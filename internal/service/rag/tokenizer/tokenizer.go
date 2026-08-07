// Package tokenizer 提供中文/英文混合的确定性分词内核。
// 分词内核是普通 Go 接口，可替换；对 Eino 编排暴露 Transformer（见 transformer 包）。
// 选型说明：不引入 gse（词典约 30MB，二进制与内存开销大）/gojieba（cgo 依赖），
// 采用确定性 N-gram + 英文空格切分，满足 PostgreSQL simple 配置检索需求。
package tokenizer

import (
	"strings"
	"unicode"
)

// Tokenizer 是可替换的分词内核接口。
type Tokenizer interface {
	// Tokenize 将文本切分为规范化的 Token 列表（去重、去空）。
	Tokenize(text string) []string
}

// NgramConfig 配置 N-gram 分词。
type NgramConfig struct {
	// MinGram / MaxGram 是中文 N-gram 范围。
	MinGram int
	MaxGram int
}

// DefaultNgramConfig 返回保守默认值（一元与二元，覆盖中文检索常见需求）。
func DefaultNgramConfig() NgramConfig {
	return NgramConfig{MinGram: 1, MaxGram: 2}
}

// NgramTokenizer 是确定性 N-gram 分词器：
//   - 中文（CJK 与扩展区）按 N-gram 切分；
//   - 英文/数字按连续字母数字串切分并转小写；
//   - 标点与空白作为分隔符；
//   - 单字符英文/数字 Token 被过滤（避免噪声），中文单字保留。
type NgramTokenizer struct {
	cfg NgramConfig
}

// NewNgramTokenizer 构造 N-gram 分词器。
func NewNgramTokenizer(cfg NgramConfig) *NgramTokenizer {
	if cfg.MinGram <= 0 {
		cfg.MinGram = 1
	}
	if cfg.MaxGram < cfg.MinGram {
		cfg.MaxGram = cfg.MinGram
	}
	return &NgramTokenizer{cfg: cfg}
}

// Tokenize 实现 Tokenizer 接口。
func (t *NgramTokenizer) Tokenize(text string) []string {
	var tokens []string
	// 全局去重：重复 Token 只保留首次出现，避免重复词拉高检索权重。
	seen := make(map[string]struct{})

	segments := splitSegments(text)
	for _, segment := range segments {
		var segTokens []string
		if hasCJK(segment) {
			// 中英混合段：连续字母数字子串保留为整体，中文子串走 N-gram。
			segTokens = t.mixedSegment(segment)
		} else {
			segTokens = []string{strings.ToLower(segment)}
		}
		for _, token := range segTokens {
			if token == "" {
				continue
			}
			if len([]rune(token)) == 1 && !isCJKChar(token) {
				continue
			}
			if _, dup := seen[token]; dup {
				continue
			}
			seen[token] = struct{}{}
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// mixedSegment 处理中英混合段：字母数字子串保持整体，中文子串生成 N-gram。
func (t *NgramTokenizer) mixedSegment(segment string) []string {
	var out []string
	var latin strings.Builder
	flushLatin := func() {
		if latin.Len() > 0 {
			out = append(out, strings.ToLower(latin.String()))
			latin.Reset()
		}
	}
	var han strings.Builder
	flushHan := func() {
		if han.Len() > 0 {
			out = append(out, t.ngram(han.String())...)
			han.Reset()
		}
	}
	for _, r := range segment {
		if unicode.Is(unicode.Han, r) {
			flushLatin()
			han.WriteRune(r)
		} else {
			flushHan()
			latin.WriteRune(r)
		}
	}
	flushLatin()
	flushHan()
	return out
}

// ngram 生成中文 N-gram Token。
func (t *NgramTokenizer) ngram(segment string) []string {
	runes := []rune(segment)
	var out []string
	for n := t.cfg.MinGram; n <= t.cfg.MaxGram && n <= len(runes); n++ {
		for i := 0; i+n <= len(runes); i++ {
			out = append(out, string(runes[i:i+n]))
		}
	}
	return out
}

// splitSegments 将文本按非字母数字非中文的字符切分为连续段。
func splitSegments(text string) []string {
	var segments []string
	var builder strings.Builder
	flush := func() {
		if builder.Len() > 0 {
			segments = append(segments, builder.String())
			builder.Reset()
		}
	}
	for _, r := range text {
		if isTokenChar(r) {
			builder.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return segments
}

// isTokenChar 判断字符是否属于 Token 字符（中文或字母数字）。
func isTokenChar(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// hasCJK 判断字符串是否包含中文。
func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// isCJKChar 判断字符串是否全部为中文。
func isCJKChar(s string) bool {
	runes := []rune(s)
	if len(runes) != 1 {
		return false
	}
	return unicode.Is(unicode.Han, runes[0])
}
