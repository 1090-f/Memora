package contracts

import (
	"context"
	"encoding/json"
	"time"
)

// EventType 表示 Agent 事件的类型。
type EventType string

// Agent 运行过程发布的事件类型常量。
const (
	// EventRunQueued 表示 Agent 运行已排队等待执行。
	EventRunQueued EventType = "agent.run.queued"
	// EventRunStarted 表示 Agent 运行已开始执行。
	EventRunStarted EventType = "agent.run.started"
	// EventRouterCompleted 表示路由器已完成执行模式的确定。
	EventRouterCompleted EventType = "agent.router.completed"
	// EventStepStarted 表示计划步骤已开始执行。
	EventStepStarted EventType = "agent.step.started"
	// EventStepCompleted 表示计划步骤已完成执行。
	EventStepCompleted EventType = "agent.step.completed"
	// EventToolStarted 表示工具调用已开始。
	EventToolStarted EventType = "agent.tool.started"
	// EventToolCompleted 表示工具调用已完成。
	EventToolCompleted EventType = "agent.tool.completed"
	// EventAnswerDelta 表示已收到流式回答增量。
	EventAnswerDelta EventType = "agent.answer.delta"
	// EventRunCompleted 表示 Agent 运行已成功完成。
	EventRunCompleted EventType = "agent.run.completed"
	// EventRunFailed 表示 Agent 运行失败。
	EventRunFailed EventType = "agent.run.failed"
	// EventRunCancelled 表示 Agent 运行已取消。
	EventRunCancelled EventType = "agent.run.cancelled"
	// EventReactRoundStarted 表示 ReAct 轮次开始。
	EventReactRoundStarted EventType = "agent.react.round.started"
	// EventReactRoundCompleted 表示 ReAct 轮次完成。
	EventReactRoundCompleted EventType = "agent.react.round.completed"
	// EventToolCallFailed 表示工具调用失败（区别于 EventToolCompleted 的完整生命周期）。
	EventToolCallFailed EventType = "agent.tool.call.failed"
)

// AgentEvent 表示在 Agent 执行运行期间发出的事件。
type AgentEvent struct {
	EventID   ID              `json:"event_id"`   // 事件唯一 ID
	RunID     ID              `json:"run_id"`     // 所属运行 ID
	EventType EventType       `json:"event_type"` // 事件类型
	Sequence  int64           `json:"sequence"`   // 事件序号，用于顺序消费
	Timestamp time.Time       `json:"timestamp"`  // 事件发生时间
	Data      json.RawMessage `json:"data"`       // 事件附加数据（原始 JSON）
}

// EventPublisher 发布 Agent 事件以供实时消费。
type EventPublisher interface {
	// Publish 将 Agent 事件发送给订阅者。
	Publish(ctx context.Context, event AgentEvent) error
}

// EventSubscriber 订阅 Agent 事件以供实时消费。
type EventSubscriber interface {
	// Subscribe 返回一个通道，接收指定运行在指定序列号之后的事件。
	Subscribe(ctx context.Context, runID ID, afterSequence int64) (<-chan AgentEvent, error)
}
