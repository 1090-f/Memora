package observability

import (
	"github.com/1090-f/Memora/pkg/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// 本文件是「节点正文写入 Langfuse」的唯一入口。
//
// 背景：Langfuse 的 OTLP ingestion 只认下面这几个属性名，缺一个节点在 UI 上就是空的：
//
//	langfuse.observation.type   节点类型（generation / agent / tool / span）
//	langfuse.observation.input  节点入参
//	langfuse.observation.output 节点产出
//
// 此前这些属性只在 generation（traced_chat_model.go）与工具 Span（middleware.go）上写过，
// 结果 Langfuse 里 agent.run / agent.route / agent.react / context.build 全是没有内容的空壳。
// 统一收口在这里，新增埋点时不要再各写一份 attribute.String。
//
// 隐私约束（铁律）：正文一律走 langfuse.* 前缀，绝不能用 gen_ai.* 承载 ——
// OTLP 导出器拿到的是原始 Span，postgres_exporter.go 的属性白名单对它无效，
// 对外导出的隐私控制只能做在埋点侧。是否外发正文由 LangfuseCaptureContent() 决定。

// LangfuseIOMaxRunes 是单个节点写入 Langfuse 的正文上限（按「字符」计，不是字节）。
// 工具结果、最终回答都可能很长，不截断会让单条 Span 过肥、拖累导出与存储。
// 参考：Langfuse 对 observation 字段的限制是 LANGFUSE_OBSERVATION_FIELD_SIZE_LIMIT_BYTES（默认 2 MiB）。
const LangfuseIOMaxRunes = 8000

// LangfuseCaptureContent 报告是否允许把正文（prompt / 回答 / 工具入参出参）外发到 Langfuse。
// 与 internal/ai/traced_chat_model.go 读同一份配置，保证各埋点口径一致。
//
// ⚠️ config.Get() 在配置未初始化时 panic，而埋点是旁路代码，绝不允许把主流程带崩 ——
// 例如 internal/worker 的既有测试直接调用 executeRun、不加载配置。
// 因此这里兜底 recover 并返回 false（不导出正文）：生产路径必然已 Load 过配置，
// 该分支只在测试/异常初始化顺序下命中。
func LangfuseCaptureContent() (allowed bool) {
	defer func() {
		if recover() != nil {
			allowed = false
		}
	}()
	return config.Get().Langfuse.CaptureContent
}

// SetLangfuseInput 写入节点入参。capture_content 关闭或 value 为空时是 no-op。
func SetLangfuseInput(span trace.Span, value string) {
	setLangfuseIO(span, "langfuse.observation.input", value)
}

// SetLangfuseOutput 写入节点产出。capture_content 关闭或 value 为空时是 no-op。
func SetLangfuseOutput(span trace.Span, value string) {
	setLangfuseIO(span, "langfuse.observation.output", value)
}

// SetLangfuseObservationType 显式声明节点类型，不依赖 Langfuse 按 Span 名隐式推断。
// 合法值（v3）：generation / span / event / agent / tool / chain / retriever / embedding。
// 类型不是正文，因此**不受 capture_content 控制**。
func SetLangfuseObservationType(span trace.Span, observationType string) {
	if span == nil || observationType == "" {
		return
	}
	span.SetAttributes(attribute.String("langfuse.observation.type", observationType))
}

func setLangfuseIO(span trace.Span, key, value string) {
	if span == nil || value == "" || !LangfuseCaptureContent() {
		return
	}
	span.SetAttributes(attribute.String(key, TruncateRunesForLangfuse(value, LangfuseIOMaxRunes)))
}

// TruncateRunesForLangfuse 按「字符」而非「字节」截断。
// ⚠️ 不要用 s[:maxLen] 那种按字节切的写法 —— 正文大多是中文，会被切在 UTF-8 字符中间产生乱码。
func TruncateRunesForLangfuse(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "...(已截断)"
}
