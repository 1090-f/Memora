package tools

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/1090-f/Memora/internal/contracts"
)

// sensitiveKeyRe 匹配常见的敏感字段名（密钥、口令、访问令牌等），
// 用于在记录日志前将这些字段值打码，避免泄密。
var sensitiveKeyRe = regexp.MustCompile(`(?i)(secret|token|password|api[_-]?key|access[_-]?key|private[_-]?key|authorization)`)

// sanitizeText 清洗工具输出的文本：
// 保留换行/制表/回车，过滤掉其余的控制字符，防止脏数据进入日志与响应。
func sanitizeText(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			b.WriteRune(r)
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// summarizeAndRedact 把工具的原始参数做摘要化与脱敏处理，
// 用于写日志时展示：非 JSON 内容直接截断，JSON 则递归检测灵敏度字段并打码。
func summarizeAndRedact(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return truncateUTF8ByBytes(string(raw), 800)
	}
	redactValue(v)
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return truncateUTF8ByBytes(string(b), 800)
}

// summarizeAndRedactResult 把工具执行结果压缩成一份简短摘要用于日志：
// 只记录文本、是否有结构化数据、引用数量、截断状态等信息，控制日志体积。
func summarizeAndRedactResult(result contracts.ToolResult) string {
	out := map[string]any{
		"text": truncateUTF8ByBytes(result.Text, 800),
	}
	if len(result.StructuredData) > 0 {
		out["structured_data"] = "present"
	}
	if len(result.Citations) > 0 {
		out["citations"] = len(result.Citations)
	}
	if result.Truncated {
		out["truncated"] = true
	}
	if !result.Success {
		out["error_code"] = result.ErrorCode
		out["error_message"] = truncateUTF8ByBytes(result.ErrorMessage, 200)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}

// redactValue 递归遍历 JSON 值，把所有敏感键对应的值替换为 "***"。
func redactValue(v any) {
	switch vv := v.(type) {
	case map[string]any:
		for k, val := range vv {
			if sensitiveKeyRe.MatchString(k) {
				vv[k] = "***"
				continue
			}
			redactValue(val)
		}
	case []any:
		for i := range vv {
			redactValue(vv[i])
		}
	}
}

// truncateUTF8ByBytes 按字节数截断字符串，同时保证不切断 UTF-8 多字节字符，
// 截断后也只会得到长度不超过 maxBytes 的完整字符序列。
func truncateUTF8ByBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || s == "" {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	b := []byte(s)
	if maxBytes > len(b) {
		maxBytes = len(b)
	}
	cut := b[:maxBytes]
	if utf8.Valid(cut) {
		return string(cut)
	}
	// 末尾可能被切断成一个多字节字符的一部分，向前回退到完整字符边界。
	for i := len(cut) - 1; i >= 0; i-- {
		if (cut[i] & 0xC0) != 0x80 {
			if utf8.Valid(cut[:i]) {
				return string(cut[:i])
			}
		}
	}
	return string(bytes.Runes(cut))
}
