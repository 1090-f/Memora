package core

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// EventPublisher 是 Core 内部使用的生命周期事件发布抽象。
type EventPublisher interface {
	Publish(ctx context.Context, event contracts.AgentEvent) error
	PublishRunStarted(ctx context.Context, runID contracts.ID, mode contracts.ExecutionMode) error
	PublishRunCompleted(ctx context.Context, runID contracts.ID, result contracts.AgentRunResult) error
	PublishRunFailed(ctx context.Context, runID contracts.ID, err error) error
	PublishRunCancelled(ctx context.Context, runID contracts.ID) error
	PublishRouterSelected(ctx context.Context, runID contracts.ID, decision contracts.RouterDecision) error
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
func (NoopEventPublisher) PublishRunFailed(context.Context, contracts.ID, error) error { return nil }
func (NoopEventPublisher) PublishRunCancelled(context.Context, contracts.ID) error     { return nil }
func (NoopEventPublisher) PublishRouterSelected(context.Context, contracts.ID, contracts.RouterDecision) error {
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
		event.Timestamp = time.Now()
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
func (p *SequencedEventPublisher) PublishRunFailed(ctx context.Context, id contracts.ID, runErr error) error {
	return p.publish(ctx, id, contracts.EventRunFailed, map[string]any{"error_code": errorCode(runErr)})
}
func (p *SequencedEventPublisher) PublishRunCancelled(ctx context.Context, id contracts.ID) error {
	return p.publish(ctx, id, contracts.EventRunCancelled, nil)
}
func (p *SequencedEventPublisher) PublishRouterSelected(ctx context.Context, id contracts.ID, decision contracts.RouterDecision) error {
	return p.publish(ctx, id, contracts.EventRouterCompleted, decision)
}
