package repository

import (
	"testing"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/datatypes"
)

func TestSpansForRunExcludesAnotherRunWithSameTraceID(t *testing.T) {
	rootA, rootB := "aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb"
	spans := []entity.TraceSpan{
		{SpanID: rootA, Attributes: datatypes.JSON(`{"memora.run_id":"run-a"}`)},
		{SpanID: "aaaaaaaaaaaaaaa1", ParentSpanID: &rootA, Attributes: datatypes.JSON(`{}`)},
		{SpanID: rootB, Attributes: datatypes.JSON(`{"memora.run_id":"run-b"}`)},
		{SpanID: "bbbbbbbbbbbbbbb1", ParentSpanID: &rootB, Attributes: datatypes.JSON(`{}`)},
	}
	got := spansForRun(spans, "run-a")
	if len(got) != 2 || got[0].SpanID != rootA || got[1].SpanID != "aaaaaaaaaaaaaaa1" {
		t.Fatalf("unexpected filtered spans: %#v", got)
	}
}
