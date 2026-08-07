package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

// Executor 是 contracts.ToolExecutor 的安全执行实现。
type Executor struct{ registry *Registry }

func NewExecutor(registry *Registry) *Executor { return &Executor{registry: registry} }

func (e *Executor) Execute(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	value, ok := e.registry.find(call.ToolName)
	if !ok {
		return failure(call, contracts.ErrResourceNotFound, "tool is not registered")
	}
	spec := value.Spec()
	if !spec.Enabled {
		return failure(call, contracts.ErrInvalidState, "tool is disabled")
	}
	if !spec.ReadOnly {
		return failure(call, contracts.ErrWriteMCPToolForbidden, "write tool is forbidden")
	}
	if !contains(toolContext.AllowedToolNames, call.ToolName) {
		return failure(call, contracts.ErrForbidden, "tool is not allowed")
	}
	if spec.NetworkRequired && !toolContext.NetworkEnabled {
		return failure(call, contracts.ErrNetworkDisabled, "network tool is disabled")
	}
	if len(call.Arguments) > 64*1024 {
		return failure(call, contracts.ErrPayloadTooLarge, "tool arguments are too large")
	}
	if !json.Valid(call.Arguments) {
		return failure(call, contracts.ErrInvalidArgument, "tool arguments must be valid JSON")
	}

	callContext := ctx
	if spec.Timeout > 0 {
		var cancel context.CancelFunc
		callContext, cancel = context.WithTimeout(ctx, spec.Timeout)
		defer cancel()
	}
	result, err := value.Execute(callContext, toolContext, call)
	if err != nil {
		return result, err
	}
	return truncateResult(result, toolContext.MaxResultBytes), nil
}

func (e *Executor) InvokeEino(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (string, error) {
	result, err := e.Execute(ctx, toolContext, call)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal tool result: %w", err)
	}
	return string(data), nil
}

func (e *Executor) InvokeContext(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (string, error) {
	return e.InvokeEino(withToolContext(ctx, toolContext), toolContext, call)
}

func failure(call contracts.ToolCall, code contracts.ErrorCode, message string) (contracts.ToolResult, error) {
	result := contracts.ToolResult{CallID: call.CallID, ToolName: call.ToolName, Success: false, ErrorCode: code, ErrorMessage: message}
	return result, fmt.Errorf("%s: %s", code, message)
}

func truncateResult(result contracts.ToolResult, maxBytes int) contracts.ToolResult {
	if maxBytes <= 0 || len(result.Text) <= maxBytes {
		return result
	}
	result.Text = result.Text[:maxBytes]
	result.Truncated = true
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var _ contracts.ToolExecutor = (*Executor)(nil)
