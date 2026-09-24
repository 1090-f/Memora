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
	appobservability "github.com/1090-f/Memora/pkg/observability"
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
		span.SetAttributes(attribute.String("memora.run_id", string(m.RunID)), attribute.String("memora.tool_name", toolName), attribute.String("memora.tool_call_id", callID))
		appobservability.SetLangfuseObservationType(span, "tool")
		// 工具入参写进 Langfuse —— 这是"点开一条 trace 能看到工具收到什么"的关键
		appobservability.SetLangfuseInput(span, argumentsInJSON)

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
		// 工具返回结果同样写进 Langfuse（工具结果可能很长，helper 内部按字符截断）
		appobservability.SetLangfuseOutput(span, result)
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
		span.SetAttributes(attribute.String("memora.run_id", string(m.RunID)), attribute.String("memora.tool_name", toolName), attribute.String("memora.tool_call_id", callID))
		appobservability.SetLangfuseObservationType(span, "tool")
		// 工具入参写进 Langfuse —— 这是"点开一条 trace 能看到工具收到什么"的关键
		appobservability.SetLangfuseInput(span, argumentsInJSON)

		if m.EventPublisher != nil {
			_ = m.EventPublisher.PublishToolCallStarted(ctx, m.RunID, toolName, contracts.ID(callID))
		}

		result, err := endpoint(toolCtx, argumentsInJSON, opts...)
		success := err == nil
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "tool call failed")
			span.End()
		} else if result == nil {
			// 理论不可达（err 为 nil 时必然有流）；真发生了也必须闭合 Span，
			// 否则该节点在 Langfuse 里会永远停在「进行中」。
			span.End()
		} else if appobservability.LangfuseCaptureContent() {
			// 流式工具的产出是 StreamReader，Span 必须等流读完才能写 output ——
			// 此前这里直接 span.End() 且不写 output，Langfuse 上所有 tool.stream.open.* 节点都是空的。
			// 做法与 internal/ai/traced_chat_model.go 一致：Copy 出旁路副本，边读边累计，读完写 output 再 End。
			// 捕获关闭时不做 Copy，隐私模式下不产生额外开销。
			// ⚠️ Eino 语义（两个易错点）：
			//   ① Copy(1) 因 n<2 直接返回原 reader、不产生副本，必须 Copy(2)；
			//   ② Copy 之后原 reader 即不可用，只能返回副本；副本必须 Close，
			//      否则上游 pipe 阻塞，整条 pipeline 的协程与内存都会泄漏。
			copies := result.Copy(2)
			go func() {
				defer span.End()        // defer 逆序：先执行下一行的 Close，再 End
				defer copies[1].Close() // 流结束（含取消）后本 goroutine 必定退出
				var sb strings.Builder
				for {
					chunk, recvErr := copies[1].Recv()
					if recvErr != nil {
						break // io.EOF，或流被取消
					}
					// 超预算后只把流读干净、不再累积：中途 break 会让上游阻塞。
					if sb.Len() < appobservability.LangfuseIOMaxRunes*4 {
						sb.WriteString(chunk)
					}
				}
				appobservability.SetLangfuseOutput(span, sb.String())
			}()
			result = copies[0]
		} else {
			span.End()
		}

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

// 说明：Langfuse 正文写入（input / output / observation.type）与字符截断
// 统一由 pkg/observability 的 SetLangfuseInput / SetLangfuseOutput /
// SetLangfuseObservationType / TruncateRunesForLangfuse 提供，
// 本包不再保留私有副本 —— 多份实现会导致「有的节点写了、有的没写」。

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
