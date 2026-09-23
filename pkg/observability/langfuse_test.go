package observability

import (
	"encoding/base64"
	"testing"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestLangfuseAttributesDoNotLeakIntoTraceSpans 把「langfuse.* 天然隔离、gen_ai.* 正文会落库」
// 这个隐私边界固化成测试，将来有人动白名单会立刻暴露。
func TestLangfuseAttributesDoNotLeakIntoTraceSpans(t *testing.T) {
	// Langfuse 专属属性必须被过滤，不落库
	for _, key := range []string{
		"langfuse.observation.input",
		"langfuse.observation.output",
		"langfuse.trace.name",
		"langfuse.session.id",
		"langfuse.user.id",
		"langfuse.trace.metadata.run_id",
	} {
		if safeAttributeKey(key) {
			t.Fatalf("%s 属性不应落库", key)
		}
	}

	// 「官方推荐的 gen_ai.prompt / gen_ai.completion 只有一半安全」这个危险事实必须锁死：
	// prompt 被 deny 拦下，completion / input.messages 却照样落库。
	if safeAttributeKey("gen_ai.prompt") {
		t.Fatal("gen_ai.prompt 应被 deny 子串拦下")
	}
	if !safeAttributeKey("gen_ai.completion") {
		t.Fatal("白名单行为已变化：gen_ai.completion 不再落库，请复核方案 4.6 的结论")
	}
	if !safeAttributeKey("gen_ai.input.messages") {
		t.Fatal("白名单行为已变化：gen_ai.input.messages 不再落库，请复核方案 4.6 的结论")
	}
	// 纯数值用量应保留（这是我们要的）
	if !safeAttributeKey("gen_ai.usage.input_tokens") {
		t.Fatal("gen_ai.usage.input_tokens 应落库（纯数值，无害）")
	}
}

func TestBasicAuth(t *testing.T) {
	got := basicAuth("pk-lf-xxx", "sk-lf-xxx")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-lf-xxx:sk-lf-xxx"))
	if got != want {
		t.Fatalf("basicAuth = %q, want %q", got, want)
	}
}

// TestLangfuseSpanFilterKeepsOnlyAgentTraces 验证只有 Agent 执行链路被放行，
// db.*、HTTP、探活等噪声一律丢弃。这是"Langfuse 里一条 trace = 一次 Agent run"的前提。
func TestLangfuseSpanFilterKeepsOnlyAgentTraces(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	processor := newLangfuseSpanFilter(recorder)

	// 应当被丢弃的噪声
	for _, name := range []string{
		"db.query", // GORM tracing 的洪泛来源
		"db.update",
		"GET /api/v1/agent/runs/:id",  // 前端轮询
		"GET /api/v1/knowledge-bases", // 普通接口
		"GET /health/ready",           // 探活
		"POST /api/v1/agent/runs",     // HTTP 触发（只保留 Agent 执行链本身）
	} {
		processor.OnEnd(stubSpan(name, false).Snapshot())
	}
	if n := len(recorder.Ended()); n != 0 {
		t.Fatalf("噪声 Span 应全部被过滤，实际 Ended=%d", n)
	}

	// 应当被放行的 Agent 执行链路
	kept := []string{
		"queue.wait",
		"agent.run",
		"context.build",
		"agent.route",
		"agent.react",
		"agent.plan_execute",
		"tool.knowledge_search",
		"tool.stream.open.document_read",
		"chat gpt-4o-mini",
	}
	for _, name := range kept {
		processor.OnEnd(stubSpan(name, true).Snapshot())
	}
	if n := len(recorder.Ended()); n != len(kept) {
		t.Fatalf("Agent 链路 Span 应全部放行，期望 %d 实际 %d", len(kept), n)
	}
}

func stubSpan(name string, hasParent bool) *tracetest.SpanStub {
	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	parentID, _ := trace.SpanIDFromHex("fedcba9876543210")

	parent := trace.SpanContext{}
	if hasParent {
		parent = trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: parentID})
	}
	return &tracetest.SpanStub{
		Name:        name,
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID}),
		Parent:      parent,
	}
}
