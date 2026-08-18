package core

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/google/uuid"
)

// EventPublisher 是 Core 内部使用的生命周期事件发布抽象。
type EventPublisher interface {
	Publish(ctx context.Context, event contracts.AgentEvent) error
	PublishRunStarted(ctx context.Context, runID contracts.ID, mode contracts.ExecutionMode) error
	PublishRunCompleted(ctx context.Context, runID contracts.ID, result contracts.AgentRunResult) error
	PublishRunFailed(ctx context.Context, runID contracts.ID, mode contracts.ExecutionMode, err error) error
	PublishRunCancelled(ctx context.Context, runID contracts.ID) error
	PublishRouterSelected(ctx context.Context, runID contracts.ID, decision contracts.RouterDecision) error
	// PublishReactRoundStarted 发布 ReAct 轮次开始事件。
	PublishReactRoundStarted(ctx context.Context, runID contracts.ID, round int) error
	// PublishReactRoundCompleted 发布 ReAct 轮次完成事件。
	PublishReactRoundCompleted(ctx context.Context, runID contracts.ID, round int, toolCallCount int) error
	// PublishToolCallStarted 发布工具调用开始事件。
	PublishToolCallStarted(ctx context.Context, runID contracts.ID, toolName string, callID contracts.ID) error
	// PublishToolCallCompleted 发布工具调用完成事件。
	PublishToolCallCompleted(ctx context.Context, runID contracts.ID, callID contracts.ID, toolName string, success bool, summary string) error
	// PublishAnswerDelta 发布流式回答增量事件。
	PublishAnswerDelta(ctx context.Context, runID contracts.ID, delta string) error
	// PublishPlanCreated 发布计划创建事件。
	PublishPlanCreated(ctx context.Context, runID contracts.ID, plan *contracts.Plan) error
	// PublishPlanReplanned 发布计划重新规划事件。
	PublishPlanReplanned(ctx context.Context, runID contracts.ID, plan *contracts.Plan) error
	// PublishStepStarted 发布计划步骤开始事件。
	PublishStepStarted(ctx context.Context, runID contracts.ID, stepNo int, title string) error
	// PublishStepCompleted 发布计划步骤完成事件。
	PublishStepCompleted(ctx context.Context, runID contracts.ID, stepNo int, title string, success bool) error
}

// NoopEventPublisher 用于未接入事件存储时保持执行链路可运行。
type NoopEventPublisher struct{}

