package observability

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1090-f/Memora/pkg/config"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// 这组测试锁的是「Langfuse 上每个节点都得有 Input/Output」这条性质的两端约束：
//   ① 开关打开 → input/output 必须落到正确的属性名上（写错名字 UI 就是空白）；
//   ② 开关关闭 → 正文一概不外发，但 observation.type 这类非正文属性照写。
// 埋点散在 worker / adkcore / ai 三个包里，一旦有人绕过本文件的 helper 自己拼属性名，
// 这里不会报警 —— 所以新增埋点必须走 SetLangfuseInput / SetLangfuseOutput。

func TestSetLangfuseIOHonoursCaptureFlag(t *testing.T) {
	loadLangfuseTestConfig(t, false)

	attrs := recordSpanAttrs(t, func(span trace.Span) {
		SetLangfuseInput(span, "用户问题")
		SetLangfuseOutput(span, "最终回答")
		SetLangfuseObservationType(span, "agent")
	})

	if _, ok := attrs["langfuse.observation.input"]; ok {
		t.Fatal("capture_content 关闭时不应外发 input")
	}
	if _, ok := attrs["langfuse.observation.output"]; ok {
		t.Fatal("capture_content 关闭时不应外发 output")
	}
	if attrs["langfuse.observation.type"] != "agent" {
		t.Fatalf("observation.type 不是正文，应始终写入，实际 %q", attrs["langfuse.observation.type"])
	}
}

func TestSetLangfuseIOWritesTruncatedContent(t *testing.T) {
	loadLangfuseTestConfig(t, true)

	long := strings.Repeat("中", LangfuseIOMaxRunes+500)
	attrs := recordSpanAttrs(t, func(span trace.Span) {
		SetLangfuseInput(span, "用户问题")
		SetLangfuseOutput(span, long)
	})

	if got := attrs["langfuse.observation.input"]; got != "用户问题" {
		t.Fatalf("input 未按预期写入: %q", got)
	}

	out := attrs["langfuse.observation.output"]
	const suffix = "...(已截断)"
	if !strings.HasSuffix(out, suffix) {
		t.Fatalf("超长 output 应被截断，实际结尾 %q", out[len(out)-16:])
	}
	if want := LangfuseIOMaxRunes + len([]rune(suffix)); len([]rune(out)) != want {
		t.Fatalf("截断后字符数 = %d，期望 %d", len([]rune(out)), want)
	}
	// 按字符而非字节截断的反面：中文被切在 UTF-8 字符中间会出现替换字符
	if strings.ContainsRune(out, '\uFFFD') {
		t.Fatal("截断产生了乱码，说明用了按字节切的写法")
	}
}

func TestSetLangfuseIOSkipsEmptyValue(t *testing.T) {
	loadLangfuseTestConfig(t, true)

	attrs := recordSpanAttrs(t, func(span trace.Span) {
		SetLangfuseInput(span, "")
		SetLangfuseOutput(span, "")
		SetLangfuseObservationType(span, "")
	})

	for _, key := range []string{
		"langfuse.observation.input",
		"langfuse.observation.output",
		"langfuse.observation.type",
	} {
		if _, ok := attrs[key]; ok {
			t.Fatalf("%s 在空值下不应写入", key)
		}
	}
}

// loadLangfuseTestConfig 用仓库里的示例配置初始化全局配置，并设置 capture_content。
// 复用 config.yaml.example 而不是手写配置，避免示例配置演进后这里悄悄失效。
func loadLangfuseTestConfig(t *testing.T, capture bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "configs", "config.yaml.example"))
	if err != nil {
		t.Fatalf("读取示例配置失败: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	if capture {
		t.Setenv("MEMORA_LANGFUSE_CAPTURE_CONTENT", "true")
	} else {
		t.Setenv("MEMORA_LANGFUSE_CAPTURE_CONTENT", "false")
	}
	if _, err := config.Load(path); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
}

// recordSpanAttrs 建一个内存 Span，执行 write 后返回它结束时落下的字符串属性。
func recordSpanAttrs(t *testing.T, write func(span trace.Span)) map[string]string {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	// SpanRecorder 本身就是 SpanProcessor（本仓库 OTel SDK 版本里它不是 SpanExporter）
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	_, span := provider.Tracer("langfuse-attrs-test").Start(context.Background(), "probe")
	write(span)
	span.End()

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("期望 1 条已结束 Span，实际 %d", len(ended))
	}
	attrs := make(map[string]string)
	for _, kv := range ended[0].Attributes() {
		if kv.Value.Type() == attribute.STRING {
			attrs[string(kv.Key)] = kv.Value.AsString()
		}
	}
	return attrs
}
