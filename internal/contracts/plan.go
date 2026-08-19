package contracts

import (
	"fmt"
	"time"
)

// PlanStatus 计划状态
type PlanStatus string

const (
	PlanStatusPending   PlanStatus = "pending"
	PlanStatusRunning   PlanStatus = "running"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusFailed    PlanStatus = "failed"
	PlanStatusCancelled PlanStatus = "cancelled"
)

// PlanStepStatus 步骤状态
type PlanStepStatus string

type PlanStepKind string

const (
	PlanStepKindTool      PlanStepKind = "tool"
	PlanStepKindReasoning PlanStepKind = "reasoning"
)

type ToolPolicy string

const (
	ToolPolicyRequired  ToolPolicy = "required"
	ToolPolicyPreferred ToolPolicy = "preferred"
	ToolPolicyForbidden ToolPolicy = "forbidden"
)

const (
	PlanStepStatusPending   PlanStepStatus = "pending"
	PlanStepStatusRunning   PlanStepStatus = "running"
	PlanStepStatusCompleted PlanStepStatus = "completed"
	PlanStepStatusFailed    PlanStepStatus = "failed"
	PlanStepStatusSkipped   PlanStepStatus = "skipped"
)

// StepErrorKind 步骤错误类型
type StepErrorKind string

const (
	StepErrorNotFound        StepErrorKind = "not_found"
	StepErrorTimeout         StepErrorKind = "timeout"
	StepErrorInvalidArgument StepErrorKind = "invalid_argument"
	StepErrorPermission      StepErrorKind = "permission_denied"
	StepErrorUnavailable     StepErrorKind = "unavailable"
	StepErrorPermanent       StepErrorKind = "permanent"
	StepErrorDependency      StepErrorKind = "dependency_failed"
)

// StepExecutionError 表示步骤执行错误
type StepExecutionError struct {
	Kind        StepErrorKind
	Recoverable bool
	Retryable   bool
	Message     string
}

func (e *StepExecutionError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Kind, e.Message)
}

// FailurePolicy 步骤失败策略
type FailurePolicy string

const (
	FailurePolicyFail     FailurePolicy = "fail"
	FailurePolicyRetry    FailurePolicy = "retry"
	FailurePolicyFallback FailurePolicy = "fallback"
	FailurePolicySkip     FailurePolicy = "skip"
)

// URLEvidence 表示合法的 URL 来源
type URLEvidence struct {
	URL        string `json:"url"`
	SourceStep int    `json:"source_step"`
	ToolCallID string `json:"tool_call_id"`
	Source     string `json:"source"` // "user_query", "search_result", "tool_result", "system_config"
}

// Plan 结构化执行计划
type Plan struct {
	ID          ID         `json:"id"`
	RunID       ID         `json:"run_id"`
	Goal        string     `json:"goal"`         // 用户目标
	Steps       []PlanStep `json:"steps"`        // 执行步骤
	MaxSteps    int        `json:"max_steps"`    // 最大步数限制
	ReplanCount int        `json:"replan_count"` // 重规划次数
	MaxReplans  int        `json:"max_replans"`  // 最大重规划次数
	Status      PlanStatus `json:"status"`
	FinalAnswer string     `json:"final_answer"` // 最终答案
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// HasFailures 检查计划是否有失败的步骤
func (p *Plan) HasFailures() bool {
	for _, step := range p.Steps {
		if step.Status == PlanStepStatusFailed {
			return true
		}
	}
	return false
}

// GetPendingSteps 获取所有待执行的步骤
func (p *Plan) GetPendingSteps() []PlanStep {
	var pending []PlanStep
	for _, step := range p.Steps {
		if step.Status == PlanStepStatusPending {
			pending = append(pending, step)
		}
	}
	return pending
}

// GetCompletedSteps 获取所有已完成的步骤
func (p *Plan) GetCompletedSteps() []PlanStep {
	var completed []PlanStep
	for _, step := range p.Steps {
		if step.Status == PlanStepStatusCompleted {
			completed = append(completed, step)
		}
	}
	return completed
}

// GetFailedSteps 获取所有失败的步骤
func (p *Plan) GetFailedSteps() []PlanStep {
	var failed []PlanStep
	for _, step := range p.Steps {
		if step.Status == PlanStepStatusFailed {
			failed = append(failed, step)
		}
	}
	return failed
}

// PlanStep 单个执行步骤
type PlanStep struct {
	ID                   ID             `json:"id"`
	StepNumber           int            `json:"step_number"` // 步骤序号
	Title                string         `json:"title"`       // 步骤标题
	Description          string         `json:"description"` // 步骤描述
	Kind                 PlanStepKind   `json:"kind,omitempty"`
	ToolPolicy           ToolPolicy     `json:"tool_policy,omitempty"`
	RequiredCapabilities []string       `json:"required_capabilities,omitempty"`
	ToolName             string         `json:"tool_name"`  // 执行工具名（可选）
	Arguments            map[string]any `json:"arguments"`  // 工具参数（可选）
	DependsOn            []ID           `json:"depends_on"` // 依赖的步骤 ID
	Status               PlanStepStatus `json:"status"`
	Output               string         `json:"output"`                // 步骤输出
	Error                string         `json:"error"`                 // 错误信息
	Recoverable          bool           `json:"recoverable,omitempty"` // 是否可恢复（如404错误）
	StartedAt            *time.Time     `json:"started_at"`
	CompletedAt          *time.Time     `json:"completed_at"`
}

// ReviewerResult 审查结果
type ReviewerResult struct {
	Approved   bool             `json:"approved"`
	Issues     string           `json:"issues"`               // 存在的问题
	Suggestion string           `json:"suggestion"`           // 改进建议
	FactCheck  *FactCheckResult `json:"fact_check,omitempty"` // 事实核查结果
}

// FactCheckResult 事实核查结果
type FactCheckResult struct {
	Consistent        bool     `json:"consistent"`         // 事实是否一致
	InconsistentFacts []string `json:"inconsistent_facts"` // 不一致的事实列表
}
