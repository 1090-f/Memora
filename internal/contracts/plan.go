package contracts

import "context"

// PlanStatus 表示整个计划的整体状态。
type PlanStatus string

// PlanStepStatus 表示单个步骤的状态。
type PlanStepStatus string

// 计划与步骤的状态常量。
const (
	PlanPending    PlanStatus     = "pending"    // 计划待执行
	PlanExecuting  PlanStatus     = "executing"  // 计划执行中
	PlanReplanning PlanStatus     = "replanning" // 计划重新规划中
	PlanReviewing  PlanStatus     = "reviewing"  // 计划评审中
	PlanCompleted  PlanStatus     = "completed"  // 计划已完成
	PlanFailed     PlanStatus     = "failed"     // 计划失败
	PlanCancelled  PlanStatus     = "cancelled"  // 计划已取消
	StepPending    PlanStepStatus = "pending"    // 步骤待执行
	StepRunning    PlanStepStatus = "running"    // 步骤执行中
	StepCompleted  PlanStepStatus = "completed"  // 步骤已完成
	StepFailed     PlanStepStatus = "failed"     // 步骤失败
	StepSkipped    PlanStepStatus = "skipped"    // 步骤被跳过
	StepCancelled  PlanStepStatus = "cancelled"  // 步骤已取消
)

// Plan 是一次计划-执行模式下的整体任务计划。
type Plan struct {
	ID                 ID         `json:"id"`                  // 计划 ID
	RunID              ID         `json:"run_id"`              // 关联的运行 ID
	Version            int        `json:"version"`             // 计划版本（重新规划后递增）
	Goal               string     `json:"goal"`                // 计划目标
	CompletionCriteria []string   `json:"completion_criteria"` // 完成判据列表
	Status             PlanStatus `json:"status"`              // 计划状态
	Steps              []PlanStep `json:"steps"`               // 步骤列表
}

// PlanStep 是计划中的单个执行步骤。
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

// ReviewerResult 是计划评审的产出。
type ReviewerResult struct {
	Result  string `json:"result"`  // 评审结论
	Summary string `json:"summary"` // 评审摘要
}

// Planner 负责根据上下文生成初始计划。
type Planner interface {
	// Plan 生成一份计划。
	Plan(ctx context.Context, agentContext AgentContext, config AgentConfig) (Plan, error)
}

// PlanExecutor 负责按序执行计划中的步骤。
type PlanExecutor interface {
	// Execute 逐步执行计划并返回更新后的计划状态。
	Execute(ctx context.Context, agentContext AgentContext, plan Plan) (Plan, error)
}

// PlanReviewer 负责评审计划质量，判断是否需要重新规划。
type PlanReviewer interface {
	// Review 对计划进行评审并给出结果。
	Review(ctx context.Context, agentContext AgentContext, plan Plan) (ReviewerResult, error)
}
