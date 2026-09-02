package core

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/google/uuid"
)

// EventPublisher 是 Core 内部使用的生命周期事件发布抽象。
type EventPublisher interface {
	Publish(ctx context.Context, event contracts.AgentEvent) error
	// PublishRunStarted 发布运行开始事件。
	// 注意：执行模式由路由器决定，开始事件发布时模式尚未确定，因此不携带模式参数。
	PublishRunStarted(ctx context.Context, runID contracts.ID) error
	PublishRunCompleted(ctx context.Context, runID contracts.ID, result contracts.AgentRunResult) error
	PublishRunFailed(ctx context.Context, runID contracts.ID, mode contracts.ExecutionMode, err error) error
	PublishRunCancelled(ctx context.Context, runID contracts.ID) error
	PublishRouterSelected(ctx context.Context, runID contracts.ID, decision contracts.RouterDecision) error
	// PublishReactRoundStarted 发布 ReAct 轮次开始事件。
	PublishReactRoundStarted(ctx context.Context, runID contracts.ID, round int, inputSummary string) error
	// PublishReactRoundCompleted 发布 ReAct 轮次完成事件。
	PublishReactRoundCompleted(ctx context.Context, runID contracts.ID, round int, toolCallCount int, modelDecision string, durationMs int64, tokenUsage contracts.TokenUsage) error
	// PublishToolCallStarted 发布工具调用开始事件。
	PublishToolCallStarted(ctx context.Context, runID contracts.ID, toolName string, callID contracts.ID) error
	// PublishToolCallCompleted 发布工具调用完成事件。
	PublishToolCallCompleted(ctx context.Context, runID contracts.ID, callID contracts.ID, toolName string, success bool, summary string) error
	// PublishModelGenerationStarted 标记本次运行第一次模型生成开始。
	PublishModelGenerationStarted(ctx context.Context, runID contracts.ID) error
	// PublishAnswerDelta 发布流式回答增量事件。
	PublishAnswerDelta(ctx context.Context, runID contracts.ID, delta string) error
	// PublishPlanCreated 发布计划创建事件。
	PublishPlanCreated(ctx context.Context, runID contracts.ID, plan *contracts.Plan, inputSummary string) error
	// PublishPlanReplanned 发布计划重新规划事件。
	PublishPlanReplanned(ctx context.Context, runID contracts.ID, plan *contracts.Plan, inputSummary string) error
	// PublishStepStarted 发布计划步骤开始事件。
	PublishStepStarted(ctx context.Context, runID contracts.ID, stepNo int, title string, inputSummary string) error
	// PublishStepCompleted 发布计划步骤完成事件。
	PublishStepCompleted(ctx context.Context, runID contracts.ID, stepNo int, title string, success bool, outputSummary string, durationMs int64, tokenUsage contracts.TokenUsage) error
}

// NoopEventPublisher 用于未接入事件存储时保持执行链路可运行。
type NoopEventPublisher struct{}

