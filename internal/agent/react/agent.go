// Package react 提供基于 Eino 编排的 ReAct Agent。
package react

import (
	"context"

	"github.com/1090-f/Memora/internal/contracts"
)

// Agent 定义 ReAct Agent 的完整和流式执行能力。
type Agent interface {
	Run(ctx context.Context, agentContext contracts.AgentContext, config contracts.AgentConfig) (contracts.AgentRunResult, error)
	Stream(ctx context.Context, agentContext contracts.AgentContext, config contracts.AgentConfig) (<-chan contracts.AgentEvent, error)
}

// CitationCollector 收集工具返回的真实引用，避免 Agent 自行生成引用。
type CitationCollector interface {
	Add(citations []contracts.Citation)
	Get() []contracts.Citation
	Reset()
}

// Dependencies 是 ReAct 编排所需的外部依赖，均由上层注入。
type Dependencies struct {
	ChatModel         contracts.ChatModel
	ToolRegistry      contracts.ToolRegistry
	ToolExecutor      contracts.ToolExecutor
	EventPublisher    contracts.EventPublisher
	CitationCollector CitationCollector
}

// NewAgent 创建 ReAct Agent。
func NewAgent(dependencies Dependencies) Agent {
	if dependencies.CitationCollector == nil {
		dependencies.CitationCollector = newCitationCollector()
	}
	return &agent{dependencies: dependencies}
}

type agent struct {
	dependencies Dependencies
}
