package adkcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// AgentMiddleware 是 ADK ChatModelAgent 的自定义中间件，实现 adk.ChatModelAgentMiddleware 接口。
// 用于收集引用、发布运行时事件、控制预算。
type AgentMiddleware struct {
	// 嵌入实现（提供所有方法的空操作默认行为）
	adk.BaseChatModelAgentMiddleware

	// EventPublisher 发布 Agent 运行时事件。
	EventPublisher core.EventPublisher
	// CitationCollector 收集工具调用返回的引用。
	CitationCollector core.CitationCollector
	// RunID 当前运行的 ID。
	RunID contracts.ID
	// ToolContext 传递给工具的执行上下文。
	ToolContext contracts.ToolContext
	// Config 运行配置（预算控制）。
	Config contracts.AgentConfig
	// StartedAt 记录运行开始时间。
	StartedAt time.Time
	// Usage 中间件收集到的 token 消耗，供 Runner 取回。
	Usage contracts.TokenUsage
	// RoundNo 记录当前 ReAct 模型调用轮次。
	RoundNo int
	// roundStartTime 记录当前轮次开始时间，用于计算耗时
	roundStartTime time.Time
	// knowledgeStatus 汇总实际知识库检索工具返回的充分性状态。
	knowledgeStatus knowledgeStatusTracker
}

// Ensure AgentMiddleware implements the interface.
var _ adk.ChatModelAgentMiddleware = (*AgentMiddleware)(nil)

// BeforeAgent 在 Agent 开始执行前调用。
func (m *AgentMiddleware) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext) (context.Context, *adk.ChatModelAgentContext, error) {
	return ctx, runCtx, nil
}

// AfterAgent 在 Agent 执行完成后调用，收集 token 用量供 Runner 取回。
func (m *AgentMiddleware) AfterAgent(ctx context.Context, state *adk.ChatModelAgentState) (context.Context, error) {
	m.Usage = extractUsage(state.Messages)
	return ctx, nil
}

// BeforeModelRewriteState 在每次模型调用前触发，用于预算控制。
func (m *AgentMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if m.Config.MaxRunSeconds > 0 {
		if time.Since(m.StartedAt) > time.Duration(m.Config.MaxRunSeconds)*time.Second {
			return ctx, state, fmt.Errorf("adk agent: max run time exceeded (%d seconds)", m.Config.MaxRunSeconds)
		}
	}
	m.RoundNo++
	m.roundStartTime = time.Now()

	// 构建输入摘要
	var inputSummary string
	if m.EventPublisher != nil {
		_ = m.EventPublisher.PublishModelGenerationStarted(ctx, m.RunID)
		var msgCount int
		if state.Messages != nil {
			msgCount = len(state.Messages)
		}
		toolNames := getToolNamesFromInfos(state.ToolInfos)
		inputSummary = fmt.Sprintf("消息历史: %d 条\n可用工具: %v", msgCount, toolNames)
		_ = m.EventPublisher.PublishReactRoundStarted(ctx, m.RunID, m.RoundNo, inputSummary)
	}

	return ctx, state, nil
}

// AfterModelRewriteState 在每次模型调用后调用，发布 ReAct 轮次完成事件。
// 注意：内容增量（agent.answer.delta）不由这里发布——Runner 事件循环在
// EnableStreaming 模式下已逐 chunk 发布。此处只发布轮次元数据。
func (m *AgentMiddleware) AfterModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if m.EventPublisher == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}
	last := state.Messages[len(state.Messages)-1]

	durationMs := int64(time.Since(m.roundStartTime).Milliseconds())

	// 提取 token 用量
	var tokenUsage contracts.TokenUsage
	if last != nil && last.ResponseMeta != nil && last.ResponseMeta.Usage != nil {
		tokenUsage.InputTokens = last.ResponseMeta.Usage.PromptTokens
		tokenUsage.OutputTokens = last.ResponseMeta.Usage.CompletionTokens
		tokenUsage.TotalTokens = last.ResponseMeta.Usage.TotalTokens
	}

	// 构建模型决策摘要
	modelDecision := ""
	if last != nil && len(last.ToolCalls) > 0 {
		var decisions []string
		for _, tc := range last.ToolCalls {
			decisions = append(decisions, fmt.Sprintf("调用 %s: %s", tc.Function.Name, tc.Function.Arguments))
		}
		modelDecision = strings.Join(decisions, "; ")
	} else if last != nil && last.Content != "" {
		modelDecision = fmt.Sprintf("直接回答: %s", truncateString(last.Content, 300))
	}

	_ = m.EventPublisher.PublishReactRoundCompleted(ctx, m.RunID, m.RoundNo, len(last.ToolCalls), modelDecision, durationMs, tokenUsage)
	return ctx, state, nil
}

