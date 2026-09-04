package repository

import (
	"context"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/testutil"
	"gorm.io/datatypes"
)

func TestTraceSpanRepositoryRoundTrip(t *testing.T) {
	db := testutil.OpenRAGTestDB(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	traceID := "0123456789abcdef0123456789abcdef"
	parentID := "fedcba9876543210"
	spans := []entity.TraceSpan{
		{TraceID: traceID, SpanID: parentID, Name: "POST /api/v1/agent/runs", Kind: "server", StatusCode: "Ok", StartedAt: now, EndedAt: now.Add(time.Second), DurationMS: 1000, Attributes: datatypes.JSON(`{"memora.run_id":"run-1"}`), Events: datatypes.JSON(`[]`), CreatedAt: now},
		{TraceID: traceID, SpanID: "0123456789abcdef", ParentSpanID: &parentID, Name: "agent.run", Kind: "consumer", StatusCode: "Ok", StartedAt: now.Add(100 * time.Millisecond), EndedAt: now.Add(900 * time.Millisecond), DurationMS: 800, Attributes: datatypes.JSON(`{"memora.run_id":"run-1"}`), Events: datatypes.JSON(`[]`), CreatedAt: now},
		{TraceID: traceID, SpanID: "1111111111111111", Name: "other-user-root", Kind: "server", StatusCode: "Ok", StartedAt: now, EndedAt: now.Add(time.Second), DurationMS: 1000, Attributes: datatypes.JSON(`{"memora.run_id":"run-2"}`), Events: datatypes.JSON(`[]`), CreatedAt: now},
	}
	if err := db.WithContext(context.Background()).Create(&spans).Error; err != nil {
		t.Fatalf("create spans: %v", err)
	}
	got, err := NewTraceSpanRepository(db).ListByRunTrace(context.Background(), traceID, "run-1")
	if err != nil {
		t.Fatalf("list spans: %v", err)
	}
	if len(got) != 2 || got[0].SpanID != parentID || got[1].ParentSpanID == nil || *got[1].ParentSpanID != parentID {
		t.Fatalf("unexpected spans: %#v", got)
	}
}
