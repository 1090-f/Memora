package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
	events        EventPublisher // 事件发布器，用于实时推送 Plan/Step 生命周期事件
}

// NewPlanRunner 创建 Plan-Execute 执行器。
// events: 事件发布器（由 SequencedEventPublisher 包装），注入后 PlanRunner 可在各阶段发布实时事件。
func NewPlanRunner(
	planner *service.PlannerService,
	executor *service.PlanExecutorService,
	reviewer *service.ReviewerService,
	replanService *service.ReplanService,
	stateStore service.PlanStateStore,
	events EventPublisher,
) PlanRunner {
	if events == nil {
		events = NoopEventPublisher{}
	}
	return &planRunner{
		planner:       planner,
		executor:      executor,
		reviewer:      reviewer,
		replanService: replanService,
		stateStore:    stateStore,
		events:        events,
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

	// 创建线程安全的 token 用量累计器（executeStep 可能并行执行）
	var usageMu sync.Mutex
	var totalUsage contracts.TokenUsage

	// 设置 token 消耗回调，各 service 在每次模型调用后累加用量
	r.planner.SetUsageCallback(func(inputTokens, outputTokens, totalTokens int) {
		usageMu.Lock()
		totalUsage.InputTokens += inputTokens
		totalUsage.OutputTokens += outputTokens
		totalUsage.TotalTokens += totalTokens
		usageMu.Unlock()
	})
	r.executor.SetUsageCallback(func(inputTokens, outputTokens, totalTokens int) {
		usageMu.Lock()
		totalUsage.InputTokens += inputTokens
		totalUsage.OutputTokens += outputTokens
		totalUsage.TotalTokens += totalTokens
		usageMu.Unlock()
	})
	r.reviewer.SetUsageCallback(func(inputTokens, outputTokens, totalTokens int) {
		usageMu.Lock()
		totalUsage.InputTokens += inputTokens
		totalUsage.OutputTokens += outputTokens
		totalUsage.TotalTokens += totalTokens
		usageMu.Unlock()
	})

	var lastReview contracts.ReviewerResult
	var lastPlan contracts.Plan

	for replanCount := 0; replanCount <= cfg.MaxReplans; replanCount++ {
		if err := ctx.Err(); err != nil {
			return RunOutput{Usage: totalUsage}, err
		}
		if err := (DefaultBudgetController{Config: cfg}).CheckRunDuration(startedAt); err != nil {
			return RunOutput{Usage: totalUsage}, err
		}

		// 1. 生成计划
		plan, err := r.planner.Plan(ctx, agentCtx, cfg)
		if err != nil {
			return RunOutput{Usage: totalUsage}, newCoreError(contracts.ErrModelCallFailed, err)
		}

		// 2. 保存计划到数据库
		planID, err := r.stateStore.Save(ctx, plan, agentCtx.RunID, agentCtx.UserID, agentCtx.KnowledgeBaseID)
		if err != nil {
			logger.Error("Failed to save plan", zap.Error(err))
			// 继续执行，不阻断
		} else {
			plan.ID = planID
		}

		// 发布计划创建事件，前端据此初始化计划展示面板
		if err := r.events.PublishPlanCreated(ctx, agentCtx.RunID, plan); err != nil {
			logger.Warn("发布计划创建事件失败",
				zap.String("plan_id", string(plan.ID)),
				zap.Error(err),
			)
		}

		// 设置步骤事件回调，让 PlanExecutorService 在执行步骤时实时发布生命周期事件。
		r.executor.SetStepEventCallbacks(
			func(ctx context.Context, runID contracts.ID, stepNo int, title string) error {
				return r.events.Publish(ctx, contracts.AgentEvent{
					RunID:     runID,
					EventType: contracts.EventStepStarted,
					Data:      mustMarshal(map[string]any{"step_no": stepNo, "title": title, "status": "running"}),
				})
			},
			func(ctx context.Context, runID contracts.ID, stepNo int, title string) error {
				return r.events.Publish(ctx, contracts.AgentEvent{
					RunID:     runID,
					EventType: contracts.EventStepCompleted,
					Data:      mustMarshal(map[string]any{"step_no": stepNo, "title": title, "status": "completed"}),
				})
			},
		)

		// 3. 执行计划
		plan, err = r.executor.Execute(ctx, agentCtx, plan)
		if err != nil {
			return RunOutput{Usage: totalUsage}, newCoreError(contracts.ErrInternal, err)
		}

		// 4. 审查计划
		review, err := r.reviewer.Review(ctx, agentCtx, plan)
		if err != nil {
			return RunOutput{Usage: totalUsage}, newCoreError(contracts.ErrModelCallFailed, err)
		}

		lastPlan = plan
		lastReview = review

		// 5. 检查是否需要重新规划
		if strings.EqualFold(strings.TrimSpace(lastReview.Result), "failed") {
			return RunOutput{Usage: totalUsage}, newCoreError(contracts.ErrInternal, fmt.Errorf("plan review failed"))
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
			return RunOutput{Usage: totalUsage}, newCoreError(contracts.ErrInternal, err)
		}

		// 发布计划重新规划事件
		if err := r.events.PublishPlanReplanned(ctx, agentCtx.RunID, plan); err != nil {
			logger.Warn("发布计划重新规划事件失败",
				zap.String("plan_id", string(plan.ID)),
				zap.Error(err),
			)
		}
	}

	// 构建最终结果
	result := strings.TrimSpace(lastReview.Summary)
	if result == "" {
		result = strings.TrimSpace(lastPlan.Goal)
	}
	if result == "" {
		return RunOutput{Usage: totalUsage}, newCoreError(contracts.ErrInternal, fmt.Errorf("plan result is empty"))
	}

	return RunOutput{
		FinalResult:     result,
		Summary:         result,
		Usage:           totalUsage,
		KnowledgeStatus: agentCtx.KnowledgeStatus,
	}, nil
}

// mustMarshal 将任意值序列化为 JSON RawMessage，序列化失败时 panic。
// 用于步骤事件回调中的数据序列化，因为回调中不应吞掉编码错误。
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("序列化事件数据失败: %v", err))
	}
	return data
}
