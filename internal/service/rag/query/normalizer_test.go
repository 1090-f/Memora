package query

import "testing"

func TestNormalize(t *testing.T) {
	if got := Normalize("  ＲＡＧ\n  Go  "); got != "rag go" {
		t.Fatalf("Normalize = %q", got)
	}
}
