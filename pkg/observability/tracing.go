package observability

import (
	"context"
	"crypto/rand"

	"github.com/1090-f/Memora/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// InitializeTracing 安装 W3C Trace Context 与 SDK Provider。未配置 Collector 时仍创建真实 Span，
// 但不外发数据；配置 OTLP HTTP endpoint 后使用批量导出。
func InitializeTracing(ctx context.Context, cfg config.ObservabilityConfig, serviceName string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", serviceName))),
	}
	if cfg.OTLPEndpoint != "" {
		exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint))
		if err != nil {
			return nil, err
		}
		options = append(options, sdktrace.WithBatcher(exporter))
	}
	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	return func(shutdownCtx context.Context) error {
		return provider.Shutdown(shutdownCtx)
	}, nil
}

// ContextWithTraceID 恢复异步任务的 Trace 归属。任务表保存 Trace ID，新的消费者 Span
// 使用一个合成远端父 Span ID，从而在进程重启和队列消费后仍归入同一条 Trace。
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	id, err := trace.TraceIDFromHex(traceID)
	if err != nil || !id.IsValid() {
		return ctx
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ctx
	}
	spanID := trace.SpanID(raw)
	if !spanID.IsValid() {
		return ctx
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{TraceID: id, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: true})
	return trace.ContextWithRemoteSpanContext(ctx, spanContext)
}
