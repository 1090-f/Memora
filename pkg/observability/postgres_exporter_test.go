package observability

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestProjectSpanKeepsStructureAndDropsSensitiveAttributes(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	parentID, _ := trace.SpanIDFromHex("fedcba9876543210")
	startedAt := time.Now().UTC().Add(-150 * time.Millisecond)
	stub := tracetest.SpanStub{
		Name: "agent.run", SpanContext: trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID}),
		Parent:   trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: parentID}),
		SpanKind: trace.SpanKindConsumer, StartTime: startedAt, EndTime: startedAt.Add(150 * time.Millisecond),
		Attributes: []attribute.KeyValue{
			attribute.String("memora.run_id", "run-1"),
			attribute.String("http.request.header.authorization", "Bearer secret"),
			attribute.String("gen_ai.prompt", "private prompt"),
		},
		Status:               sdktrace.Status{Code: codes.Error, Description: "private prompt leaked by dependency"},
		Resource:             resource.NewSchemaless(attribute.String("service.name", "memora-api")),
		InstrumentationScope: instrumentation.Scope{Name: "memora/worker"},
	}
	record, ok := projectSpan(stub.Snapshot())
	if !ok {
		t.Fatal("projectSpan returned false")
	}
	if record.traceID != traceID.String() || record.spanID != spanID.String() || record.parentSpanID == nil || *record.parentSpanID != parentID.String() {
		t.Fatalf("unexpected span identity: %#v", record)
	}
	if record.durationMS != 150 || record.serviceName == nil || *record.serviceName != "memora-api" {
		t.Fatalf("unexpected duration/service: %#v", record)
	}
	text := string(record.attributes)
	if !strings.Contains(text, "memora.run_id") || !strings.Contains(text, "run-1") {
		t.Fatalf("safe attributes missing: %s", text)
	}
	if strings.Contains(text, "authorization") || strings.Contains(text, "private prompt") {
		t.Fatalf("sensitive attributes persisted: %s", text)
	}
	if record.statusMessage != nil {
		t.Fatalf("untrusted status description persisted: %q", *record.statusMessage)
	}
}

func TestContextWithTraceParentPreservesSamplingDecision(t *testing.T) {
	ctx := ContextWithTraceParent(context.Background(), "0123456789abcdef0123456789abcdef", "0123456789abcdef", false)
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() || spanContext.IsSampled() {
		t.Fatalf("unexpected span context: %v", spanContext)
	}
}
