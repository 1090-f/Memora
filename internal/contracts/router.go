package contracts

import (
	"context"
	"time"
)

type ExecutionMode string

const (
	ExecutionReact       ExecutionMode = "react"
	ExecutionPlanExecute ExecutionMode = "plan_execute"
)

type RouterDecision struct {
	ExecutionMode ExecutionMode `json:"execution_mode"`
	ReasonSummary string        `json:"reason_summary"`
	Confidence    float64       `json:"confidence"`
	FallbackUsed  bool          `json:"fallback_used"`
	CreatedAt     time.Time     `json:"created_at"`
}

type Router interface {
	Route(ctx context.Context, agentContext AgentContext) (RouterDecision, error)
}
