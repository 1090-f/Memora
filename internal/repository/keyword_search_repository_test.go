package repository

import "testing"

func TestParadeDBKeywordOperator(t *testing.T) {
	tests := []struct {
		mode KeywordSearchMode
		want string
	}{
		{mode: KeywordSearchExact, want: "###"},
		{mode: KeywordSearchAll, want: "&&&"},
		{mode: KeywordSearchAny, want: "|||"},
		{mode: KeywordSearchMode("invalid"), want: "|||"},
	}
	for _, tt := range tests {
		if got := paradeDBKeywordOperator(tt.mode); got != tt.want {
			t.Fatalf("mode %q: got %q, want %q", tt.mode, got, tt.want)
		}
	}
}
