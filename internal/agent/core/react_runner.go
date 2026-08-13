package core

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service"
)

// ReactRunner 执行有限轮次的 ReAct 循环。
// 参考 PlanRunner 的设计模式，作为薄编排层，核心业务逻辑委托给 ReactService。
type ReactRunner interface {
	Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error)
}

// reactRunner 使用 ReactService 执行 ReAct 循环，并在外层叠加预算控制和引用收集。
type reactRunner struct {
	service   *service.ReactService // ReAct 循环业务逻辑
	events    EventPublisher        // 事件发布器
	budget    BudgetController      // 预算控制器
	collector CitationCollector     // 引用收集器
}

// NewReactRunner 创建 ReAct 执行器。
// 参数:
//   - reactService: ReAct 循环服务（注入 ModelFactory、ToolExecutor、Registry）
//   - events: 事件发布器，用于发布 ReAct 生命周期事件
//
// BudgetController 和 CitationCollector 在 Run 中按需初始化。
func NewReactRunner(reactService *service.ReactService, events EventPublisher) ReactRunner {
	if events == nil {
		events = NoopEventPublisher{}
	}
	return &reactRunner{
		service:   reactService,
		events:    events,
		collector: NewCitationCollector(),
	}
}

// Run 实现 ReactRunner 接口。
// 编排流程：初始化预算 → 执行 ReAct 循环 → 归一化结果
func (r *reactRunner) Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error) {
	// 前置校验：确保所有依赖已注入
	if r == nil || r.service == nil {
		return RunOutput{}, newCoreError(contracts.ErrInternal, ErrExecutionDependency)
	}

	// 使用默认值填充未设置的配置项
	cfg = withDefaults(cfg)

	// 初始化预算控制器（如果未设置，使用默认值）
	budget := r.budget
	if budget == nil {
		budget = DefaultBudgetController{Config: cfg}
	}

	// 重置引用收集器
	r.collector.Reset()

	startedAt := time.Now()
	runID := agentCtx.RunID

	// 创建带超时的 Context
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.MaxRunSeconds)*time.Second)
	defer cancel()

	// 工具调用计数
	var totalToolCalls int
	var accumulatedUsage contracts.TokenUsage

	// 调用 ReactService 执行 ReAct 循环
	// 传递回调函数用于发布事件和预算检查
	result, err := r.service.RunReActLoop(
		ctx,
		agentCtx,
		cfg,
		// onRoundStarted: 轮次开始时回调
		func(ctx context.Context, round int) error {
			// 预算检查：轮次上限
			if err := budget.CheckReactRounds(round); err != nil {
				return err
			}
			// 预算检查：运行时长
			if err := budget.CheckRunDuration(startedAt); err != nil {
				return err
			}
			// 发布轮次开始事件
			return r.events.PublishReactRoundStarted(ctx, runID, round)
		},
		// onToolStarted: 工具调用开始时回调
		func(ctx context.Context, toolName string, callID contracts.ID) error {
			totalToolCalls++
			// 预算检查：工具调用次数
			if err := budget.CheckToolCalls(totalToolCalls); err != nil {
				return err
			}
			// 发布工具调用开始事件
			return r.events.PublishToolCallStarted(ctx, runID, toolName, callID)
		},
		// onToolCompleted: 工具调用完成时回调
		func(ctx context.Context, toolName string, callID contracts.ID, success bool, summary string) error {
			// 发布工具调用完成事件
			return r.events.PublishToolCallCompleted(ctx, runID, callID, toolName, success, summary)
		},
		// onAnswerDelta: 流式回答增量回调
		func(ctx context.Context, delta string) error {
			return r.events.PublishAnswerDelta(ctx, runID, delta)
		},
	)

	// 累加 Token 用量
	accumulatedUsage = result.Usage

	// 收集引用
	if len(result.Citations) > 0 {
		r.collector.Add(result.Citations)
	}

	// 如果 ReAct 循环因预算超限而失败，但已有部分结果，返回部分结果
	if err != nil {
		// 检查是否是预算超限错误
		if isBudgetError(err) && result.FinalResult != "" {
			return RunOutput{
				FinalResult:     result.FinalResult,
				Citations:       r.collector.Get(),
				Usage:           accumulatedUsage,
				Summary:         result.FinalResult,
				KnowledgeStatus: agentCtx.KnowledgeStatus,
			}, nil
		}
		return RunOutput{
			FinalResult:     result.FinalResult,
			Citations:       r.collector.Get(),
			Usage:           accumulatedUsage,
			Summary:         result.FinalResult,
			KnowledgeStatus: agentCtx.KnowledgeStatus,
		}, err
	}

	// 归一化最终结果
	return RunOutput{
		FinalResult:     result.FinalResult,
		Citations:       r.collector.Get(),
		Usage:           accumulatedUsage,
		Summary:         result.FinalResult,
		KnowledgeStatus: agentCtx.KnowledgeStatus,
	}, nil
}

// isBudgetError 判断是否为预算超限错误。
func isBudgetError(err error) bool {
	if err == nil {
		return false
	}
	// 检查轮次超限、工具调用超限、运行时长超限等
	return containsSubstring(err.Error(), "rounds") ||
		containsSubstring(err.Error(), "tool calls") ||
		containsSubstring(err.Error(), "超时") ||
		containsSubstring(err.Error(), "budget")
}

// containsSubstring 检查字符串是否包含子串。
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

// searchSubstring 简单子串搜索。
func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
