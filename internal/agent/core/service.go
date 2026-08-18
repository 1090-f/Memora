package core

import (
	"context"

	"github.com/1090-f/Memora/internal/contracts"
)

// AgentCore 是 contracts.AgentRunService 的别名，不新增同义业务接口。
type AgentCore = contracts.AgentRunService

// RunRepository 只定义 Agent Service 生命周期需要的持久化动作。
// 具体 SQL、用户归属和并发条件更新由 internal/repository 实现。
type RunRepository interface {
	Cancel(ctx context.Context, runID, userID contracts.ID, executionMode string) error
	Retry(ctx context.Context, runID, userID contracts.ID) (contracts.ID, error)
}
