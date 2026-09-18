package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakeToolCallingModel 是 ToolCallingChatModel 的最小假实现，行为用函数字段注入。
type fakeToolCallingModel struct {
	generate func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
	stream   func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error)
}

func (f *fakeToolCallingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if f.generate != nil {
		return f.generate(ctx, input, opts...)
	}
	return &schema.Message{
		Content:      "hello",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5}},
	}, nil
}

func (f *fakeToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if f.stream != nil {
		return f.stream(ctx, input, opts...)
	}
	return schema.StreamReaderFromArray([]*schema.Message{{Content: "hello"}}), nil
}

func (f *fakeToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return f, nil
}

// withRecorder 安装带 SpanRecorder 的全局 TracerProvider，测试结束后恢复。
func withRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(old)
		_ = tp.Shutdown(context.Background())
	})
	return rec
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

// TestWrapChatModelWithTracingDisabledReturnsOriginal 覆盖「关闭态原样返回、零开销」这条最易被误改的路径。
func TestWrapChatModelWithTracingDisabledReturnsOriginal(t *testing.T) {
	if _, err := config.Load("../../configs/config.yaml.example"); err != nil {
		t.Skipf("无法加载示例配置: %v", err)
	}
	inner := &fakeToolCallingModel{}
	got := WrapChatModelWithTracing(inner, "gpt-4o-mini")
	if got != model.ToolCallingChatModel(inner) {
		t.Fatalf("关闭态应原样返回，实际 %T", got)
	}
}

// TestTracedChatModelWithToolsPreservesWrapper 覆盖「WithTools 返回的实例仍是包装体」这条最易漏的点。
func TestTracedChatModelWithToolsPreservesWrapper(t *testing.T) {
	tc := &tracedChatModel{inner: &fakeToolCallingModel{}, modelName: "gpt-4o-mini"}
	bound, err := tc.WithTools(nil)
	if err != nil {
		t.Fatalf("WithTools 报错: %v", err)
	}
	if _, ok := bound.(*tracedChatModel); !ok {
		t.Fatalf("WithTools 应返回 *tracedChatModel，实际 %T", bound)
	}
}

// TestTracedChatModelGenerateSpanLifecycle 覆盖 Generate 正常返回与报错两条路径的 Span 闭合。
func TestTracedChatModelGenerateSpanLifecycle(t *testing.T) {
	t.Run("正常返回", func(t *testing.T) {
		rec := withRecorder(t)
		tc := &tracedChatModel{inner: &fakeToolCallingModel{}, modelName: "gpt-4o-mini", captureContent: true}
		_, err := tc.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
		if err != nil {
			t.Fatalf("Generate 报错: %v", err)
		}
		spans := rec.Ended()
		if len(spans) != 1 {
			t.Fatalf("期望 1 个已闭合 Span，实际 %d", len(spans))
		}
		attrs := spanAttrMap(spans[0])
		if attrs["gen_ai.request.model"] != "gpt-4o-mini" {
			t.Fatalf("缺少模型名属性: %v", attrs)
		}
		if attrs["langfuse.observation.type"] != "generation" {
			t.Fatalf("缺少 generation 类型声明: %v", attrs)
		}
		if attrs["gen_ai.usage.input_tokens"] != int64(10) {
			t.Fatalf("缺少输入 token 用量: %v", attrs)
		}
		if attrs["langfuse.observation.output"] == "" {
			t.Fatalf("capture_content=true 时应写 output: %v", attrs)
		}
	})

	t.Run("报错闭合", func(t *testing.T) {
		rec := withRecorder(t)
		inner := &fakeToolCallingModel{generate: func(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
			return nil, errors.New("boom")
		}}
		tc := &tracedChatModel{inner: inner, modelName: "gpt-4o-mini"}
		if _, err := tc.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}}); err == nil {
			t.Fatal("Generate 应返回错误")
		}
		if len(rec.Ended()) != 1 {
			t.Fatalf("报错路径 Span 也应闭合，实际 %d", len(rec.Ended()))
		}
	})
}

// TestTracedChatModelStreamClosesSpan 覆盖 Stream 读完后的 Span 闭合与 output/usage 写入。
func TestTracedChatModelStreamClosesSpan(t *testing.T) {
	rec := withRecorder(t)
	inner := &fakeToolCallingModel{stream: func(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
		return schema.StreamReaderFromArray([]*schema.Message{
			{Content: "hel"},
			{Content: "lo", ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 3, CompletionTokens: 4}}},
		}), nil
	}}
	tc := &tracedChatModel{inner: inner, modelName: "gpt-4o-mini", captureContent: true}
	stream, err := tc.Stream(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	if err != nil {
		t.Fatalf("Stream 报错: %v", err)
	}
	// 读原始流到 EOF，触发旁路 goroutine 累积输出并 End Span
	for {
		if _, recvErr := stream.Recv(); recvErr != nil {
			break
		}
	}
	waitFor(t, time.Second, func() bool { return len(rec.Ended()) == 1 })

	attrs := spanAttrMap(rec.Ended()[0])
	if attrs["langfuse.observation.output"] != "hello" {
		t.Fatalf("Stream output 应累积为 hello，实际 %v", attrs["langfuse.observation.output"])
	}
	if attrs["gen_ai.usage.output_tokens"] != int64(4) {
		t.Fatalf("Stream 应写入 output token 用量，实际 %v", attrs)
	}
}

func spanAttrMap(s sdktrace.ReadOnlySpan) map[string]any {
	m := make(map[string]any)
	for _, kv := range s.Attributes() {
		m[string(kv.Key)] = kv.Value.AsInterface()
	}
	return m
}