func (NoopEventPublisher) Publish(context.Context, contracts.AgentEvent) error { return nil }
func (NoopEventPublisher) PublishRunStarted(context.Context, contracts.ID) error {
	return nil
}
func (NoopEventPublisher) PublishRunCompleted(context.Context, contracts.ID, contracts.AgentRunResult) error {
	return nil
}
func (NoopEventPublisher) PublishRunFailed(context.Context, contracts.ID, contracts.ExecutionMode, error) error {
	return nil
}
func (NoopEventPublisher) PublishRunCancelled(context.Context, contracts.ID) error { return nil }
func (NoopEventPublisher) PublishRouterSelected(context.Context, contracts.ID, contracts.RouterDecision) error {
	return nil
}
func (NoopEventPublisher) PublishReactRoundStarted(context.Context, contracts.ID, int, string) error {
	return nil
}
func (NoopEventPublisher) PublishReactRoundCompleted(context.Context, contracts.ID, int, int, string, int64, contracts.TokenUsage) error {
	return nil
}
func (NoopEventPublisher) PublishToolCallStarted(context.Context, contracts.ID, string, contracts.ID) error {
	return nil
}
func (NoopEventPublisher) PublishToolCallCompleted(context.Context, contracts.ID, contracts.ID, string, bool, string) error {
	return nil
}
func (NoopEventPublisher) PublishModelGenerationStarted(context.Context, contracts.ID) error {
	return nil
}
func (NoopEventPublisher) PublishAnswerDelta(context.Context, contracts.ID, string) error { return nil }
func (NoopEventPublisher) PublishPlanCreated(context.Context, contracts.ID, *contracts.Plan, string) error {
	return nil
}
func (NoopEventPublisher) PublishPlanReplanned(context.Context, contracts.ID, *contracts.Plan, string) error {
	return nil
}
func (NoopEventPublisher) PublishStepStarted(context.Context, contracts.ID, int, string, string) error {
	return nil
}
func (NoopEventPublisher) PublishStepCompleted(context.Context, contracts.ID, int, string, bool, string, int64, contracts.TokenUsage) error {
	return nil
}

// SequencedEventPublisher 为每个 Run 分配单调递增的事件序号。
type SequencedEventPublisher struct {
	publisher               contracts.EventPublisher
	captureSensitiveContent bool
	mu                      sync.Mutex
	sequences               map[contracts.ID]int64
	timings                 map[contracts.ID]*agentRunTimingState
}

type agentRunTimingState struct {
	runStarted    time.Time
	modelStarted  time.Time
	firstTokenAt  time.Time
	modelFinished time.Time
}

func NewSequencedEventPublisher(publisher contracts.EventPublisher, captureSensitiveContent ...bool) *SequencedEventPublisher {
	capture := len(captureSensitiveContent) > 0 && captureSensitiveContent[0]
	return &SequencedEventPublisher{publisher: publisher, captureSensitiveContent: capture, sequences: make(map[contracts.ID]int64), timings: make(map[contracts.ID]*agentRunTimingState)}
}

func (p *SequencedEventPublisher) Publish(ctx context.Context, event contracts.AgentEvent) error {
	if event.TraceID == "" || event.RequestID == "" {
		traceID, requestID := contracts.CorrelationFromContext(ctx)
		if event.TraceID == "" {
			event.TraceID = traceID
		}
		if event.RequestID == "" {
			event.RequestID = requestID
		}
	}
	p.mu.Lock()
	if event.Sequence <= p.sequences[event.RunID] {
		event.Sequence = p.sequences[event.RunID] + 1
	}
	if event.Sequence == 0 {
		event.Sequence = 1
	}
	p.sequences[event.RunID] = event.Sequence
	p.mu.Unlock()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.EventID == "" {
		event.EventID = contracts.ID(uuid.NewString())
	}
	return p.publisher.Publish(ctx, event)
}

func (p *SequencedEventPublisher) publish(ctx context.Context, runID contracts.ID, typ contracts.EventType, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	stage, status := eventStage(typ)
	return p.Publish(ctx, contracts.AgentEvent{RunID: runID, EventType: typ, Stage: stage, Status: status, Data: payload})
}

func (p *SequencedEventPublisher) publishStage(ctx context.Context, runID contracts.ID, stage contracts.AgentStage, status contracts.StageStatus, observation contracts.StageObservation) error {
	payload, err := json.Marshal(observation)
	if err != nil {
		return err
	}
	return p.Publish(ctx, contracts.AgentEvent{RunID: runID, EventType: contracts.EventStageUpdated, Stage: stage, Status: status, Data: payload})
}

