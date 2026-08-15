package adkcore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
)

// Service 是基于 Eino ADK ChatModelAgent 和 Router 的 AgentRunService。
// 每次 Run() 先由 Router 决定 execution mode，再分发到 ReactRunner 或 PlanRunner。
type Service struct {
	reactRunner       *ADKReactRunner
	planRunner        core.PlanRunner
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
	planRunner core.PlanRunner,
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
		planRunner:        planRunner,
		router:            router,
		eventPublisher:    eventPublisher,
		citationCollector: citationCollector,
		repository:        repository,
		cancel:            make(map[contracts.ID]context.CancelFunc),
	}
}

// Run 执行一次 Agent 运行。
// 流程：Router 决策 → 按模式分发 → 执行 → 返回结果。
func (s *Service) Run(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	if request.RunID == "" || request.Context.UserID == "" || request.Context.Query == "" {
		return contracts.AgentRunResult{}, fmt.Errorf("run id, user id and query are required")
	}

	// 1. Router 决策
	decision, err := s.router.Route(ctx, request.Context)
	if err != nil {
		// Router 失败时降级为 React
		decision = contracts.RouterDecision{
			ExecutionMode: contracts.ExecutionReact,
			ReasonSummary: fmt.Sprintf("路由器失败，降级为 React: %v", err),
			Confidence:    0.0,
			FallbackUsed:  true,
			CreatedAt:     time.Now(),
		}
	}
	// 发布路由决策事件
	_ = s.eventPublisher.PublishRouterSelected(ctx, request.RunID, decision)

	// 2. 按模式分发
	switch decision.ExecutionMode {
	case contracts.ExecutionPlanExecute:
		return s.runPlan(ctx, request)
	default:
		return s.runReact(ctx, request)
	}
}

// runReact 用 ADK ChatModelAgent 执行 ReAct 模式。
func (s *Service) runReact(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	if s.reactRunner == nil {
		return contracts.AgentRunResult{}, fmt.Errorf("adk react runner is nil")
	}

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

	result, err := s.reactRunner.Run(runCtx, request, s.eventPublisher, s.citationCollector)
	if err != nil {
		if runCtx.Err() != nil || ctx.Err() != nil {
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

// runPlan 用 PlanRunner 执行 Plan-Execute 模式。
func (s *Service) runPlan(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	if s.planRunner == nil {
		// PlanRunner 未注入时降级为 React
		_ = s.eventPublisher.PublishRouterSelected(ctx, request.RunID, contracts.RouterDecision{
			ExecutionMode: contracts.ExecutionReact,
			ReasonSummary: "PlanRunner 未注入，降级为 ReAct",
			Confidence:    0.0,
			FallbackUsed:  true,
			CreatedAt:     time.Now(),
		})
		return s.runReact(ctx, request)
	}

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

	startedAt := time.Now().UTC()
	output, err := s.planRunner.Run(runCtx, request.Context, request.Config)
	if err != nil {
		if runCtx.Err() != nil || ctx.Err() != nil {
			_ = s.eventPublisher.PublishRunCancelled(ctx, request.RunID)
		} else {
			_ = s.eventPublisher.PublishRunFailed(ctx, request.RunID, contracts.ExecutionPlanExecute, err)
		}
		return output.Result(request.RunID, contracts.ExecutionPlanExecute, startedAt, time.Now().UTC()),
			&contracts.AgentRunError{ExecutionMode: contracts.ExecutionPlanExecute, Err: err}
	}

	result := output.Result(request.RunID, contracts.ExecutionPlanExecute, startedAt, time.Now().UTC())
	if err := s.eventPublisher.PublishRunCompleted(ctx, request.RunID, result); err != nil {
		return contracts.AgentRunResult{}, err
	}
	return result, nil
}

// Cancel 停止正在运行的 Agent 执行。
func (s *Service) Cancel(ctx context.Context, runID, userID contracts.ID) error {
	if s.repository != nil {
		if err := s.repository.Cancel(ctx, runID, userID); err != nil {
			return err
		}
	}

	s.mu.Lock()
	cancel := s.cancel[runID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}

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
