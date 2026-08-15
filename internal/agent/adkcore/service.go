package adkcore

import (
	"context"
	"fmt"
	"sync"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
)

// Service 是基于 Eino ADK 实现的 AgentRunService。
// 它使用 ADK ChatModelAgent 替代了原有的自实现 ReAct 循环。
// 与现有的 core.Service 接口完全兼容。
type Service struct {
	reactRunner       *ADKReactRunner
	eventPublisher    core.EventPublisher
	citationCollector core.CitationCollector
	repository        core.RunRepository

	mu     sync.Mutex
	cancel map[contracts.ID]context.CancelFunc
}

// NewService 创建 ADK 驱动的 Agent 运行服务。
func NewService(
	reactRunner *ADKReactRunner,
	eventPublisher core.EventPublisher,
	citationCollector core.CitationCollector,
	repository core.RunRepository,
) *Service {
	if eventPublisher == nil {
		eventPublisher = core.NoopEventPublisher{}
	}
	return &Service{
		reactRunner:       reactRunner,
		eventPublisher:    eventPublisher,
		citationCollector: citationCollector,
		repository:        repository,
		cancel:            make(map[contracts.ID]context.CancelFunc),
	}
}

// Run 执行一次 Agent 运行。
// 与 contracts.AgentRunService.Run 接口兼容。
func (s *Service) Run(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	if request.RunID == "" || request.Context.UserID == "" || request.Context.Query == "" {
		return contracts.AgentRunResult{}, fmt.Errorf("run id, user id and query are required")
	}
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
