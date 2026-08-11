package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/compose"
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

type planState struct {
	Context contracts.AgentContext
	Config  contracts.AgentConfig
	Plan    contracts.Plan
	Review  contracts.ReviewerResult
}

func (r *planRunner) Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error) {
	if r == nil || r.planner == nil || r.executor == nil || r.reviewer == nil {
		return RunOutput{}, newCoreError(contracts.ErrInternal, ErrExecutionDependency)
	}
	cfg = withDefaults(cfg)
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.MaxRunSeconds)*time.Second)
	defer cancel()

	var lastReview contracts.ReviewerResult
	var lastPlan contracts.Plan
	for replanCount := 0; replanCount <= cfg.MaxReplans; replanCount++ {
		if err := ctx.Err(); err != nil {
			return RunOutput{}, err
		}
		if err := (DefaultBudgetController{Config: cfg}).CheckRunDuration(startedAt); err != nil {
			return RunOutput{}, err
		}
		state, err := r.runGraph(ctx, agentCtx, cfg)
		if err != nil {
			return RunOutput{}, err
		}
		lastPlan = state.Plan
		lastReview = state.Review
		if strings.EqualFold(strings.TrimSpace(lastReview.Result), "failed") {
			return RunOutput{}, newCoreError(contracts.ErrInternal, fmt.Errorf("plan review failed"))
		}
		if !strings.EqualFold(strings.TrimSpace(lastReview.Result), "needs_attention") || replanCount == cfg.MaxReplans {
			break
		}
	}

	result := strings.TrimSpace(lastReview.Summary)
	if result == "" {
		result = strings.TrimSpace(lastPlan.Goal)
	}
	if result == "" {
		return RunOutput{}, newCoreError(contracts.ErrInternal, fmt.Errorf("plan result is empty"))
	}
	return RunOutput{FinalResult: result, Summary: result}, nil
}

func (r *planRunner) runGraph(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (planState, error) {
	budget := DefaultBudgetController{Config: cfg}
	state := planState{Context: agentCtx, Config: cfg}
	graph := compose.NewGraph[planState, planState]()
	if err := graph.AddLambdaNode("planner", compose.InvokableLambda(func(nodeCtx context.Context, input planState) (planState, error) {
		plan, err := r.planner.Plan(nodeCtx, input.Context, input.Config)
		if err != nil {
			return input, newCoreError(contracts.ErrModelCallFailed, err)
		}
		if err := validatePlan(plan, input.Config); err != nil {
			return input, newCoreError(contracts.ErrInvalidArgument, err)
		}
		input.Plan = plan
		return input, nil
	})); err != nil {
		return planState{}, newCoreError(contracts.ErrInternal, err)
	}
	if err := graph.AddLambdaNode("executor", compose.InvokableLambda(func(nodeCtx context.Context, input planState) (planState, error) {
		if err := nodeCtx.Err(); err != nil {
			return input, err
		}
		if err := budget.CheckPlanSteps(len(input.Plan.Steps)); err != nil {
			return input, err
		}
		plan, err := r.executor.Execute(nodeCtx, input.Context, input.Plan)
		if err != nil {
			return input, newCoreError(contracts.ErrInternal, err)
		}
		input.Plan = plan
		return input, nil
	})); err != nil {
		return planState{}, newCoreError(contracts.ErrInternal, err)
	}
	if err := graph.AddLambdaNode("reviewer", compose.InvokableLambda(func(nodeCtx context.Context, input planState) (planState, error) {
		review, err := r.reviewer.Review(nodeCtx, input.Context, input.Plan)
		if err != nil {
			return input, newCoreError(contracts.ErrModelCallFailed, err)
		}
		input.Review = review
		return input, nil
	})); err != nil {
		return planState{}, newCoreError(contracts.ErrInternal, err)
	}
	for from, to := range map[string]string{compose.START: "planner", "planner": "executor", "executor": "reviewer", "reviewer": compose.END} {
		if err := graph.AddEdge(from, to); err != nil {
			return planState{}, newCoreError(contracts.ErrInternal, err)
		}
	}
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return planState{}, newCoreError(contracts.ErrInternal, err)
	}
	return runnable.Invoke(ctx, state)
}

func validatePlan(plan contracts.Plan, cfg contracts.AgentConfig) error {
	if len(plan.Steps) == 0 || len(plan.Steps) > cfg.MaxPlanSteps {
		return fmt.Errorf("plan step count must be between 1 and %d", cfg.MaxPlanSteps)
	}
	for index, step := range plan.Steps {
		if step.StepNo != index+1 {
			return fmt.Errorf("plan step number must be continuous")
		}
		seenDependencies := make(map[int]struct{}, len(step.DependsOn))
		for _, dependency := range step.DependsOn {
			if dependency < 1 || dependency >= step.StepNo {
				return fmt.Errorf("plan step dependency is invalid")
			}
			if _, exists := seenDependencies[dependency]; exists {
				return fmt.Errorf("plan step dependency is duplicated")
			}
			seenDependencies[dependency] = struct{}{}
		}
		if step.Title == "" {
			return fmt.Errorf("plan step title is required")
		}
	}
	return nil
}
