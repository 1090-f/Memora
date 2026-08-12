package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// planRunner 实现 Plan-Execute 执行模式。
type planRunner struct {
	planner       *service.PlannerService
	executor      *service.PlanExecutorService
	reviewer      *service.ReviewerService
	replanService *service.ReplanService
	stateStore    service.PlanStateStore
}

// NewPlanRunner 创建 Plan-Execute 执行器。
func NewPlanRunner(
	planner *service.PlannerService,
	executor *service.PlanExecutorService,
	reviewer *service.ReviewerService,
	replanService *service.ReplanService,
	stateStore service.PlanStateStore,
) PlanRunner {
	return &planRunner{
		planner:       planner,
		executor:      executor,
		reviewer:      reviewer,
		replanService: replanService,
		stateStore:    stateStore,
	}
}

// Run 实现 PlanRunner 接口。
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

		// 1. 生成计划
		plan, err := r.planner.Plan(ctx, agentCtx, cfg)
		if err != nil {
			return RunOutput{}, newCoreError(contracts.ErrModelCallFailed, err)
		}

		// 2. 保存计划到数据库
		planID, err := r.stateStore.Save(ctx, plan, agentCtx.RunID, agentCtx.UserID, agentCtx.KnowledgeBaseID)
		if err != nil {
			logger.Error("Failed to save plan", zap.Error(err))
			// 继续执行，不阻断
		} else {
			plan.ID = planID
		}

		// 3. 执行计划
		plan, err = r.executor.Execute(ctx, agentCtx, plan)
		if err != nil {
			return RunOutput{}, newCoreError(contracts.ErrInternal, err)
		}

		// 4. 审查计划
		review, err := r.reviewer.Review(ctx, agentCtx, plan)
		if err != nil {
			return RunOutput{}, newCoreError(contracts.ErrModelCallFailed, err)
		}

		lastPlan = plan
		lastReview = review

		// 5. 检查是否需要重新规划
		if strings.EqualFold(strings.TrimSpace(lastReview.Result), "failed") {
			return RunOutput{}, newCoreError(contracts.ErrInternal, fmt.Errorf("plan review failed"))
		}

		if !r.replanService.ShouldReplan(plan, review) || replanCount == cfg.MaxReplans {
			break
		}

		// 6. 重新规划
		logger.Info("Replanning",
			zap.String("plan_id", string(plan.ID)),
			zap.Int("replan_count", replanCount),
		)

		plan, err = r.replanService.Replan(ctx, agentCtx, plan)
		if err != nil {
			return RunOutput{}, newCoreError(contracts.ErrInternal, err)
		}
	}

	// 构建最终结果
	result := strings.TrimSpace(lastReview.Summary)
	if result == "" {
		result = strings.TrimSpace(lastPlan.Goal)
	}
	if result == "" {
		return RunOutput{}, newCoreError(contracts.ErrInternal, fmt.Errorf("plan result is empty"))
	}

	return RunOutput{FinalResult: result, Summary: result}, nil
}
