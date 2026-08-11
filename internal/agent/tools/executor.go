package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
)

const maxToolArgumentBytes = 64 * 1024

// ToolAvailabilityChecker 在工具真正执行前动态复核工具是否仍可用。
// 内置工具的启用状态由注册时快照固化即可；MCP 工具的启用状态可在前端动态修改，
// 因此需要在每次调用前向数据层复核 Server 与 Tool 的启用状态（第二层拦截）。
// 该接口由上层（如 MCP 服务）注入实现，工具模块自身不依赖数据层。
type ToolAvailabilityChecker interface {
	CheckToolAvailable(ctx context.Context, userID contracts.ID, spec contracts.ToolSpec) (bool, error)
}

// Executor 是 contracts.ToolExecutor 的唯一安全执行入口。
type Executor struct {
	registry  *Registry
	available ToolAvailabilityChecker // 可选的调用前动态可用性检查器
}

// NewExecutor 创建绑定指定注册表的执行器。
func NewExecutor(registry *Registry) *Executor { return &Executor{registry: registry} }

// SetAvailabilityChecker 注入调用前动态可用性检查器（如 MCP 工具的启用状态复核）。
// 内置工具可省略；注入后 Executor 会对带 SourceID 的工具（MCP）在真正执行前动态复检。
func (e *Executor) SetAvailabilityChecker(checker ToolAvailabilityChecker) {
	e.available = checker
}

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
	// 第二层拦截：MCP 工具启用状态可在前端动态修改，注册时快照已不可信，
	// 因此对带 SourceID 的 MCP 工具在真正执行前向数据层动态复核一次。
	// 内置工具 SourceID 为空，跳过动态检查，仅依赖注册时的静态快照。
	if e.available != nil && spec.SourceID != "" {
		available, err := e.available.CheckToolAvailable(ctx, toolContext.UserID, spec)
		if err != nil {
			return failure(call, contracts.ErrInternal, "failed to check tool availability")
		}
		if !available {
			return failure(call, contracts.ErrMCPToolDisabled, "tool is no longer available")
		}
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
