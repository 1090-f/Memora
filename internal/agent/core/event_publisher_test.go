package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
)

type capturedEventPublisher struct {
	events []contracts.AgentEvent
}

func (p *capturedEventPublisher) Publish(_ context.Context, event contracts.AgentEvent) error {
	p.events = append(p.events, event)
	return nil
}

func eventPayload(t *testing.T, event contracts.AgentEvent) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	return payload
}

func TestSequencedEventPublisherUsesCanonicalReactAndToolFields(t *testing.T) {
	capture := &capturedEventPublisher{}
	publisher := NewSequencedEventPublisher(capture)
	ctx := context.Background()
	runID := contracts.ID("run-1")

	if err := publisher.PublishReactRoundStarted(ctx, runID, 2); err != nil {
		t.Fatalf("publish round: %v", err)
	}
	if err := publisher.PublishToolCallStarted(ctx, runID, "knowledge_search", contracts.ID("call-1")); err != nil {
		t.Fatalf("publish tool start: %v", err)
	}
	if err := publisher.PublishToolCallCompleted(ctx, runID, contracts.ID("call-1"), "knowledge_search", true, "3 results"); err != nil {
		t.Fatalf("publish tool completion: %v", err)
	}

	if len(capture.events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(capture.events))
	}
	if capture.events[0].Sequence != 1 || capture.events[1].Sequence != 2 || capture.events[2].Sequence != 3 {
		t.Fatalf("unexpected sequences: %d, %d, %d", capture.events[0].Sequence, capture.events[1].Sequence, capture.events[2].Sequence)
	}

	round := eventPayload(t, capture.events[0])
	if round["round_no"] != float64(2) {
		t.Fatalf("round_no = %#v, want 2", round["round_no"])
	}
	started := eventPayload(t, capture.events[1])
	if started["tool_call_id"] != "call-1" || started["tool_name"] != "knowledge_search" {
		t.Fatalf("unexpected tool started payload: %#v", started)
	}
	completed := eventPayload(t, capture.events[2])
	if completed["tool_call_id"] != "call-1" || completed["output_summary"] != "3 results" {
		t.Fatalf("unexpected tool completed payload: %#v", completed)
	}
}
