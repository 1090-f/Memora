package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
)

type captureAgentEventRepo struct{ events []entity.AgentEvent }

func (r *captureAgentEventRepo) BatchCreate(_ context.Context, events []entity.AgentEvent) error {
	r.events = append(r.events, events...)
	return nil
}

func (*captureAgentEventRepo) ListAfterSequence(context.Context, string, int64) ([]entity.AgentEvent, error) {
	return nil, nil
}

func (*captureAgentEventRepo) DeleteByRunID(context.Context, string) error { return nil }

func TestPostgresPublisherPersistsFailureDiagnosticsAndCorrelation(t *testing.T) {
	repo := &captureAgentEventRepo{}
	publisher := NewPostgresEventPublisher(repo)
	payload, _ := json.Marshal(map[string]any{
		"error_code": contracts.ErrModelCallFailed, "failure_stage": contracts.AgentStageModelGenerate,
		"retryable": true, "recovery_advice": "请重试",
	})
	event := contracts.AgentEvent{
		RunID: "run-1", TraceID: "0123456789abcdef0123456789abcdef", RequestID: "request-1",
		Stage: contracts.AgentStageAnswer, Status: contracts.StageFailed, EventType: contracts.EventRunFailed,
		Sequence: 7, Timestamp: time.Now().UTC(), Data: payload,
	}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 1 {
		t.Fatalf("persisted events = %d", len(repo.events))
	}
	stored := repo.events[0]
	if stored.Sequence != 7 || stored.EventType != string(contracts.EventRunFailed) || stored.Stage == nil || *stored.Stage != string(contracts.AgentStageAnswer) || stored.Status == nil || *stored.Status != string(contracts.StageFailed) {
		t.Fatalf("unexpected stored event: %#v", stored)
	}
	if stored.TraceID == nil || *stored.TraceID != event.TraceID || stored.RequestID == nil || *stored.RequestID != event.RequestID {
		t.Fatalf("correlation was not persisted: %#v", stored)
	}
	var storedPayload map[string]any
	if err := json.Unmarshal(stored.Data, &storedPayload); err != nil {
		t.Fatal(err)
	}
	if storedPayload["error_code"] != string(contracts.ErrModelCallFailed) || storedPayload["retryable"] != true {
		t.Fatalf("failure diagnostics were not persisted: %#v", storedPayload)
	}
}

func TestPostgresPublisherDoesNotPersistAnswerDelta(t *testing.T) {
	repo := &captureAgentEventRepo{}
	if err := NewPostgresEventPublisher(repo).Publish(context.Background(), contracts.AgentEvent{EventType: contracts.EventAnswerDelta, Data: []byte("sensitive answer")}); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 0 {
		t.Fatal("answer delta must not be persisted")
	}
}
