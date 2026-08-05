package contracts

import (
	"context"
	"encoding/json"
	"time"
)

// EventType 标识 Agent 运行生命周期事件的具体类型。
type EventType string

// Agent 运行过程发布的事件类型常量。
const (
	EventRunQueued       EventType = "agent.run.queued"       // 运行已入队
	EventRunStarted      EventType = "agent.run.started"      // 运行已开始
	EventRouterCompleted EventType = "agent.router.completed" // 路由决策完成
	EventStepStarted     EventType = "agent.step.started"     // 某个步骤开始
	EventStepCompleted   EventType = "agent.step.completed"   // 某个步骤完成
	EventToolStarted     EventType = "agent.tool.started"     // 某个工具开始调用
	EventToolCompleted   EventType = "agent.tool.completed"   // 某个工具调用完成
	EventAnswerDelta     EventType = "agent.answer.delta"     // 回答流式增量
	EventRunCompleted    EventType = "agent.run.completed"    // 运行成功完成
	EventRunFailed       EventType = "agent.run.failed"       // 运行失败
	EventRunCancelled    EventType = "agent.run.cancelled"    // 运行被取消
)

// AgentEvent 是 Agent 运行过程的通用事件结构。
type AgentEvent struct {
	EventID   ID              `json:"event_id"`   // 事件唯一 ID
	RunID     ID              `json:"run_id"`     // 所属运行 ID
	EventType EventType       `json:"event_type"` // 事件类型
	Sequence  int64           `json:"sequence"`   // 事件序号，用于顺序消费
	Timestamp time.Time       `json:"timestamp"`  // 事件发生时间
	Data      json.RawMessage `json:"data"`       // 事件附加数据（原始 JSON）
}

// EventPublisher 抽象事件发布能力。
type EventPublisher interface {
	// Publish 发布一个事件。
	Publish(ctx context.Context, event AgentEvent) error
}

// EventSubscriber 抽象事件订阅能力，支持按运行 ID 消费增量事件。
type EventSubscriber interface {
	// Subscribe 订阅指定运行在某个序号之后的事件流。
	Subscribe(ctx context.Context, runID ID, afterSequence int64) (<-chan AgentEvent, error)
}