// WrapInvokableToolCall 包装不可流式工具调用，用于收集引用和发布事件。
func (m *AgentMiddleware) WrapInvokableToolCall(ctx context.Context, endpoint adk.InvokableToolCallEndpoint, tc *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	if tc == nil {
		return endpoint, nil
	}

	wrapped := func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		toolName := tc.Name
		callID := tc.CallID
		toolCtx, span := otel.Tracer("github.com/1090-f/Memora/agent").Start(ctx, "tool."+toolName)
		span.SetAttributes(attribute.String("memora.run_id", string(m.RunID)), attribute.String("memora.tool_name", toolName), attribute.String("memora.tool_call_id", callID), attribute.String("langfuse.observation.type", "tool"))
		if captureLangfuseContent() {
			// 工具入参写进 Langfuse —— 这是"点开一条 trace 能看到工具收到什么"的关键
			span.SetAttributes(attribute.String("langfuse.observation.input", truncateRunesForTrace(argumentsInJSON, langfuseToolIOMaxRunes)))
		}

		if m.EventPublisher != nil {
			_ = m.EventPublisher.PublishToolCallStarted(ctx, m.RunID, toolName, contracts.ID(callID))
		}

		result, err := endpoint(toolCtx, argumentsInJSON, opts...)
		m.knowledgeStatus.observe(toolName, result, err)
		success := err == nil
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "tool call failed")
		}
		if captureLangfuseContent() {
			// 工具返回结果同样写进 Langfuse（工具结果可能很长，已按字符截断）
			span.SetAttributes(attribute.String("langfuse.observation.output", truncateRunesForTrace(result, langfuseToolIOMaxRunes)))
		}
		span.End()

		if m.CitationCollector != nil {
			citations := extractCitationsFromContext(ctx)
			m.CitationCollector.Add(citations)
		}

		if m.EventPublisher != nil {
			summary := ""
			if err != nil {
				summary = err.Error()
			}
			_ = m.EventPublisher.PublishToolCallCompleted(ctx, m.RunID, contracts.ID(callID), toolName, success, summary)
		}

		return result, err
	}

	return wrapped, nil
}

// WrapStreamableToolCall 包装流式工具调用。
func (m *AgentMiddleware) WrapStreamableToolCall(ctx context.Context, endpoint adk.StreamableToolCallEndpoint, tc *adk.ToolContext) (adk.StreamableToolCallEndpoint, error) {
	if tc == nil {
		return endpoint, nil
	}

	wrapped := func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (*schema.StreamReader[string], error) {
		toolName := tc.Name
		callID := tc.CallID
		toolCtx, span := otel.Tracer("github.com/1090-f/Memora/agent").Start(ctx, "tool.stream.open."+toolName)
		span.SetAttributes(attribute.String("memora.run_id", string(m.RunID)), attribute.String("memora.tool_name", toolName), attribute.String("memora.tool_call_id", callID), attribute.String("langfuse.observation.type", "tool"))
		if captureLangfuseContent() {
			// 流式工具只记入参：输出是 StreamReader，要消费掉才能拿到结果，代价太大
			span.SetAttributes(attribute.String("langfuse.observation.input", truncateRunesForTrace(argumentsInJSON, langfuseToolIOMaxRunes)))
		}

		if m.EventPublisher != nil {
			_ = m.EventPublisher.PublishToolCallStarted(ctx, m.RunID, toolName, contracts.ID(callID))
		}

		result, err := endpoint(toolCtx, argumentsInJSON, opts...)
		success := err == nil
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "tool call failed")
		}
		span.End()

		if m.CitationCollector != nil {
			citations := extractCitationsFromContext(ctx)
			m.CitationCollector.Add(citations)
		}

		if m.EventPublisher != nil {
			summary := ""
			if err != nil {
				summary = err.Error()
			}
			_ = m.EventPublisher.PublishToolCallCompleted(ctx, m.RunID, contracts.ID(callID), toolName, success, summary)
		}

		return result, err
	}

	return wrapped, nil
}

// --- 辅助函数 ---

// extractUsage 从消息列表中提取 token 用量。
func extractUsage(messages []*schema.Message) contracts.TokenUsage {
	var usage contracts.TokenUsage
	for _, msg := range messages {
		if msg != nil && msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
			usage.InputTokens += msg.ResponseMeta.Usage.PromptTokens
			usage.OutputTokens += msg.ResponseMeta.Usage.CompletionTokens
			usage.TotalTokens += msg.ResponseMeta.Usage.TotalTokens
		}
	}
	return usage
}

// citationsContextKey 用于在 context 中传递引用。
type citationsContextKey struct{}

// WithCitations 将引用列表存入 context。
func WithCitations(ctx context.Context, citations []contracts.Citation) context.Context {
	return context.WithValue(ctx, citationsContextKey{}, citations)
}

// extractCitationsFromContext 从 context 中提取引用列表。
func extractCitationsFromContext(ctx context.Context) []contracts.Citation {
	if v, ok := ctx.Value(citationsContextKey{}).([]contracts.Citation); ok {
		return v
	}
	return nil
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// langfuseToolIOMaxRunes 是工具输入/输出写入 Langfuse 的单条字符上限。
// 工具结果可能很大（如整页文档），不截断会让 Span 过肥、拖累导出。
const langfuseToolIOMaxRunes = 8000

// captureLangfuseContent 返回是否允许把工具正文外发到 Langfuse。
// 与 internal/ai/traced_chat_model.go 读同一份配置。
func captureLangfuseContent() bool {
	return config.Get().Langfuse.CaptureContent
}

// truncateRunesForTrace 按「字符」而非「字节」截断。
// 不要复用上面的 truncateString —— 它用 s[:maxLen] 按字节切，
// 中文会被切在 UTF-8 字符中间产生乱码。
func truncateRunesForTrace(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "...(已截断)"
}

// getToolNamesFromInfos 从 ToolInfo 列表获取工具名称
func getToolNamesFromInfos(infos []*schema.ToolInfo) []string {
	if infos == nil {
		return nil
	}
	names := make([]string, 0, len(infos))
	for _, info := range infos {
		if info != nil {
			names = append(names, info.Name)
		}
	}
	return names
}
