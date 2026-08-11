package service

import (
	"context"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// ReplanService 实现计划重新规划逻辑。
type ReplanService struct {
	planner    contracts.Planner
	stateStore PlanStateStore
	maxReplans int
}

// NewReplanService 创建 ReplanService 实例。
func NewReplanService(planner contracts.Planner, stateStore PlanStateStore, maxReplans int) *ReplanService {
	return &ReplanService{
		planner:    planner,
		stateStore: stateStore,
		maxReplans: maxReplans,
	}
}

// Replan 重新规划计划。
func (s *ReplanService) Replan(ctx context.Context, agentContext contracts.AgentContext, plan contracts.Plan) (contracts.Plan, error) {
	start := time.Now()

	logger.Info("开始重新规划",
		zap.String("plan_id", string(plan.ID)),
		zap.Int("version", plan.Version),
		zap.Int("max_replans", s.maxReplans),
	)

	// 检查是否已达重规划上限
	if plan.Version > s.maxReplans {
		return plan, fmt.Errorf("已达重规划上限 %d，当前版本 %d", s.maxReplans, plan.Version)
	}

	// 更新计划状态为重新规划中
	if err := s.stateStore.UpdateStatus(ctx, plan.ID, contracts.PlanReplanning); err != nil {
		return plan, fmt.Errorf("update plan status: %w", err)
	}

	// 收集已执行步骤的结果
	completedSteps := s.getCompletedSteps(plan.Steps)

	// 调用 Planner 重新生成剩余步骤
	newPlan, err := s.planner.Plan(ctx, agentContext, contracts.AgentConfig{
		MaxPlanSteps: len(plan.Steps),
		MaxReplans:   s.maxReplans,
	})
	if err != nil {
		return plan, fmt.Errorf("replan: %w", err)
	}

	// 合并步骤：保留已完成步骤，替换未执行步骤
	mergedPlan := s.mergeSteps(plan, newPlan, completedSteps)

	// 递增版本号
	mergedPlan.Version = plan.Version + 1

	// 保存新计划
	if _, err := s.stateStore.Save(ctx, mergedPlan, plan.RunID, agentContext.UserID, agentContext.KnowledgeBaseID); err != nil {
		return plan, fmt.Errorf("save replanned plan: %w", err)
	}

	logger.Info("重新规划完成",
		zap.String("plan_id", string(mergedPlan.ID)),
		zap.Int("version", mergedPlan.Version),
		zap.Int("steps", len(mergedPlan.Steps)),
		zap.Duration("elapsed", time.Since(start)),
	)

	return mergedPlan, nil
}

// getCompletedSteps 获取已完成的步骤。
func (s *ReplanService) getCompletedSteps(steps []contracts.PlanStep) []contracts.PlanStep {
	var completed []contracts.PlanStep
	for _, step := range steps {
		if step.Status == contracts.StepCompleted {
			completed = append(completed, step)
		}
	}
	return completed
}

// mergeSteps 合并步骤。
func (s *ReplanService) mergeSteps(oldPlan, newPlan contracts.Plan, completedSteps []contracts.PlanStep) contracts.Plan {
	// 创建新的步骤列表
	var mergedSteps []contracts.PlanStep

	// 添加已完成的步骤
	mergedSteps = append(mergedSteps, completedSteps...)

	// 添加新计划中未执行的步骤
	for _, step := range newPlan.Steps {
		// 检查是否已在已完成步骤中
		alreadyCompleted := false
		for _, completed := range completedSteps {
			if step.StepNo == completed.StepNo {
				alreadyCompleted = true
				break
			}
		}

		if !alreadyCompleted {
			// 重新编号
			step.StepNo = len(mergedSteps) + 1
			mergedSteps = append(mergedSteps, step)
		}
	}

	// 重新计算依赖关系
	mergedSteps = s.recalculateDependencies(mergedSteps)

	return contracts.Plan{
		ID:                 oldPlan.ID,
		RunID:              oldPlan.RunID,
		Version:            oldPlan.Version, // 外部会递增
		Goal:               oldPlan.Goal,
		CompletionCriteria: oldPlan.CompletionCriteria,
		Status:             contracts.PlanPending,
		Steps:              mergedSteps,
	}
}

// recalculateDependencies 重新计算依赖关系。
func (s *ReplanService) recalculateDependencies(steps []contracts.PlanStep) []contracts.PlanStep {
	// 创建步骤序号到索引的映射
	stepMap := make(map[int]int)
	for i, step := range steps {
		stepMap[step.StepNo] = i
	}

	// 重新计算依赖关系
	for i := range steps {
		var newDependsOn []int
		for _, dep := range steps[i].DependsOn {
			if _, exists := stepMap[dep]; exists {
				newDependsOn = append(newDependsOn, dep)
			}
		}
		steps[i].DependsOn = newDependsOn
	}

	return steps
}

// ShouldReplan 判断是否应该重新规划。
func (s *ReplanService) ShouldReplan(plan contracts.Plan, review contracts.ReviewerResult) bool {
	// 如果审查结果为 "needs_attention"，则应该重新规划
	if review.Result == "needs_attention" {
		return true
	}

	// 如果有步骤失败，也应该重新规划
	for _, step := range plan.Steps {
		if step.Status == contracts.StepFailed {
			return true
		}
	}

	return false
}