func eventStage(typ contracts.EventType) (contracts.AgentStage, contracts.StageStatus) {
	switch typ {
	case contracts.EventRouterCompleted:
		return contracts.AgentStageRoute, contracts.StageSucceeded
	case contracts.EventToolStarted:
		return contracts.AgentStageToolCall, contracts.StageRunning
	case contracts.EventToolCompleted:
		return contracts.AgentStageToolCall, contracts.StageSucceeded
	case contracts.EventToolCallFailed:
		return contracts.AgentStageToolCall, contracts.StageFailed
	case contracts.EventAnswerDelta:
		return contracts.AgentStageModelGenerate, contracts.StageRunning
	case contracts.EventRunCompleted:
		return contracts.AgentStageAnswer, contracts.StageSucceeded
	case contracts.EventRunFailed:
		return contracts.AgentStageAnswer, contracts.StageFailed
	case contracts.EventRunCancelled:
		return contracts.AgentStageAnswer, contracts.StageSkipped
	case contracts.EventCitationCreated:
		return contracts.AgentStageAnswer, contracts.StageSucceeded
	default:
		return "", ""
	}
}

func (p *SequencedEventPublisher) PublishRunStarted(ctx context.Context, id contracts.ID) error {
	p.mu.Lock()
	p.timings[id] = &agentRunTimingState{runStarted: time.Now().UTC()}
	p.mu.Unlock()
	return p.publish(ctx, id, contracts.EventRunStarted, map[string]any{"execution_mode": ""})
}
func (p *SequencedEventPublisher) PublishModelGenerationStarted(ctx context.Context, id contracts.ID) error {
	now := time.Now().UTC()
	p.mu.Lock()
	timing := p.timings[id]
	if timing == nil {
		timing = &agentRunTimingState{runStarted: now}
		p.timings[id] = timing
	}
	first := timing.modelStarted.IsZero()
	if first {
		timing.modelStarted = now
	}
	p.mu.Unlock()
	if !first {
		return nil
	}
	return p.publishStage(ctx, id, contracts.AgentStageModelGenerate, contracts.StageRunning, contracts.StageObservation{Stage: string(contracts.AgentStageModelGenerate), Status: contracts.StageRunning, StartedAt: &now, Summary: "模型开始生成"})
}
func (p *SequencedEventPublisher) PublishRunCompleted(ctx context.Context, id contracts.ID, result contracts.AgentRunResult) error {
	finished := time.Now().UTC()
	p.mu.Lock()
	timing := p.timings[id]
	if timing == nil {
		timing = &agentRunTimingState{runStarted: result.StartedAt}
		p.timings[id] = timing
	}
	timing.modelFinished = finished
	modelStarted := timing.modelStarted
	firstTokenAt := timing.firstTokenAt
	runStarted := timing.runStarted
	p.mu.Unlock()
	if modelStarted.IsZero() {
		modelStarted = result.StartedAt
	}
	durationMS := finished.Sub(modelStarted).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}
	if err := p.publishStage(ctx, id, contracts.AgentStageModelGenerate, contracts.StageSucceeded, contracts.StageObservation{
		Stage: string(contracts.AgentStageModelGenerate), Status: contracts.StageSucceeded, StartedAt: &modelStarted, EndedAt: &finished, DurationMS: &durationMS,
		Summary: "模型生成完成", Metadata: map[string]any{"input_tokens": result.Usage.InputTokens, "output_tokens": result.Usage.OutputTokens},
	}); err != nil {
		return err
	}
	for _, citation := range result.Citations {
		citationData := map[string]any{
			"source_type": citation.SourceType, "knowledge_base_id": citation.KnowledgeBaseID,
			"document_id": citation.DocumentID, "document_title": citation.DocumentTitle,
			"chunk_id": citation.ChunkID, "title": citation.Title, "url": citation.URL,
		}
		if p.captureSensitiveContent {
			quoted := []rune(citation.QuotedText)
			if len(quoted) > 240 {
				quoted = quoted[:240]
			}
			citationData["snippet"] = string(quoted)
		}
		if err := p.publish(ctx, id, contracts.EventCitationCreated, citationData); err != nil {
			return err
		}
	}
	completion := map[string]any{
		"answer_available":           true,
		"citation_count":             len(result.Citations),
		"knowledge_status":           result.KnowledgeStatus,
		"token_usage":                result.Usage,
		"model_generate_duration_ms": durationMS,
	}
	if !firstTokenAt.IsZero() {
		completion["first_token_at"] = firstTokenAt
		completion["first_token_latency_ms"] = firstTokenAt.Sub(runStarted).Milliseconds()
	}
	return p.publish(ctx, id, contracts.EventRunCompleted, completion)
}
func (p *SequencedEventPublisher) PublishRunFailed(ctx context.Context, id contracts.ID, mode contracts.ExecutionMode, runErr error) error {
	now := time.Now().UTC()
	p.mu.Lock()
	if timing := p.timings[id]; timing != nil {
		timing.modelFinished = now
	}
	p.mu.Unlock()
	return p.publish(ctx, id, contracts.EventRunFailed, map[string]any{
		"execution_mode":  mode,
		"error_code":      errorCode(runErr),
		"failure_stage":   contracts.AgentStageModelGenerate,
		"retryable":       true,
		"recovery_advice": "请重试；若仍失败，请检查模型服务状态并使用 Trace ID 诊断。",
	})
}
func (p *SequencedEventPublisher) PublishRunCancelled(ctx context.Context, id contracts.ID) error {
	return p.publish(ctx, id, contracts.EventRunCancelled, nil)
}
func (p *SequencedEventPublisher) PublishRouterSelected(ctx context.Context, id contracts.ID, decision contracts.RouterDecision) error {
	return p.publish(ctx, id, contracts.EventRouterCompleted, map[string]any{
		"execution_mode": decision.ExecutionMode,
		"reason_summary": decision.ReasonSummary,
		"confidence":     decision.Confidence,
		"fallback_used":  decision.FallbackUsed,
		"input_summary":  "", // 由调用方填充
	})
}

