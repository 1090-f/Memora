package observability

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/1090-f/Memora/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// InitializeTracing 安装 W3C Trace Context 与 SDK Provider。
// Span 在 PostgreSQL 初始化后由 AttachPostgresSpanExporter 接入内置存储。
func InitializeTracing(ctx context.Context, cfg config.ObservabilityConfig, serviceName string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", serviceName))),
	}
	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	return func(shutdownCtx context.Context) error {
		return provider.Shutdown(shutdownCtx)
	}, nil
}

// AttachPostgresSpanExporter 将内置 PostgreSQL exporter 注册到当前 Provider。
// 注册发生在数据库连接建立后；Provider Shutdown 会负责刷新并关闭处理器。
func AttachPostgresSpanExporter(db *sql.DB) error {
	provider, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return fmt.Errorf("当前 OpenTelemetry Provider 不支持注册 Span Processor")
	}
	provider.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(
		NewPostgresSpanExporter(db),
		sdktrace.WithBatchTimeout(time.Second),
		sdktrace.WithMaxExportBatchSize(128),
	))
	return nil
}

// ContextWithTraceID 恢复异步任务的 Trace 归属。任务表保存 Trace ID，新的消费者 Span
// 使用一个合成远端父 Span ID，从而在进程重启和队列消费后仍归入同一条 Trace。
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return ContextWithTraceParent(ctx, traceID, "", true)
}

// ContextWithTraceParent 使用已保存的父 Span ID 续接异步调用树。
// 旧任务没有父 Span ID 时生成合成父节点，仍保持 Trace ID 关联。
func ContextWithTraceParent(ctx context.Context, traceID, parentSpanID string, sampled bool) context.Context {
	id, err := trace.TraceIDFromHex(traceID)
	if err != nil || !id.IsValid() {
		return ctx
	}
	spanID, err := trace.SpanIDFromHex(parentSpanID)
	if err != nil || !spanID.IsValid() {
		var raw [8]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return ctx
		}
		spanID = trace.SpanID(raw)
	}
	if !spanID.IsValid() {
		return ctx
	}
	flags := trace.TraceFlags(0)
	if sampled {
		flags = trace.FlagsSampled
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{TraceID: id, SpanID: spanID, TraceFlags: flags, Remote: true})
	return trace.ContextWithRemoteSpanContext(ctx, spanContext)
}
