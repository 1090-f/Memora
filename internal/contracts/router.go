package contracts

import (
	"context"
	"time"
)

// ExecutionMode 表示 Agent 处理查询时使用的策略。
type ExecutionMode string

// 支持的执行模式常量。
const (
	// ExecutionReact 使用响应式循环，Agent 逐步推理和行动。
	ExecutionReact ExecutionMode = "react"
	// ExecutionPlanExecute 使用计划执行模式，Agent 先生成计划再按步骤执行。
	ExecutionPlanExecute ExecutionMode = "plan_execute"
)

// RouterDecision 表示 Agent 运行的路由决策结果。
type RouterDecision struct {
	ExecutionMode ExecutionMode `json:"execution_mode"` // 选定的执行模式
	ReasonSummary string        `json:"reason_summary"` // 决策理由摘要
	Confidence    float64       `json:"confidence"`     // 决策置信度
	FallbackUsed  bool          `json:"fallback_used"`  // 是否使用了兜底策略
	CreatedAt     time.Time     `json:"created_at"`     // 决策时间
}

// Router 根据查询上下文确定 Agent 的执行模式。
type Router interface {
	// Route 分析 Agent 上下文并返回路由决策。
	Route(ctx context.Context, agentContext AgentContext) (RouterDecision, error)
}
