package core

import (
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

// RunState 表示 Agent Run 的生命周期状态。
type RunState string

const (
	RunQueued    RunState = "queued"
	RunRunning   RunState = "running"
	RunCompleted RunState = "completed"
	RunFailed    RunState = "failed"
	RunCancelled RunState = "cancelled"
)

// ValidTransition 校验状态机允许的状态迁移。
func ValidTransition(from, to RunState) bool {
	switch from {
	case RunQueued:
		return to == RunRunning || to == RunCancelled
	case RunRunning:
		return to == RunCompleted || to == RunFailed || to == RunCancelled
	default:
		return false
	}
}

func validateRunState(runID contracts.ID, from, to RunState) error {
	if !ValidTransition(from, to) {
		return fmt.Errorf("run %s: invalid state transition %s -> %s", runID, from, to)
	}
	return nil
}
