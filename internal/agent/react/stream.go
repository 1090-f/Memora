package react

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// Stream 执行一次 ReAct Run，并将安全的生命周期事件通过通道返回。
// 完整模型流由上层 ChatModel 适配器负责；本层不暴露隐藏推理内容。
func (a *agent) Stream(ctx context.Context, agentContext contracts.AgentContext, config contracts.AgentConfig) (<-chan contracts.AgentEvent, error) {
	if err := validate(agentContext, a.dependencies); err != nil {
		return nil, err
	}
	events := make(chan contracts.AgentEvent, 4)
	go func() {
		defer close(events)
		sequence := int64(0)
		emit := func(eventType contracts.EventType, data any) {
			sequence++
			raw, _ := json.Marshal(data)
			events <- contracts.AgentEvent{RunID: agentContext.RunID, EventType: eventType, Sequence: sequence, Timestamp: time.Now(), Data: raw}
		}
		emit(contracts.EventRunStarted, map[string]any{"execution_mode": "react"})
		result, err := a.Run(ctx, agentContext, config)
		if err != nil {
			if ctx.Err() != nil {
				emit(contracts.EventRunCancelled, map[string]any{"error_code": "CANCELLED"})
			} else {
				emit(contracts.EventRunFailed, map[string]any{"error_code": errorCode(err)})
			}
			return
		}
		if result.FinalResult != "" {
			emit(contracts.EventAnswerDelta, map[string]any{"delta": result.FinalResult})
		}
		emit(contracts.EventRunCompleted, map[string]any{"citations": result.Citations, "usage": result.Usage})
	}()
	return events, nil
}

func errorCode(err error) string {
	var value *agentError
	if errors.As(err, &value) {
		return string(value.code)
	}
	return string(contracts.ErrInternal)
}
