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

var sensitiveKeyRe = regexp.MustCompile(`(?i)(secret|token|password|api[_-]?key|access[_-]?key|private[_-]?key|authorization)`)

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
	for i := len(cut) - 1; i >= 0; i-- {
		if (cut[i] & 0xC0) != 0x80 {
			if utf8.Valid(cut[:i]) {
				return string(cut[:i])
			}
		}
	}
	return string(bytes.Runes(cut))
}
