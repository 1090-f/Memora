package adkcore

import (
	"encoding/json"
	"sync"

	agenttools "github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/contracts"
)

// knowledgeStatusTracker 汇总一次运行中实际 knowledge_search 调用的结果。
// 多次检索按 sufficient > ambiguous > insufficient 合并，避免后续弱查询覆盖已有强证据。
type knowledgeStatusTracker struct {
	mu     sync.Mutex
	status string
}

func (t *knowledgeStatusTracker) observe(toolName, output string, callErr error) {
	if toolName != agenttools.KnowledgeSearchToolName || callErr != nil {
		return
	}
	status, ok := knowledgeStatusFromToolOutput(output)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if knowledgeStatusPriority(status) > knowledgeStatusPriority(t.status) {
		t.status = status
	}
}

func (t *knowledgeStatusTracker) value() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

func knowledgeStatusFromToolOutput(output string) (string, bool) {
	var toolResult contracts.ToolResult
	if err := json.Unmarshal([]byte(output), &toolResult); err != nil || !toolResult.Success || toolResult.Text == "" {
		return "", false
	}
	var retrievalResult contracts.RetrievalResult
	if err := json.Unmarshal([]byte(toolResult.Text), &retrievalResult); err != nil {
		return "", false
	}
	if knowledgeStatusPriority(retrievalResult.KnowledgeStatus) == 0 {
		return "", false
	}
	return retrievalResult.KnowledgeStatus, true
}

func knowledgeStatusPriority(status string) int {
	switch status {
	case "insufficient":
		return 1
	case "ambiguous":
		return 2
	case "sufficient":
		return 3
	default:
		return 0
	}
}

func knowledgeStatusFromPlan(plan *contracts.Plan) string {
	if plan == nil {
		return ""
	}
	var tracker knowledgeStatusTracker
	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Status == contracts.PlanStepStatusCompleted {
			tracker.observe(step.ToolName, step.Output, nil)
		}
	}
	return tracker.value()
}
