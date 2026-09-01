package contracts

import (
	"context"
	"testing"
)

func TestAgentStageReporterPropagatesThroughContext(t *testing.T) {
	called := false
	ctx := WithAgentStageReporter(context.Background(), func(_ context.Context, stage AgentStage, status StageStatus, durationMS int64, _ string, metadata map[string]any) {
		called = stage == AgentStageFusion && status == StageSucceeded && durationMS == 12 && metadata["result_count"] == 3
	})
	ReportAgentStage(ctx, AgentStageFusion, StageSucceeded, 12, "done", map[string]any{"result_count": 3})
	if !called {
		t.Fatal("stage reporter did not receive the observation")
	}
}
