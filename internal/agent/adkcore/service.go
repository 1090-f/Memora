package adkcore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
)

// Service 是基于 Eino ADK ChatModelAgent 和 Router 的 AgentRunService。
// 每次 Run() 先由 Router 决定 execution mode，再分发到 ReactRunner 或 PlanExecuteGraph。
type Service struct {
	reactRunner       *ADKReactRunner
	planGraph         *PlanExecuteGraph
	router            contracts.Router
	eventPublisher    core.EventPublisher
	citationCollector core.CitationCollector
	repository        core.RunRepository

	mu     sync.Mutex
	cancel map[contracts.ID]context.CancelFunc
}

// NewService 创建带路由功能的 Agent 运行服务。
func NewService(
	reactRunner *ADKReactRunner,
	planGraph *PlanExecuteGraph,
	router contracts.Router,
	eventPublisher core.EventPublisher,
	citationCollector core.CitationCollector,
	repository core.RunRepository,
) *Service {
	if eventPublisher == nil {
		eventPublisher = core.NoopEventPublisher{}
	}

	return &Service{
		reactRunner:       reactRunner,
		planGraph:         planGraph,
		router:            router,
		eventPublisher:    eventPublisher,
		citationCollector: citationCollector,
		repository:        repository,
		cancel:            make(map[contracts.ID]context.CancelFunc),
	}
}

// Run 执行一次 Agent 运行。
// 流程：尽早注册 cancel 函数 → Router 决策 → 按模式分发 → 执行 → 返回结果。
//
// 注意：cancel 函数必须在路由之前注册，否则在路由耗时期间（如 LLM 路由决策）
// 调用 Cancel() 将无法找到 cancel 函数，导致取消信号无法传播到正在执行的 Agent。
func (s *Service) Run(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	if request.RunID == "" || request.Context.UserID == "" || request.Context.Query == "" {
		return contracts.AgentRunResult{}, fmt.Errorf("run id, user id and query are required")
	}

	// 尽早创建可取消的 context 并注册 cancel 函数。
	// 确保 Cancel() 在路由阶段就能找到对应的 cancel 函数并立即取消执行。
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.mu.Lock()
	s.cancel[request.RunID] = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.cancel, request.RunID)
		s.mu.Unlock()
	}()

	// 1. Router 决策（此时 cancel 函数已注册，Cancel() 可以生效）
	decision, err := s.router.Route(runCtx, request.Context)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// context 已被 Cancel() 取消，不再继续执行
			return contracts.AgentRunResult{}, &contracts.AgentRunError{
				ExecutionMode: contracts.ExecutionReact,
				Err:           context.Canceled,
			}
		}
		// Router 其他失败时降级为 React
		decision = contracts.RouterDecision{
			ExecutionMode: contracts.ExecutionReact,
			ReasonSummary: fmt.Sprintf("路由器失败，降级为 React: %v", err),
			Confidence:    0.0,
			FallbackUsed:  true,
			CreatedAt:     time.Now(),
		}
	}
	// 发布路由决策事件（使用原始 ctx 而非 runCtx，确保事件能正常发布）
	_ = s.eventPublisher.PublishRouterSelected(ctx, request.RunID, decision)

	// 2. 分发前再次检查 context 是否已被取消（防止路由返回后、分发前取消）
	if runCtx.Err() != nil {
		return contracts.AgentRunResult{}, &contracts.AgentRunError{
			ExecutionMode: contracts.ExecutionReact,
			Err:           context.Canceled,
		}
	}

	// 3. 按模式分发（使用可取消的 runCtx）
	switch decision.ExecutionMode {
	case contracts.ExecutionPlanExecute:
		return s.runPlanExecute(ctx, request)
	default:
		return s.runReact(runCtx, request)
	}
}