func (p *SequencedEventPublisher) PublishReactRoundStarted(ctx context.Context, id contracts.ID, round int, inputSummary string) error {
	return p.publish(ctx, id, contracts.EventReactRoundStarted, map[string]any{
		"round_no":      round,
		"round":         round,
		"input_summary": inputSummary,
	})
}

func (p *SequencedEventPublisher) PublishReactRoundCompleted(ctx context.Context, id contracts.ID, round int, toolCallCount int, modelDecision string, durationMs int64, tokenUsage contracts.TokenUsage) error {
	return p.publish(ctx, id, contracts.EventReactRoundCompleted, map[string]any{
		"round_no":        round,
		"round":           round,
		"tool_call_count": toolCallCount,
		"model_decision":  modelDecision,
		"output_summary":  modelDecision,
		"duration_ms":     durationMs,
		"token_usage":     tokenUsage,
	})
}

func (p *SequencedEventPublisher) PublishToolCallStarted(ctx context.Context, id contracts.ID, toolName string, callID contracts.ID) error {
	return p.publish(ctx, id, contracts.EventToolStarted, map[string]any{
		"tool_name":     toolName,
		"tool_call_id":  callID,
		"call_id":       callID,
		"input_summary": "",
	})
}

func (p *SequencedEventPublisher) PublishToolCallCompleted(ctx context.Context, id contracts.ID, callID contracts.ID, toolName string, success bool, summary string) error {
	eventType := contracts.EventToolCompleted
	if !success {
		eventType = contracts.EventToolCallFailed
	}
	payload := map[string]any{
		"tool_name":      toolName,
		"tool_call_id":   callID,
		"call_id":        callID,
		"success":        success,
		"summary":        summary,
		"output_summary": summary,
	}
	if !success {
		payload["error_message"] = summary
	}
	return p.publish(ctx, id, eventType, payload)
}

