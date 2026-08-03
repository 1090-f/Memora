package contracts

import (
	"context"
	"time"
)

// AgentConfig 是 Agent 运行的各项限额与控制参数。
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

// DefaultAgentConfig 返回一组合理的默认 Agent 配置。
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MaxReactRounds: 8, MaxPlanSteps: 5, MaxReplans: 1, ReviewerRuns: 1, MaxToolCalls: 10,
		MaxDocumentReadTokens: 6000, MaxToolResultBytes: 1048576, MaxRunSeconds: 300, MemoryTopK: 8}
}

// AgentRunRequest 是一次 Agent 运行的请求。
type AgentRunRequest struct {
	RunID   ID           `json:"run_id"`  // 运行唯一标识
	Context AgentContext `json:"context"` // 运行所需的完整上下文
	Config  AgentConfig  `json:"config"`  // 运行配置
}

// AgentRunResult 是一次 Agent 运行的最终结果。
type AgentRunResult struct {
	RunID           ID            `json:"run_id"`             // 运行 ID
	ExecutionMode   ExecutionMode `json:"execution_mode"`     // 实际采用的执行模式
	KnowledgeStatus string        `json:"knowledge_status"`   // 知识检索状态标识
	FinalResult     string        `json:"final_result"`       // 最终回答文本
	Citations       []Citation    `json:"citations"`          // 回答引用的来源
	Usage           TokenUsage    `json:"usage"`              // token 消耗
	StartedAt       time.Time     `json:"started_at"`         // 开始时间
	EndedAt         time.Time     `json:"ended_at"`           // 结束时间
}

// AgentRunService 抽象 Agent 运行的对外服务能力。
type AgentRunService interface {
	// Run 启动并等待一次 Agent 运行完成。
	Run(ctx context.Context, request AgentRunRequest) (AgentRunResult, error)
	// Cancel 取消一次正在进行的运行。
	Cancel(ctx context.Context, runID, userID ID) error
	// Retry 对一次运行进行重试，返回新的运行 ID。
	Retry(ctx context.Context, runID, userID ID) (ID, error)
}