// runReact 用 ADK ChatModelAgent 执行 ReAct 模式。
// ctx 由 Run() 传入，已经是可取消的 context，无需再次创建。
func (s *Service) runReact(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	if s.reactRunner == nil {
		return contracts.AgentRunResult{}, fmt.Errorf("adk react runner is nil")
	}

	// 在开始执行前检查 context 是否已被取消
	if ctx.Err() != nil {
		_ = s.eventPublisher.PublishRunCancelled(ctx, request.RunID)
		return contracts.AgentRunResult{}, &contracts.AgentRunError{
			ExecutionMode: contracts.ExecutionReact,
			Err:           context.Canceled,
		}
	}

	result, err := s.reactRunner.Run(ctx, request, s.eventPublisher, s.citationCollector)
	if err != nil {
		if ctx.Err() != nil {
			_ = s.eventPublisher.PublishRunCancelled(ctx, request.RunID)
		} else {
			_ = s.eventPublisher.PublishRunFailed(ctx, request.RunID, contracts.ExecutionReact, err)
		}
		return result, &contracts.AgentRunError{ExecutionMode: contracts.ExecutionReact, Err: err}
	}
	if err := s.eventPublisher.PublishRunCompleted(ctx, request.RunID, result); err != nil {
		return contracts.AgentRunResult{}, err
	}
	return result, nil
}

// runPlanExecute 用 PlanExecuteGraph 执行 Plan-Execute 模式。
func (s *Service) runPlanExecute(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	if s.planGraph == nil {
		return contracts.AgentRunResult{}, fmt.Errorf("plan execute graph is nil")
	}

	// 在开始执行前检查 context 是否已被取消
	if ctx.Err() != nil {
		_ = s.eventPublisher.PublishRunCancelled(ctx, request.RunID)
		return contracts.AgentRunResult{}, &contracts.AgentRunError{
			ExecutionMode: contracts.ExecutionPlanExecute,
			Err:           context.Canceled,
		}
	}

	// 发布运行开始事件
	_ = s.eventPublisher.PublishRunStarted(ctx, request.RunID, contracts.ExecutionPlanExecute)

	result, err := s.planGraph.Run(ctx, request)
	if err != nil {
		if ctx.Err() != nil {
			_ = s.eventPublisher.PublishRunCancelled(ctx, request.RunID)
		} else {
			_ = s.eventPublisher.PublishRunFailed(ctx, request.RunID, contracts.ExecutionPlanExecute, err)
		}
		return result, &contracts.AgentRunError{ExecutionMode: contracts.ExecutionPlanExecute, Err: err}
	}
	if err := s.eventPublisher.PublishRunCompleted(ctx, request.RunID, result); err != nil {
		return contracts.AgentRunResult{}, err
	}
	return result, nil
}

// Cancel 停止正在运行的 Agent 执行。
// 注意：必须先取消 context 再更新 DB，确保 Worker 协程能收到取消信号。
// 如果先更新 DB 再取消，在 DB 更新成功后、context 取消前，
// Worker 协程可能已完成执行并误将状态覆盖为 failed。
func (s *Service) Cancel(ctx context.Context, runID, userID contracts.ID) error {
	// 1. 先取消正在执行的 context（无论 DB 更新是否成功都要执行）
	s.mu.Lock()
	cancel := s.cancel[runID]
	delete(s.cancel, runID)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	// 2. 再更新 DB 状态（非关键路径，失败不应阻塞取消操作）
	if s.repository != nil {
		_ = s.repository.Cancel(ctx, runID, userID)
	}

	// 3. 发布取消事件
	if s.eventPublisher != nil {
		_ = s.eventPublisher.PublishRunCancelled(ctx, runID)
	}
	return nil
}

// Retry 重新启动已有的 Agent 执行。
func (s *Service) Retry(ctx context.Context, runID, userID contracts.ID) (contracts.ID, error) {
	if s.repository == nil {
		return "", fmt.Errorf("repository is nil")
	}
	return s.repository.Retry(ctx, runID, userID)
}
