package adkcore

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
)

func knowledgeToolOutput(t *testing.T, status string) string {
	t.Helper()
	inner, err := json.Marshal(contracts.RetrievalResult{KnowledgeStatus: status})
	if err != nil {
		t.Fatal(err)
	}
	outer, err := json.Marshal(contracts.ToolResult{ToolName: "knowledge_search", Text: string(inner), Success: true})
	if err != nil {
		t.Fatal(err)
	}
	return string(outer)
}

func TestKnowledgeStatusTrackerUsesActualSearchResult(t *testing.T) {
	var tracker knowledgeStatusTracker
	tracker.observe("web_search", knowledgeToolOutput(t, "sufficient"), nil)
	tracker.observe("knowledge_search", knowledgeToolOutput(t, "insufficient"), nil)
	tracker.observe("knowledge_search", knowledgeToolOutput(t, "ambiguous"), nil)
	tracker.observe("knowledge_search", knowledgeToolOutput(t, "sufficient"), nil)
	tracker.observe("knowledge_search", knowledgeToolOutput(t, "insufficient"), nil)
	if got := tracker.value(); got != "sufficient" {
		t.Fatalf("expected sufficient, got %q", got)
	}
}

func TestKnowledgeStatusTrackerIgnoresFailedAndMalformedSearches(t *testing.T) {
	var tracker knowledgeStatusTracker
	tracker.observe("knowledge_search", knowledgeToolOutput(t, "sufficient"), errors.New("failed"))
	tracker.observe("knowledge_search", `{"success":true,"text":"not-json"}`, nil)
	tracker.observe("knowledge_search", knowledgeToolOutput(t, "unknown"), nil)
	if got := tracker.value(); got != "" {
		t.Fatalf("expected unassessed status, got %q", got)
	}
}

func TestKnowledgeStatusFromPlanReadsCompletedKnowledgeSteps(t *testing.T) {
	plan := &contracts.Plan{Steps: []contracts.PlanStep{
		{ToolName: "knowledge_search", Status: contracts.PlanStepStatusCompleted, Output: knowledgeToolOutput(t, "insufficient")},
		{ToolName: "knowledge_search", Status: contracts.PlanStepStatusFailed, Output: knowledgeToolOutput(t, "sufficient")},
		{ToolName: "knowledge_search", Status: contracts.PlanStepStatusCompleted, Output: knowledgeToolOutput(t, "ambiguous")},
	}}
	if got := knowledgeStatusFromPlan(plan); got != "ambiguous" {
		t.Fatalf("expected ambiguous, got %q", got)
	}
}
