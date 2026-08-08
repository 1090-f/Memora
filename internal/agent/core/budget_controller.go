package core

import (
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// BudgetController 统一限制 Agent 的轮次、步骤、工具调用、时长和 Token 用量。
type BudgetController interface {
	CheckReactRounds(rounds int) error
	CheckPlanSteps(steps int) error
	CheckToolCalls(calls int) error
	CheckRunDuration(startedAt time.Time) error
	CheckTokenUsage(usage contracts.TokenUsage) error
}

// DefaultBudgetController 使用 AgentConfig 执行系统侧预算校验。
type DefaultBudgetController struct{ Config contracts.AgentConfig }

func (b DefaultBudgetController) CheckReactRounds(rounds int) error {
	if rounds > b.Config.MaxReactRounds {
		return fmt.Errorf("%w: react rounds %d/%d", ErrBudgetExceeded, rounds, b.Config.MaxReactRounds)
	}
	return nil
}

func (b DefaultBudgetController) CheckPlanSteps(steps int) error {
	if steps > b.Config.MaxPlanSteps {
		return fmt.Errorf("%w: plan steps %d/%d", ErrBudgetExceeded, steps, b.Config.MaxPlanSteps)
	}
	return nil
}

func (b DefaultBudgetController) CheckToolCalls(calls int) error {
	if calls > b.Config.MaxToolCalls {
		return fmt.Errorf("%w: tool calls %d/%d", ErrBudgetExceeded, calls, b.Config.MaxToolCalls)
	}
	return nil
}

func (b DefaultBudgetController) CheckRunDuration(startedAt time.Time) error {
	if time.Since(startedAt) > time.Duration(b.Config.MaxRunSeconds)*time.Second {
		return fmt.Errorf("%w: run duration exceeds %d seconds", ErrBudgetExceeded, b.Config.MaxRunSeconds)
	}
	return nil
}

func (b DefaultBudgetController) CheckTokenUsage(usage contracts.TokenUsage) error {
	if usage.TotalTokens < 0 || usage.InputTokens < 0 || usage.OutputTokens < 0 {
		return fmt.Errorf("%w: token usage cannot be negative", ErrBudgetExceeded)
	}
	if usage.TotalTokens != 0 && usage.InputTokens+usage.OutputTokens > usage.TotalTokens {
		return fmt.Errorf("%w: token usage is inconsistent", ErrBudgetExceeded)
	}
	return nil
}
