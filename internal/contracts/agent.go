package contracts

import (
	"context"
	"fmt"
	"time"
)

// AgentConfig 定义 Agent 执行参数的上限配置。
type AgentConfig struct {
	MaxReactRounds        int `json:"max_react_rounds"`         // ReAct 模式最大轮数
	MaxPlanSteps          int `json:"max_plan_steps"`           // 计划（Plan）最大步骤数
	MaxReplans            int `json:"max_replans"`              // 允许重新规划的最大次数
	ReviewerRuns          int `json:"reviewer_runs"`            // 计划评审（Reviewer）运行次数
	MaxToolCalls          int `json:"max_tool_calls"`           // 单次运行最大工具调用次数
	MaxDocumentReadTokens int `json:"max_document_read_tokens"` // 单次文档读取最大 token
	MaxToolResultBytes    int `json:"max_tool_result_bytes"`    // 工具结果最大字节数
	MaxRunSeconds         int `json:"max_run_seconds"`          // 单次运行最大时长（秒）
	MemoryTopK            int `json:"memory_top_k"`             // 记忆检索返回条数
}

// DefaultAgentConfig 返回具有合理默认值的 AgentConfig。
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MaxReactRounds: 8, MaxPlanSteps: 5, MaxReplans: 1, ReviewerRuns: 1, MaxToolCalls: 10,
		MaxDocumentReadTokens: 6000, MaxToolResultBytes: 1048576, MaxRunSeconds: 300, MemoryTopK: 8}
}

// AgentRunRequest 表示启动 Agent 执行运行的请求。
type AgentRunRequest struct {
	RunID   ID           `json:"run_id"`  // 运行唯一标识
	Context AgentContext `json:"context"` // 运行所需的完整上下文
	Config  AgentConfig  `json:"config"`  // 运行配置
}

// AgentRunError 表示已经确定执行模式后发生的运行失败。
type AgentRunError struct {
	ExecutionMode ExecutionMode
	Err           error
}

func (e *AgentRunError) Error() string { return fmt.Sprintf("%s: %v", e.ExecutionMode, e.Err) }
func (e *AgentRunError) Unwrap() error { return e.Err }

// AgentRunResult 表示已完成的 Agent 执行运行的结果。
type AgentRunResult struct {
	RunID           ID            `json:"run_id"`           // 运行 ID
	ExecutionMode   ExecutionMode `json:"execution_mode"`   // 实际采用的执行模式
	KnowledgeStatus string        `json:"knowledge_status"` // 知识检索状态标识
	FinalResult     string        `json:"final_result"`     // 最终回答文本
	Citations       []Citation    `json:"citations"`        // 回答引用的来源
	Usage           TokenUsage    `json:"usage"`            // token 消耗
	StartedAt       time.Time     `json:"started_at"`       // 开始时间
	EndedAt         time.Time     `json:"ended_at"`         // 结束时间
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
