package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
)

type stageCapturePublisher struct{ events []contracts.AgentEvent }

func (p *stageCapturePublisher) Publish(_ context.Context, event contracts.AgentEvent) error {
	p.events = append(p.events, event)
	return nil
}

func TestPublishStageIncludesLifecycleTimes(t *testing.T) {
	capture := &stageCapturePublisher{}
	worker := &AgentWorker{events: capture}
	runID := contracts.ID("run-1")
	worker.publishStage(context.Background(), runID, contracts.AgentStageFusion, contracts.StageRunning, 0, "开始")
	worker.publishStage(context.Background(), runID, contracts.AgentStageFusion, contracts.StageSucceeded, 3, "完成")
	if len(capture.events) != 2 {
		t.Fatalf("expected two events, got %d", len(capture.events))
	}
	var started, completed contracts.StageObservation
	if err := json.Unmarshal(capture.events[0].Data, &started); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(capture.events[1].Data, &completed); err != nil {
		t.Fatal(err)
	}
	if started.StartedAt == nil || started.EndedAt != nil {
		t.Fatalf("unexpected running observation: %#v", started)
	}
	if completed.StartedAt == nil || completed.EndedAt == nil || completed.DurationMS == nil {
		t.Fatalf("unexpected terminal observation: %#v", completed)
	}
}
