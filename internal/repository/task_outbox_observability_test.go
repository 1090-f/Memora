package repository

import (
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestMarshalOutboxPayloadInjectsTraceParent(t *testing.T) {
	previous := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(previous)

	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	payload, err := marshalOutboxPayload(trace.ContextWithSpanContext(context.Background(), spanContext), map[string]string{"task_id": "task-1"})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]string
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["traceparent"] != "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01" {
		t.Fatalf("traceparent = %q", fields["traceparent"])
	}
}
