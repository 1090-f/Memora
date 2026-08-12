package core

import (
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// RunOutput 是执行器向 Service 返回的最小结果集合。
type RunOutput struct {
	FinalResult     string
	Citations       []contracts.Citation
	Usage           contracts.TokenUsage
	Summary         string
	KnowledgeStatus string // 知识充分性状态（从 AgentContext 传递）
}

// Result 将执行输出归一化为对外 AgentRunResult。
func (o RunOutput) Result(runID contracts.ID, mode contracts.ExecutionMode, startedAt, endedAt time.Time) contracts.AgentRunResult {
	return contracts.AgentRunResult{RunID: runID, ExecutionMode: mode, KnowledgeStatus: o.KnowledgeStatus, FinalResult: o.FinalResult, Citations: o.Citations, Usage: o.Usage, StartedAt: startedAt, EndedAt: endedAt}
}
