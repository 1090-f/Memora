package chunking

import (
	"strings"
	"testing"
)

func TestCountCJKAndAscii(t *testing.T) {
	tk := NewHeuristicTokenizer()
	// 4 个 ASCII 字符 = 1 token；1 个中文字符 = 1 token。
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"abcd", 1},
		{"abcdefgh", 2},
		{"中文", 2},
		{"中文abcd", 3},
	}
	for _, tc := range cases {
		got, err := tk.Count(tc.text)
		if err != nil {
			t.Fatalf("Count(%q) 失败: %v", tc.text, err)
		}
		if got != tc.want {
			t.Errorf("Count(%q) = %d，期望 %d", tc.text, got, tc.want)
		}
	}
}

func TestSplitKeepsWholeText(t *testing.T) {
	tk := NewHeuristicTokenizer()
	text := strings.Repeat("正文内容内容内容。", 200) // ~1400 tokens
	pieces, err := tk.Split(text, 200, 20)
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(pieces) < 2 {
		t.Fatalf("应拆出多片，实际 %d", len(pieces))
	}
	for _, piece := range pieces {
		tokens, _ := tk.Count(piece)
		// overlap 已包含在 maxTokens 预算内，最终分片必须严格不超限。
		if tokens > 200 {
			t.Errorf("分片 %d tokens 超过上限 200: %q", tokens, piece[:min(40, len(piece))])
		}
	}
	// 拼接内容覆盖原文本（重叠可多不少）。
	joined := strings.Join(pieces, "")
	for _, sentinel := range []string{"正文内容内容内容。"} {
		if !strings.Contains(joined, sentinel) {
			t.Errorf("拼接结果缺少原文内容 %q", sentinel)
		}
	}
}

func TestSplitShortTextReturnsSingle(t *testing.T) {
	tk := NewHeuristicTokenizer()
	pieces, err := tk.Split("短文", 1000, 0)
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(pieces) != 1 || pieces[0] != "短文" {
		t.Errorf("短文应原样返回: %v", pieces)
	}
}

func TestSplitSingleOverlongUnit(t *testing.T) {
	tk := NewHeuristicTokenizer()
	// 无标点的超长连续字符。
	text := strings.Repeat("字", 500)
	pieces, err := tk.Split(text, 100, 0)
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(pieces) < 2 {
		t.Fatalf("超长单元应硬拆，实际 %d 片", len(pieces))
	}
	total := 0
	for _, piece := range pieces {
		count, _ := tk.Count(piece)
		if count > 100 {
			t.Errorf("分片超限: %d", count)
		}
		total += count
	}
	if total < 500 {
		t.Errorf("拆分内容缺失: %d", total)
	}
}

func TestSplitRejectsBadMaxTokens(t *testing.T) {
	tk := NewHeuristicTokenizer()
	if _, err := tk.Split("abc", 0, 0); err == nil {
		t.Error("maxTokens=0 应报错")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
