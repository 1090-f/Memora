package adkcore

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
)

// SafeToolMiddleware 是安全的工具调用中间件。
//
// 当工具调用失败时，将错误信息转换为正常的 JSON 字符串（ToolResult 格式）返回，
// 而不是向上传播 error 导致 Agent 运行中断。
// 这样 LLM 可以收到工具返回的错误信息并自主决定后续行为（重试、换工具或直接回答）。
//
// 工作原理：
//   - 作为 ChatModelAgentMiddleware 的最外层包装（Handlers 数组的第一个元素）
//   - 拦截 WrapInvokableToolCall / WrapStreamableToolCall 的 endpoint 调用
//   - 如果 endpoint 返回 error，捕获并将错误序列化为 ToolResult JSON 字符串
//   - 同时发布 agent.tool.call.failed 事件，确保前端能够实时刷新工具调用状态
type SafeToolMiddleware struct {
	adk.BaseChatModelAgentMiddleware

	// EventPublisher 发布工具调用失败事件。
	EventPublisher core.EventPublisher
	// RunID 当前运行的 ID。
	RunID contracts.ID
}

// Ensure SafeToolMiddleware implements the interface.
var _ adk.ChatModelAgentMiddleware = (*SafeToolMiddleware)(nil)

// publishToolCallFailed 发布工具调用失败事件。
func (m *SafeToolMiddleware) publishToolCallFailed(ctx context.Context, runID contracts.ID, callID string, toolName, errMsg string) {
	if m.EventPublisher == nil {
		return
	}
	_ = m.EventPublisher.PublishToolCallCompleted(ctx, runID, contracts.ID(callID), toolName, false, errMsg)
}

// WrapInvokableToolCall 包装非流式工具调用，捕获错误并转为字符串返回。
func (m *SafeToolMiddleware) WrapInvokableToolCall(ctx context.Context, endpoint adk.InvokableToolCallEndpoint, tc *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	if tc == nil {
		return endpoint, nil
	}

	wrapped := func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		result, err := endpoint(ctx, argumentsInJSON, opts...)
		if err != nil {
			errMsg := err.Error()

			// 发布工具调用失败事件，确保前端能刷新调用状态
			m.publishToolCallFailed(ctx, m.RunID, tc.CallID, tc.Name, errMsg)

			// 如果已经有结果字符串（如 Executor.InvokeEino 即使失败也会返回含错误信息的 ToolResult JSON），
			// 直接返回结果并清空 error，让 LLM 看到工具返回的错误信息。
			if result != "" {
				return result, nil
			}
			// 否则构造一个含错误信息的 ToolResult
			errorResult := contracts.ToolResult{
				CallID:       contracts.ID(tc.CallID),
				ToolName:     tc.Name,
				Success:      false,
				ErrorCode:    contracts.ErrInternal,
				ErrorMessage: errMsg,
			}
			data, marshalErr := json.Marshal(errorResult)
			if marshalErr != nil {
				return "", nil
			}
			return string(data), nil
		}
		return result, nil
	}

	return wrapped, nil
}

// WrapStreamableToolCall 包装流式工具调用，捕获错误并转为字符串返回。
func (m *SafeToolMiddleware) WrapStreamableToolCall(ctx context.Context, endpoint adk.StreamableToolCallEndpoint, tc *adk.ToolContext) (adk.StreamableToolCallEndpoint, error) {
	if tc == nil {
		return endpoint, nil
	}

	wrapped := func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (*schema.StreamReader[string], error) {
		result, err := endpoint(ctx, argumentsInJSON, opts...)
		if err != nil {
			errMsg := err.Error()

			// 发布工具调用失败事件，确保前端能刷新调用状态
			m.publishToolCallFailed(ctx, m.RunID, tc.CallID, tc.Name, errMsg)

			// 构造一个含错误信息的 ToolResult
			errorResult := contracts.ToolResult{
				CallID:       contracts.ID(tc.CallID),
				ToolName:     tc.Name,
				Success:      false,
				ErrorCode:    contracts.ErrInternal,
				ErrorMessage: errMsg,
			}
			data, marshalErr := json.Marshal(errorResult)
			if marshalErr != nil {
				return schema.StreamReaderFromArray([]string{""}), nil
			}
			return schema.StreamReaderFromArray([]string{string(data)}), nil
		}
		return result, nil
	}

	return wrapped, nil
}
