package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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

	if err := publisher.PublishReactRoundStarted(ctx, runID, 2, "test input"); err != nil {
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

func TestSequencedEventPublisherAddsStageAndCorrelation(t *testing.T) {
	capture := &capturedEventPublisher{}
	publisher := NewSequencedEventPublisher(capture)
	ctx := contracts.WithCorrelation(context.Background(), "0123456789abcdef0123456789abcdef", "request-1")
	if err := publisher.PublishToolCallStarted(ctx, contracts.ID("run-1"), "knowledge_search", contracts.ID("call-1")); err != nil {
		t.Fatalf("publish tool start: %v", err)
	}
	if len(capture.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(capture.events))
	}
	event := capture.events[0]
	if event.Stage != contracts.AgentStageToolCall || event.Status != contracts.StageRunning {
		t.Fatalf("unexpected stage/status: %s/%s", event.Stage, event.Status)
	}
	if event.TraceID != "0123456789abcdef0123456789abcdef" || event.RequestID != "request-1" {
		t.Fatalf("unexpected correlation: %q/%q", event.TraceID, event.RequestID)
	}
}

func TestRunCompletedEventDoesNotDuplicateAnswer(t *testing.T) {
	capture := &capturedEventPublisher{}
	publisher := NewSequencedEventPublisher(capture)
	if err := publisher.PublishRunCompleted(context.Background(), contracts.ID("run-1"), contracts.AgentRunResult{FinalResult: "sensitive full answer"}); err != nil {
		t.Fatalf("publish completion: %v", err)
	}
	payload := eventPayload(t, capture.events[len(capture.events)-1])
	if _, exists := payload["final_result"]; exists {
		t.Fatal("completion event must not persist the full answer")
	}
}

func TestRunCompletedEventOmitsCitationSnippetByDefault(t *testing.T) {
	capture := &capturedEventPublisher{}
	publisher := NewSequencedEventPublisher(capture)
	result := contracts.AgentRunResult{Citations: []contracts.Citation{{QuotedText: "sensitive source text"}}}
	if err := publisher.PublishRunCompleted(context.Background(), contracts.ID("run-1"), result); err != nil {
		t.Fatalf("publish completion: %v", err)
	}
	if capture.events[0].Stage != contracts.AgentStageModelGenerate || capture.events[0].Status != contracts.StageSucceeded {
		t.Fatalf("missing model generation completion stage: %#v", capture.events[0])
	}
	payload := eventPayload(t, capture.events[1])
	if _, exists := payload["snippet"]; exists {
		t.Fatal("citation event must omit source text unless sensitive capture is enabled")
	}
}

func TestRunTimingCapturesFirstVisibleAnswerOnce(t *testing.T) {
	capture := &capturedEventPublisher{}
	publisher := NewSequencedEventPublisher(capture)
	runID := contracts.ID("run-timing")
	ctx := context.Background()
	if err := publisher.PublishRunStarted(ctx, runID); err != nil {
		t.Fatal(err)
	}
	if err := publisher.PublishModelGenerationStarted(ctx, runID); err != nil {
		t.Fatal(err)
	}
	_ = publisher.PublishAnswerDelta(ctx, runID, "")
	_ = publisher.PublishAnswerDelta(ctx, runID, "首个可见回答")
	_ = publisher.PublishAnswerDelta(ctx, runID, "后续回答")
	now := time.Now().UTC()
	if err := publisher.PublishRunCompleted(ctx, runID, contracts.AgentRunResult{StartedAt: now, EndedAt: now}); err != nil {
		t.Fatal(err)
	}
	timing := publisher.AgentRunTiming(runID)
	if timing.FirstTokenAt == nil || timing.FirstTokenLatencyMS == nil || timing.ModelGenerateDurationMS == nil {
		t.Fatalf("incomplete timing: %#v", timing)
	}
	marked := 0
	for _, event := range capture.events {
		if event.EventType != contracts.EventAnswerDelta {
			continue
		}
		payload := eventPayload(t, event)
		if _, ok := payload["first_token_latency_ms"]; ok {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("first token marker count = %d, want 1", marked)
	}
	if second := publisher.AgentRunTiming(runID); second.FirstTokenAt != nil {
		t.Fatal("timing must be consumed to avoid retaining completed runs")
	}
}
