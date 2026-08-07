package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

const maxToolArgumentBytes = 64 * 1024

// Executor 是 contracts.ToolExecutor 的唯一安全执行入口。
type Executor struct{ registry *Registry }

// NewExecutor 创建绑定指定注册表的执行器。
func NewExecutor(registry *Registry) *Executor { return &Executor{registry: registry} }

// Execute 统一执行前置授权、参数大小/合法性、超时和结果归一化检查。
func (e *Executor) Execute(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	if e == nil || e.registry == nil {
		return failure(call, contracts.ErrInvalidState, "tool registry is unavailable")
	}
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
	if len(call.Arguments) > maxToolArgumentBytes {
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
		if result.CallID == "" {
			result.CallID = call.CallID
		}
		if result.ToolName == "" {
			result.ToolName = call.ToolName
		}
		if result.ErrorCode == "" {
			result.ErrorCode = contracts.ErrInternal
		}
		result.Success = false
		return result, err
	}
	return truncateResult(result, toolContext.MaxResultBytes), nil
}

// InvokeEino 将标准 ToolResult 序列化为模型工具调用需要的 JSON 字符串。
func (e *Executor) InvokeEino(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (string, error) {
	result, err := e.Execute(ctx, toolContext, call)
	data, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return "", fmt.Errorf("marshal tool result: %w", marshalErr)
	}
	if err != nil {
		return string(data), err
	}
	return string(data), nil
}

// InvokeContext 是将服务端 ToolContext 注入 Eino 上下文的便捷入口。
func (e *Executor) InvokeContext(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (string, error) {
	return e.InvokeEino(withToolContext(ctx, toolContext), toolContext, call)
}

func failure(call contracts.ToolCall, code contracts.ErrorCode, message string) (contracts.ToolResult, error) {
	result := contracts.ToolResult{CallID: call.CallID, ToolName: call.ToolName, Success: false, ErrorCode: code, ErrorMessage: message}
	return result, fmt.Errorf("%s: %s", code, message)
}

// truncateResult 同时限制 Text 和 StructuredData，不能只限制模型看到的文本字段。
func truncateResult(result contracts.ToolResult, maxBytes int) contracts.ToolResult {
	if maxBytes <= 0 {
		return result
	}
	if len(result.Text) > maxBytes {
		result.Text = result.Text[:maxBytes]
		result.Truncated = true
	}
	if len(result.StructuredData) > maxBytes {
		result.StructuredData = append(json.RawMessage(nil), result.StructuredData[:maxBytes]...)
		result.Truncated = true
	}
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
