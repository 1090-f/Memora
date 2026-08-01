package contracts

import "context"

type PlanStatus string
type PlanStepStatus string

const (
	PlanPending    PlanStatus     = "pending"
	PlanExecuting  PlanStatus     = "executing"
	PlanReplanning PlanStatus     = "replanning"
	PlanReviewing  PlanStatus     = "reviewing"
	PlanCompleted  PlanStatus     = "completed"
	PlanFailed     PlanStatus     = "failed"
	PlanCancelled  PlanStatus     = "cancelled"
	StepPending    PlanStepStatus = "pending"
	StepRunning    PlanStepStatus = "running"
	StepCompleted  PlanStepStatus = "completed"
	StepFailed     PlanStepStatus = "failed"
	StepSkipped    PlanStepStatus = "skipped"
	StepCancelled  PlanStepStatus = "cancelled"
)

type Plan struct {
	ID                 ID         `json:"id"`
	RunID              ID         `json:"run_id"`
	Version            int        `json:"version"`
	Goal               string     `json:"goal"`
	CompletionCriteria []string   `json:"completion_criteria"`
	Status             PlanStatus `json:"status"`
	Steps              []PlanStep `json:"steps"`
}

type PlanStep struct {
	ID                 ID             `json:"id"`
	StepNo             int            `json:"step_no"`
	Title              string         `json:"title"`
	Description        string         `json:"description,omitempty"`
	DependsOn          []int          `json:"depends_on"`
	ToolHint           string         `json:"tool_hint,omitempty"`
	CompletionCriteria string         `json:"completion_criteria,omitempty"`
	Status             PlanStepStatus `json:"status"`
}

type ReviewerResult struct {
	Result  string `json:"result"`
	Summary string `json:"summary"`
}

type Planner interface {
	Plan(ctx context.Context, agentContext AgentContext, config AgentConfig) (Plan, error)
}

type PlanExecutor interface {
	Execute(ctx context.Context, agentContext AgentContext, plan Plan) (Plan, error)
}

type PlanReviewer interface {
	Review(ctx context.Context, agentContext AgentContext, plan Plan) (ReviewerResult, error)
}
