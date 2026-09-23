package ai

import (
	"context"
	"encoding/json"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// maxInputAttributeBytes 单条 input 属性的总量预算。官方对 observation 字段的限制是
	// LANGFUSE_OBSERVATION_FIELD_SIZE_LIMIT_BYTES（默认 2 MiB），这里取远低于它的值，
	// 避免单条 Span 过肥拖累导出与存储。
	maxInputAttributeBytes = 96 * 1024
	// maxMessageChars 单条消息上限。接入目的就是「看详情」，放宽到 16k 字符，
	// 只有超长文档片段才会被截断。
	maxMessageChars = 16000
)

// genTracerName 是 generation Span 的 instrumentation 名。
// 注意：不要像某些示例那样 `var genTracer = otel.Tracer(...)` 包级缓存 ——
// 那会在包 init 时绑定当时的全局 Provider（默认 noop），而项目是运行时才 SetTracerProvider，
// 结果 generation Span 会静默失效。必须每次在函数内动态调用 otel.Tracer 获取。
const genTracerName = "github.com/1090-f/Memora/ai"

// tracedChatModel 装饰 Eino 的 ToolCallingChatModel，为每次模型调用建立 generation Span。
type tracedChatModel struct {
	inner          model.ToolCallingChatModel
	modelName      string // 真实模型名（provider_factory.go 的 config.Model）
	captureContent bool   // 是否外发 prompt / 补全正文（Wrap 时从配置固化，运行期不再读全局）
}

// WrapChatModelWithTracing 是唯一入口。Langfuse 关闭时原样返回，零开销。
func WrapChatModelWithTracing(inner model.ToolCallingChatModel, modelName string) model.ToolCallingChatModel {
	cfg := config.Get().Langfuse
	if !cfg.Enabled {
		return inner
	}
	return &tracedChatModel{inner: inner, modelName: modelName, captureContent: cfg.CaptureContent}
}

// WithTools 必须返回**包装后**的新实例，否则绑工具后的调用会丢失追踪。
func (m *tracedChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &tracedChatModel{inner: bound, modelName: m.modelName, captureContent: m.captureContent}, nil
}

func (m *tracedChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	ctx, span := m.startGenerationSpan(ctx, input)
	defer span.End() // 错误路径同样闭合，无需额外兜底逻辑

	resp, err := m.inner.Generate(ctx, input, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "chat generation failed")
		return nil, err
	}
	m.finishGenerationSpan(span, resp.Content, usageOf(resp))
	return resp, nil
}

func (m *tracedChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	ctx, span := m.startGenerationSpan(ctx, input)
	stream, err := m.inner.Stream(ctx, input, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "chat stream open failed")
		span.End()
		return nil, err
	}

	// 用 Copy(2) 拿到两个独立 reader：copies[0] 返回给调用者，copies[1] 给旁路 goroutine。
	// ⚠️ 两个易错点（Eino 语义）：
	//   ① Copy(1) 因 n<2 直接返回 []*StreamReader{sr}（原始 reader 本身），不产生副本；
	//   ② Copy 之后原始 stream 即不可用，必须返回副本之一，不能再返回原始 stream。
	//  因此这里 Copy(2) 并返回 copies[0]，goroutine 读 copies[1]。
	//   副本必须 Close —— 否则原始流无法释放，会导致整条 pipeline 的 goroutine / 内存泄漏。
	copies := stream.Copy(2)
	go func() {
		defer span.End()        // defer 逆序：先执行下一行的 Close，再 End
		defer copies[1].Close() // 流结束（含取消）后本 goroutine 必定退出并 End
		var content string
		var lastUsage *schema.TokenUsage
		for {
			msg, recvErr := copies[1].Recv()
			if recvErr != nil {
				break // io.EOF，或流被取消
			}
			if msg != nil {
				content += msg.Content
				if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
					lastUsage = msg.ResponseMeta.Usage
				}
			}
		}
		m.finishGenerationSpan(span, content, lastUsage)
	}()
	return copies[0], nil
}

// startGenerationSpan 建立 generation Span 并写入输入。
func (m *tracedChatModel) startGenerationSpan(ctx context.Context, input []*schema.Message) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(genTracerName).Start(ctx, "chat "+m.modelName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.request.model", m.modelName),
			attribute.String("langfuse.observation.type", "generation"), // 显式声明，不依赖隐式推断
		),
	)
	if m.captureContent {
		// 必须用 langfuse.* 前缀承载正文，绝不能用 gen_ai.*（见方案 4.6 铁律）
		span.SetAttributes(attribute.String("langfuse.observation.input", marshalMessagesForTracing(input)))
	}
	return ctx, span
}

// finishGenerationSpan 写入输出正文与 token 用量。
func (m *tracedChatModel) finishGenerationSpan(span trace.Span, output string, usage *schema.TokenUsage) {
	if m.captureContent {
		span.SetAttributes(attribute.String("langfuse.observation.output", truncateRunes(output, maxMessageChars)))
	}
	if usage != nil {
		span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", usage.PromptTokens),
			attribute.Int("gen_ai.usage.output_tokens", usage.CompletionTokens),
		)
	}
}

// usageOf 从响应消息中提取 token 用量。
func usageOf(msg *schema.Message) *schema.TokenUsage {
	if msg == nil || msg.ResponseMeta == nil {
		return nil
	}
	return msg.ResponseMeta.Usage
}

// marshalMessagesForTracing 把消息列表压成紧凑 JSON，供 Langfuse 展示 prompt 上下文。
// 策略：从**最新**的消息往前收集（ReAct 里最新的上下文最有诊断价值），
// 单条超长则截断，累计超出总量预算就停止追加。
func marshalMessagesForTracing(messages []*schema.Message) string {
	type entry struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	kept := make([]entry, 0, len(messages))
	total := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg == nil {
			continue
		}
		content := truncateRunes(msg.Content, maxMessageChars)
		total += len(content)
		if total > maxInputAttributeBytes && len(kept) > 0 {
			break
		}
		kept = append(kept, entry{Role: string(msg.Role), Content: content})
	}
	// 反转回原始顺序，保持可读性
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	raw, err := json.Marshal(kept)
	if err != nil {
		return ""
	}
	return string(raw)
}

// truncateRunes 按「字符」而非「字节」截断。
// ⚠️ 不要直接复用项目现有的 truncateString（middleware.go:243）—— 它用 s[:maxLen] 按字节切，
// 中文文档片段会被切在 UTF-8 字符中间产生乱码。正文大多是中文，这里必须用 rune 安全的版本。
func truncateRunes(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}
