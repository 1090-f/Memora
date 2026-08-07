package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// Tool 是工具模块的最小执行能力，由内置工具和 MCP 适配器实现。
type Tool interface {
	tool.InvokableTool
	Spec() contracts.ToolSpec
	Execute(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error)
}

type contextKey struct{}
type argumentError struct{ err error }

func (e *argumentError) Error() string { return e.err.Error() }
func (e *argumentError) Unwrap() error { return e.err }

func withToolContext(ctx context.Context, toolContext contracts.ToolContext) context.Context {
	return context.WithValue(ctx, contextKey{}, toolContext)
}

func toolContextFrom(ctx context.Context) (contracts.ToolContext, bool) {
	value, ok := ctx.Value(contextKey{}).(contracts.ToolContext)
	return value, ok
}

// parseArguments 只负责反序列化模型 JSON；业务工具随后执行必填字段和长度校验。
func parseArguments(input string, target any) error {
	if err := json.Unmarshal([]byte(input), target); err != nil {
		return &argumentError{err: fmt.Errorf("invalid tool arguments: %w", err)}
	}
	return nil
}

// invalidArgument 将工具边界上的参数校验失败标记为参数错误，避免误伤下游服务错误码。
func invalidArgument(message string) error {
	return &argumentError{err: fmt.Errorf("invalid tool arguments: %s", message)}
}

func info(name, description string, parameters *schema.ParamsOneOf) *schema.ToolInfo {
	return &schema.ToolInfo{Name: name, Desc: description, ParamsOneOf: parameters}
}

var _ tool.InvokableTool = (Tool)(nil)
