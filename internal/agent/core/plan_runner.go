package core

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

// planRunner 组合已有 Planner、PlanExecutor 和 PlanReviewer 契约，避免重复定义计划协议。
type planRunner struct {
	planner  contracts.Planner
	executor contracts.PlanExecutor
	reviewer contracts.PlanReviewer
}

// NewPlanRunner 创建 Plan-Execute 执行器。
func NewPlanRunner(planner contracts.Planner, executor contracts.PlanExecutor, reviewer contracts.PlanReviewer) PlanRunner {
	return &planRunner{planner: planner, executor: executor, reviewer: reviewer}
}

func (r *planRunner) Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error) {
	if r == nil || r.planner == nil || r.executor == nil || r.reviewer == nil {
		return RunOutput{}, newCoreError(contracts.ErrInternal, ErrExecutionDependency)
	}
	cfg = withDefaults(cfg)
	plan, err := r.planner.Plan(ctx, agentCtx, cfg)
	if err != nil {
		return RunOutput{}, newCoreError(contracts.ErrModelCallFailed, err)
	}
	if err := validatePlan(plan, cfg); err != nil {
		return RunOutput{}, newCoreError(contracts.ErrInvalidArgument, err)
	}
	plan, err = r.executor.Execute(ctx, agentCtx, plan)
	if err != nil {
		return RunOutput{}, newCoreError(contracts.ErrInternal, err)
	}
	review, err := r.reviewer.Review(ctx, agentCtx, plan)
	if err != nil {
		return RunOutput{}, newCoreError(contracts.ErrModelCallFailed, err)
	}
	if review.Result == "failed" {
		return RunOutput{}, fmt.Errorf("plan review failed: %s", review.Summary)
	}
	result := review.Summary
	if result == "" {
		result = plan.Goal
	}
	return RunOutput{FinalResult: result, Summary: result}, nil
}

func validatePlan(plan contracts.Plan, cfg contracts.AgentConfig) error {
	if len(plan.Steps) == 0 || len(plan.Steps) > cfg.MaxPlanSteps {
		return fmt.Errorf("plan step count must be between 1 and %d", cfg.MaxPlanSteps)
	}
	for index, step := range plan.Steps {
		if step.StepNo != index+1 {
			return fmt.Errorf("plan step number must be continuous")
		}
		for _, dependency := range step.DependsOn {
			if dependency < 1 || dependency >= step.StepNo {
				return fmt.Errorf("plan step dependency is invalid")
			}
		}
		// ToolHint 的授权和执行由 PlanExecutor 统一负责，这里不绕过工具执行入口。
	}
	return nil
}
