package core

import (
	"context"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// RunController 负责运行的停止、取消和超时控制。
type RunController interface {
	Stop(ctx context.Context, runID contracts.ID) error
	Cancel(ctx context.Context, runID, userID contracts.ID) error
	SetTimeout(ctx context.Context, runID contracts.ID, timeout time.Duration) error
}

type runController struct {
	mu       sync.Mutex
	cancel   map[contracts.ID]context.CancelFunc
	timeouts map[contracts.ID]time.Duration
}

// NewRunController 创建进程内运行控制器；持久化取消由 Service 的 Repository 负责。
func NewRunController() RunController {
	return &runController{cancel: make(map[contracts.ID]context.CancelFunc), timeouts: make(map[contracts.ID]time.Duration)}
}

func (c *runController) bind(runID contracts.ID, cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancel[runID] = cancel
}

func (c *runController) Stop(_ context.Context, runID contracts.ID) error {
	c.mu.Lock()
	cancel := c.cancel[runID]
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (c *runController) Cancel(ctx context.Context, runID, _ contracts.ID) error {
	return c.Stop(ctx, runID)
}

func (c *runController) SetTimeout(_ context.Context, runID contracts.ID, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeouts[runID] = timeout
	return nil
}

var _ RunController = (*runController)(nil)
