package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// AgentCore 是 contracts.AgentRunService 的别名，不新增同义业务接口。
type AgentCore = contracts.AgentRunService

// PlanRunner 执行 plan_execute 模式，具体 Planner/Executor/Reviewer 由上层注入。
type PlanRunner interface {
	Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error)
}

// RunRepository 只定义 Service 生命周期需要的持久化动作。
// 具体 SQL、用户归属和并发条件更新由 internal/repository 实现。
type RunRepository interface {
	Cancel(ctx context.Context, runID, userID contracts.ID) error
	Retry(ctx context.Context, runID, userID contracts.ID) (contracts.ID, error)
}

// Service 编排路由、执行器、事件和取消控制。
type Service struct {
	runner     ReactRunner
	planRunner PlanRunner
	router     contracts.Router
	repository RunRepository
	events     EventPublisher

	mu     sync.Mutex
	cancel map[contracts.ID]context.CancelFunc
}

// NewService 创建 Agent Core Service，依赖均通过构造函数注入。
func NewService(runner ReactRunner, router contracts.Router, repository RunRepository, events EventPublisher) *Service {
	if events == nil {
		events = NoopEventPublisher{}
	}
	return &Service{runner: runner, router: router, repository: repository, events: events, cancel: make(map[contracts.ID]context.CancelFunc)}
}

// SetPlanRunner 注入 Plan-Execute 执行器。
func (s *Service) SetPlanRunner(runner PlanRunner) { s.planRunner = runner }

func (s *Service) Run(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	if request.RunID == "" || request.Context.UserID == "" || request.Context.Query == "" {
		return contracts.AgentRunResult{}, newCoreError(contracts.ErrInvalidArgument, fmt.Errorf("run id, user id and query are required"))
	}
	if s == nil || s.runner == nil {
		return contracts.AgentRunResult{}, newCoreError(contracts.ErrInternal, ErrExecutionDependency)
	}
	cfg := withDefaults(request.Config)
	startedAt := now()
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.MaxRunSeconds)*time.Second)
	defer cancel()
	mode := contracts.ExecutionReact
	runner := s.runner
	if s.router != nil {
		decision, err := s.router.Route(runCtx, request.Context)
		if err == nil {
			if decision.ExecutionMode == contracts.ExecutionPlanExecute && s.planRunner != nil {
				mode = contracts.ExecutionPlanExecute
				runner = s.planRunner
			}
			_ = s.events.PublishRouterSelected(runCtx, request.RunID, decision)
		}
	}
	if err := s.events.PublishRunStarted(runCtx, request.RunID, mode); err != nil {
		return contracts.AgentRunResult{}, err
	}
	s.mu.Lock()
	s.cancel[request.RunID] = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.cancel, request.RunID)
		s.mu.Unlock()
		cancel()
	}()

	output, err := runner.Run(runCtx, request.Context, cfg)
	endedAt := now()
	if err != nil {
		if runCtx.Err() != nil || ctx.Err() != nil {
			_ = s.events.PublishRunCancelled(context.Background(), request.RunID)
		} else {
			_ = s.events.PublishRunFailed(context.Background(), request.RunID, err)
		}
		return contracts.AgentRunResult{}, err
	}
	result := output.Result(request.RunID, mode, "", startedAt, endedAt)
	if err := s.events.PublishRunCompleted(context.Background(), request.RunID, result); err != nil {
		return contracts.AgentRunResult{}, err
	}
	return result, nil
}

func (s *Service) Cancel(ctx context.Context, runID, userID contracts.ID) error {
	if s == nil || runID == "" || userID == "" {
		return newCoreError(contracts.ErrInvalidArgument, fmt.Errorf("run id and user id are required"))
	}
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
	return nil
}

func (s *Service) Retry(ctx context.Context, runID, userID contracts.ID) (contracts.ID, error) {
	if s == nil || s.repository == nil {
		return "", newCoreError(contracts.ErrServiceUnavailable, ErrPersistenceUnavailable)
	}
	if runID == "" || userID == "" {
		return "", newCoreError(contracts.ErrInvalidArgument, fmt.Errorf("run id and user id are required"))
	}
	return s.repository.Retry(ctx, runID, userID)
}

func now() time.Time { return time.Now() }