func (p *SequencedEventPublisher) PublishAnswerDelta(ctx context.Context, id contracts.ID, delta string) error {
	now := time.Now().UTC()
	firstLatency := int64(0)
	first := false
	if strings.TrimSpace(delta) != "" {
		p.mu.Lock()
		timing := p.timings[id]
		if timing == nil {
			timing = &agentRunTimingState{runStarted: now}
			p.timings[id] = timing
		}
		if timing.firstTokenAt.IsZero() {
			timing.firstTokenAt = now
			first = true
			firstLatency = now.Sub(timing.runStarted).Milliseconds()
		}
		p.mu.Unlock()
	}
	payload := map[string]any{"delta": delta}
	if first {
		payload["first_token_at"] = now
		payload["first_token_latency_ms"] = firstLatency
	}
	return p.publish(ctx, id, contracts.EventAnswerDelta, payload)
}

func (p *SequencedEventPublisher) AgentRunTiming(id contracts.ID) contracts.AgentRunTiming {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.timings[id]
	if state == nil {
		return contracts.AgentRunTiming{}
	}
	delete(p.timings, id)
	result := contracts.AgentRunTiming{}
	if !state.firstTokenAt.IsZero() {
		at := state.firstTokenAt
		latency := at.Sub(state.runStarted).Milliseconds()
		result.FirstTokenAt, result.FirstTokenLatencyMS = &at, &latency
	}
	if !state.modelStarted.IsZero() && !state.modelFinished.IsZero() {
		duration := state.modelFinished.Sub(state.modelStarted).Milliseconds()
		result.ModelGenerateDurationMS = &duration
	}
	return result
}

func (p *SequencedEventPublisher) PublishPlanCreated(ctx context.Context, id contracts.ID, plan *contracts.Plan, inputSummary string) error {
	steps := make([]map[string]any, len(plan.Steps))
	for i, step := range plan.Steps {
		steps[i] = map[string]any{
			"step_no":          i + 1,
			"title":            step.Title,
			"description":      step.Description,
			"recommended_tool": step.ToolName,
			"depends_on":       step.DependsOn,
			"status":           step.Status,
		}
	}
	return p.publish(ctx, id, contracts.EventPlanCreated, map[string]any{
		"version":       plan.ReplanCount + 1,
		"goal":          plan.Goal,
		"input_summary": inputSummary,
		"steps":         steps,
	})
}

func (p *SequencedEventPublisher) PublishPlanReplanned(ctx context.Context, id contracts.ID, plan *contracts.Plan, inputSummary string) error {
	steps := make([]map[string]any, len(plan.Steps))
	for i, step := range plan.Steps {
		steps[i] = map[string]any{
			"step_no":          i + 1,
			"title":            step.Title,
			"description":      step.Description,
			"recommended_tool": step.ToolName,
			"depends_on":       step.DependsOn,
			"status":           step.Status,
		}
	}
	return p.publish(ctx, id, contracts.EventPlanReplanned, map[string]any{
		"version":       plan.ReplanCount + 1,
		"goal":          plan.Goal,
		"input_summary": inputSummary,
		"steps":         steps,
	})
}

func (p *SequencedEventPublisher) PublishStepStarted(ctx context.Context, id contracts.ID, stepNo int, title string, inputSummary string) error {
	return p.publish(ctx, id, contracts.EventPlanStepStarted, map[string]any{
		"step_no":       stepNo,
		"title":         title,
		"input_summary": inputSummary,
	})
}

func (p *SequencedEventPublisher) PublishStepCompleted(ctx context.Context, id contracts.ID, stepNo int, title string, success bool, outputSummary string, durationMs int64, tokenUsage contracts.TokenUsage) error {
	return p.publish(ctx, id, contracts.EventPlanStepCompleted, map[string]any{
		"step_no":        stepNo,
		"title":          title,
		"success":        success,
		"output_summary": outputSummary,
		"duration_ms":    durationMs,
		"token_usage":    tokenUsage,
	})
}
