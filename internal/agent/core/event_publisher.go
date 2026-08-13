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
	return p.publish(ctx, id, contracts.EventReactRoundStarted, map[string]any{"round": round})
}

func (p *SequencedEventPublisher) PublishReactRoundCompleted(ctx context.Context, id contracts.ID, round int, toolCallCount int) error {
	return p.publish(ctx, id, contracts.EventReactRoundCompleted, map[string]any{"round": round, "tool_call_count": toolCallCount})
}

func (p *SequencedEventPublisher) PublishToolCallStarted(ctx context.Context, id contracts.ID, toolName string, callID contracts.ID) error {
	return p.publish(ctx, id, contracts.EventToolStarted, map[string]any{"tool_name": toolName, "call_id": callID})
}

func (p *SequencedEventPublisher) PublishToolCallCompleted(ctx context.Context, id contracts.ID, callID contracts.ID, toolName string, success bool, summary string) error {
	eventType := contracts.EventToolCompleted
	if !success {
		eventType = contracts.EventToolCallFailed
	}
	return p.publish(ctx, id, eventType, map[string]any{
		"tool_name": toolName,
		"call_id":   callID,
		"success":   success,
		"summary":   summary,
	})
}

func (p *SequencedEventPublisher) PublishAnswerDelta(ctx context.Context, id contracts.ID, delta string) error {
	return p.publish(ctx, id, contracts.EventAnswerDelta, map[string]any{"delta": delta})
}