func (NoopEventPublisher) Publish(context.Context, contracts.AgentEvent) error { return nil }
func (NoopEventPublisher) PublishRunStarted(context.Context, contracts.ID, contracts.ExecutionMode) error {
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
func (NoopEventPublisher) PublishReactRoundStarted(context.Context, contracts.ID, int) error {
	return nil
}
func (NoopEventPublisher) PublishReactRoundCompleted(context.Context, contracts.ID, int, int) error {
	return nil
}
func (NoopEventPublisher) PublishToolCallStarted(context.Context, contracts.ID, string, contracts.ID) error {
	return nil
}
func (NoopEventPublisher) PublishToolCallCompleted(context.Context, contracts.ID, contracts.ID, string, bool, string) error {
	return nil
}
func (NoopEventPublisher) PublishAnswerDelta(context.Context, contracts.ID, string) error { return nil }
func (NoopEventPublisher) PublishPlanCreated(context.Context, contracts.ID, *contracts.Plan) error {
	return nil
}
func (NoopEventPublisher) PublishPlanReplanned(context.Context, contracts.ID, *contracts.Plan) error {
	return nil
}
func (NoopEventPublisher) PublishStepStarted(context.Context, contracts.ID, int, string) error {
	return nil
}
func (NoopEventPublisher) PublishStepCompleted(context.Context, contracts.ID, int, string, bool) error {
	return nil
}

// SequencedEventPublisher 为每个 Run 分配单调递增的事件序号。
type SequencedEventPublisher struct {
	publisher contracts.EventPublisher
	mu        sync.Mutex
	sequences map[contracts.ID]int64
}

func NewSequencedEventPublisher(publisher contracts.EventPublisher) *SequencedEventPublisher {
	return &SequencedEventPublisher{publisher: publisher, sequences: make(map[contracts.ID]int64)}
}

func (p *SequencedEventPublisher) Publish(ctx context.Context, event contracts.AgentEvent) error {
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
	return p.Publish(ctx, contracts.AgentEvent{RunID: runID, EventType: typ, Data: payload})
}

func (p *SequencedEventPublisher) PublishRunStarted(ctx context.Context, id contracts.ID, mode contracts.ExecutionMode) error {
	return p.publish(ctx, id, contracts.EventRunStarted, map[string]any{"execution_mode": mode})
}
func (p *SequencedEventPublisher) PublishRunCompleted(ctx context.Context, id contracts.ID, result contracts.AgentRunResult) error {
	return p.publish(ctx, id, contracts.EventRunCompleted, map[string]any{"final_result": result.FinalResult})
}
func (p *SequencedEventPublisher) PublishRunFailed(ctx context.Context, id contracts.ID, mode contracts.ExecutionMode, runErr error) error {
	return p.publish(ctx, id, contracts.EventRunFailed, map[string]any{
		"execution_mode": mode,
		"error_code":     errorCode(runErr),
	})
}
func (p *SequencedEventPublisher) PublishRunCancelled(ctx context.Context, id contracts.ID) error {
	return p.publish(ctx, id, contracts.EventRunCancelled, nil)
}
func (p *SequencedEventPublisher) PublishRouterSelected(ctx context.Context, id contracts.ID, decision contracts.RouterDecision) error {
	return p.publish(ctx, id, contracts.EventRouterCompleted, decision)
}

func (p *SequencedEventPublisher) PublishReactRoundStarted(ctx context.Context, id contracts.ID, round int) error {
	return p.publish(ctx, id, contracts.EventReactRoundStarted, map[string]any{
		"round_no": round,
		"round":    round,
	})
}

func (p *SequencedEventPublisher) PublishReactRoundCompleted(ctx context.Context, id contracts.ID, round int, toolCallCount int) error {
	return p.publish(ctx, id, contracts.EventReactRoundCompleted, map[string]any{
		"round_no":        round,
		"round":           round,
		"tool_call_count": toolCallCount,
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
	return p.publish(ctx, id, contracts.EventAnswerDelta, map[string]any{"delta": delta})
}

func (p *SequencedEventPublisher) PublishPlanCreated(ctx context.Context, id contracts.ID, plan *contracts.Plan) error {
	steps := make([]map[string]any, len(plan.Steps))
	for i, step := range plan.Steps {
		steps[i] = map[string]any{
			"step_no": i + 1,
			"title":   step.Title,
			"status":  step.Status,
		}
	}
	return p.publish(ctx, id, contracts.EventPlanCreated, map[string]any{
		"version": plan.ReplanCount + 1,
		"goal":    plan.Goal,
		"steps":   steps,
	})
}

func (p *SequencedEventPublisher) PublishPlanReplanned(ctx context.Context, id contracts.ID, plan *contracts.Plan) error {
	steps := make([]map[string]any, len(plan.Steps))
	for i, step := range plan.Steps {
		steps[i] = map[string]any{
			"step_no": i + 1,
			"title":   step.Title,
			"status":  step.Status,
		}
	}
	return p.publish(ctx, id, contracts.EventPlanReplanned, map[string]any{
		"version": plan.ReplanCount + 1,
		"goal":    plan.Goal,
		"steps":   steps,
	})
}

func (p *SequencedEventPublisher) PublishStepStarted(ctx context.Context, id contracts.ID, stepNo int, title string) error {
	return p.publish(ctx, id, contracts.EventPlanStepStarted, map[string]any{
		"step_no": stepNo,
		"title":   title,
	})
}

func (p *SequencedEventPublisher) PublishStepCompleted(ctx context.Context, id contracts.ID, stepNo int, title string, success bool) error {
	return p.publish(ctx, id, contracts.EventPlanStepCompleted, map[string]any{
		"step_no": stepNo,
		"title":   title,
		"success": success,
	})
}
