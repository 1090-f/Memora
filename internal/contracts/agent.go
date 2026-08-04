package contracts

import (
	"context"
	"time"
)

// AgentConfig 定义 Agent 执行参数的上限配置。
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

// DefaultAgentConfig 返回具有合理默认值的 AgentConfig。
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MaxReactRounds: 8, MaxPlanSteps: 5, MaxReplans: 1, ReviewerRuns: 1, MaxToolCalls: 10,
		MaxDocumentReadTokens: 6000, MaxToolResultBytes: 1048576, MaxRunSeconds: 300, MemoryTopK: 8}
}

// AgentRunRequest 表示启动 Agent 执行运行的请求。
type AgentRunRequest struct {
	RunID   ID           `json:"run_id"`
	Context AgentContext `json:"context"`
	Config  AgentConfig  `json:"config"`
}

// AgentRunResult 表示已完成的 Agent 执行运行的结果。
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

// AgentRunService 定义 Agent 执行运行管理的接口。
type AgentRunService interface {
	// Run 执行一次 Agent 运行并返回结果。
	Run(ctx context.Context, request AgentRunRequest) (AgentRunResult, error)
	// Cancel 停止一个正在运行的 Agent 执行。
	Cancel(ctx context.Context, runID, userID ID) error
	// Retry 重新启动一个失败的 Agent 执行并返回新的运行 ID。
	Retry(ctx context.Context, runID, userID ID) (ID, error)
}
