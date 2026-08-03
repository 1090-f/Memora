package contracts

import (
	"context"
	"time"
)

// ExecutionMode Agent 的执行模式。
type ExecutionMode string

// 支持的执行模式常量。
const (
	ExecutionReact       ExecutionMode = "react"        // ReAct 模式
	ExecutionPlanExecute ExecutionMode = "plan_execute" // 计划-执行（Plan & Execute）模式
)

// RouterDecision 是路由（选择执行模式）的结果。
type RouterDecision struct {
	ExecutionMode ExecutionMode `json:"execution_mode"` // 选定的执行模式
	ReasonSummary string        `json:"reason_summary"` // 决策理由摘要
	Confidence    float64       `json:"confidence"`     // 决策置信度
	FallbackUsed  bool          `json:"fallback_used"`  // 是否使用了兜底策略
	CreatedAt     time.Time     `json:"created_at"`     // 决策时间
}

// Router 负责根据上下文决定本次运行的执行模式。
type Router interface {
	// Route 根据 Agent 上下文做出模式选择。
	Route(ctx context.Context, agentContext AgentContext) (RouterDecision, error)
}