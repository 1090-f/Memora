package database

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/gorm"
)

func TestGormTracingRecordsOnlySafeMetadata(t *testing.T) {
	previous := otel.GetTracerProvider()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	defer func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	}()

	tx := &gorm.DB{Statement: &gorm.Statement{Context: context.Background(), Table: "agent_runs"}, RowsAffected: 3}
	beginGormSpan("query")(tx)
	endGormSpan(tx)

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "db.query" {
		t.Fatalf("unexpected spans: %#v", spans)
	}
	attributes := map[string]any{}
	for _, item := range spans[0].Attributes() {
		attributes[string(item.Key)] = item.Value.AsInterface()
	}
	if attributes["db.collection.name"] != "agent_runs" || attributes["db.response.returned_rows"] != int64(3) {
		t.Fatalf("unexpected attributes: %#v", attributes)
	}
	for key := range attributes {
		if key == "db.statement" || key == "db.query.text" {
			t.Fatalf("sensitive SQL attribute must not be recorded: %s", key)
		}
	}
}
