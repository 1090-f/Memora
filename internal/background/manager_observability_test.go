package background

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestExtractStreamContextRestoresRemoteParent(t *testing.T) {
	previous := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(previous)

	ctx := extractStreamContext(context.Background(), redis.XMessage{Values: map[string]any{
		"traceparent": "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
	}})
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() || !spanContext.IsRemote() {
		t.Fatalf("invalid remote span context: %#v", spanContext)
	}
	if spanContext.TraceID().String() != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("trace id = %s", spanContext.TraceID())
	}
}
