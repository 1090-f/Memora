package contracts

import "context"

// PlanStatus 表示计划执行的当前状态。
type PlanStatus string

// PlanStepStatus 表示计划步骤执行的当前状态。
type PlanStepStatus string

// 计划与步骤的状态常量。
const (
	// PlanPending 表示计划已创建但尚未开始。
	PlanPending PlanStatus = "pending"
	// PlanExecuting 表示计划正在执行中。
	PlanExecuting PlanStatus = "executing"
	// PlanReplanning 表示计划正在被重新评估和修改。
	PlanReplanning PlanStatus = "replanning"
	// PlanReviewing 表示计划正在接受质量审查。
	PlanReviewing PlanStatus = "reviewing"
	// PlanCompleted 表示计划已全部执行完成。
	PlanCompleted PlanStatus = "completed"
	// PlanFailed 表示计划执行失败。
	PlanFailed PlanStatus = "failed"
	// PlanCancelled 表示计划执行已取消。
	PlanCancelled PlanStatus = "cancelled"
	// StepPending 表示步骤尚未开始。
	StepPending PlanStepStatus = "pending"
	// StepRunning 表示步骤正在执行中。
	StepRunning PlanStepStatus = "running"
	// StepCompleted 表示步骤已成功执行。
	StepCompleted PlanStepStatus = "completed"
	// StepFailed 表示步骤执行失败。
	StepFailed PlanStepStatus = "failed"
	// StepSkipped 表示步骤在执行过程中被跳过。
	StepSkipped PlanStepStatus = "skipped"
	// StepCancelled 表示步骤执行已取消。
	StepCancelled PlanStepStatus = "cancelled"
)

// Plan 表示 Agent 需要按顺序执行的计划。
type Plan struct {
	ID                 ID         `json:"id"`                  // 计划 ID
	RunID              ID         `json:"run_id"`              // 关联的运行 ID
	Version            int        `json:"version"`             // 计划版本（重新规划后递增）
	Goal               string     `json:"goal"`                // 计划目标
	CompletionCriteria []string   `json:"completion_criteria"` // 完成判据列表
	Status             PlanStatus `json:"status"`              // 计划状态
	Steps              []PlanStep `json:"steps"`               // 步骤列表
}

// PlanStep 表示执行计划中的单个步骤。
type PlanStep struct {
	ID                 ID             `json:"id"`                            // 步骤 ID
	StepNo             int            `json:"step_no"`                       // 步骤序号
	Title              string         `json:"title"`                         // 步骤标题
	Description        string         `json:"description,omitempty"`         // 可选：步骤描述
	DependsOn          []int          `json:"depends_on"`                    // 依赖的步骤序号
	ToolHint           string         `json:"tool_hint,omitempty"`           // 可选：建议使用的工具
	CompletionCriteria string         `json:"completion_criteria,omitempty"` // 可选：步骤完成判据
	Status             PlanStepStatus `json:"status"`                        // 步骤状态
}

// ReviewerResult 表示计划审查的结果。
type ReviewerResult struct {
	Result  string `json:"result"`  // 评审结论
	Summary string `json:"summary"` // 评审摘要
}

// Planner 为 Agent 运行创建执行计划。
type Planner interface {
	// Plan 根据 Agent 上下文和配置创建执行计划。
	Plan(ctx context.Context, agentContext AgentContext, config AgentConfig) (Plan, error)
}

// PlanExecutor 按顺序执行计划的步骤。
type PlanExecutor interface {
	// Execute 运行计划步骤并返回包含执行结果的更新后计划。
	Execute(ctx context.Context, agentContext AgentContext, plan Plan) (Plan, error)
}

// PlanReviewer 审查已完成计划的质量和正确性。
type PlanReviewer interface {
	// Review 评估计划执行情况并返回审查结果。
	Review(ctx context.Context, agentContext AgentContext, plan Plan) (ReviewerResult, error)
}
