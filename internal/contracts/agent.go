package contracts

import (
	"context"
	"time"
)

type AgentConfig struct {
	MaxReactRounds        int `json:"max_react_rounds"`
	MaxPlanSteps          int `json:"max_plan_steps"`
	MaxReplans            int `json:"max_replans"`
	ReviewerRuns          int `json:"reviewer_runs"`
	MaxToolCalls          int `json:"max_tool_calls"`
	MaxDocumentReadTokens int `json:"max_document_read_tokens"`
	MaxToolResultBytes    int `json:"max_tool_result_bytes"`
	MaxRunSeconds         int `json:"max_run_seconds"`
	MemoryTopK            int `json:"memory_top_k"`
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MaxReactRounds: 8, MaxPlanSteps: 5, MaxReplans: 1, ReviewerRuns: 1, MaxToolCalls: 10,
		MaxDocumentReadTokens: 6000, MaxToolResultBytes: 1048576, MaxRunSeconds: 300, MemoryTopK: 8}
}

type AgentRunRequest struct {
	RunID   ID           `json:"run_id"`
	Context AgentContext `json:"context"`
	Config  AgentConfig  `json:"config"`
}

type AgentRunResult struct {
	RunID           ID            `json:"run_id"`
	ExecutionMode   ExecutionMode `json:"execution_mode"`
	KnowledgeStatus string        `json:"knowledge_status"`
	FinalResult     string        `json:"final_result"`
	Citations       []Citation    `json:"citations"`
	Usage           TokenUsage    `json:"usage"`
	StartedAt       time.Time     `json:"started_at"`
	EndedAt         time.Time     `json:"ended_at"`
}

type AgentRunService interface {
	Run(ctx context.Context, request AgentRunRequest) (AgentRunResult, error)
	Cancel(ctx context.Context, runID, userID ID) error
	Retry(ctx context.Context, runID, userID ID) (ID, error)
}
