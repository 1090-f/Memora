package contracts

import (
	"context"
	"encoding/json"
	"time"
)

type EventType string

const (
	EventRunQueued       EventType = "agent.run.queued"
	EventRunStarted      EventType = "agent.run.started"
	EventRouterCompleted EventType = "agent.router.completed"
	EventStepStarted     EventType = "agent.step.started"
	EventStepCompleted   EventType = "agent.step.completed"
	EventToolStarted     EventType = "agent.tool.started"
	EventToolCompleted   EventType = "agent.tool.completed"
	EventAnswerDelta     EventType = "agent.answer.delta"
	EventRunCompleted    EventType = "agent.run.completed"
	EventRunFailed       EventType = "agent.run.failed"
	EventRunCancelled    EventType = "agent.run.cancelled"
)

type AgentEvent struct {
	EventID   ID              `json:"event_id"`
	RunID     ID              `json:"run_id"`
	EventType EventType       `json:"event_type"`
	Sequence  int64           `json:"sequence"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type EventPublisher interface {
	Publish(ctx context.Context, event AgentEvent) error
}

type EventSubscriber interface {
	Subscribe(ctx context.Context, runID ID, afterSequence int64) (<-chan AgentEvent, error)
}
