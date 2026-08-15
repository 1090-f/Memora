package adkcore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/contracts"
)

// ToolAdapter 将内部 tools.Tool 适配为 ADK 的 tool.InvokableTool。
type ToolAdapter struct {
	inner    tools.Tool
	executor *tools.Executor
}

// NewToolAdapter 为已注册的工具构建 ADK 兼容适配器。
func NewToolAdapter(t tools.Tool, executor *tools.Executor) *ToolAdapter {
	return &ToolAdapter{inner: t, executor: executor}
}

// Info 返回工具的 schema.ToolInfo。
func (a *ToolAdapter) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return a.inner.Info(ctx)
}

// InvokableRun 以 JSON 字符串形式执行工具并返回结果。
func (a *ToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	spec := a.inner.Spec()

	call := contracts.ToolCall{
		ToolName:  spec.Name,
		Arguments: json.RawMessage(argumentsInJSON),
	}

	if a.executor != nil {
		return a.executor.InvokeContext(ctx, extractToolContext(ctx), call)
	}

	toolCtx := extractToolContext(ctx)
	result, err := a.inner.Execute(ctx, toolCtx, call)
	if err != nil {
		return "", fmt.Errorf("tool %q execute: %w", spec.Name, err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("tool %q marshal result: %w", spec.Name, err)
	}
	return string(data), nil
}

// StreamableRun 实现流式工具调用。
func (a *ToolAdapter) StreamableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (*schema.StreamReader[string], error) {
	result, err := a.InvokableRun(ctx, argumentsInJSON, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]string{result}), nil
}

// toolContextKey 是 context 中存储 ToolContext 的键。
type toolContextKey struct{}

// WithToolContext 将 ToolContext 注入 context。
func WithToolContext(ctx context.Context, tc contracts.ToolContext) context.Context {
	return context.WithValue(ctx, toolContextKey{}, tc)
}

// extractToolContext 从 context 中提取 ToolContext。
func extractToolContext(ctx context.Context) contracts.ToolContext {
	if tc, ok := ctx.Value(toolContextKey{}).(contracts.ToolContext); ok {
		return tc
	}
	return contracts.ToolContext{}
}
