package tools

import (
	"context"
	"encoding/json"

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

func withToolContext(ctx context.Context, toolContext contracts.ToolContext) context.Context {
	return context.WithValue(ctx, contextKey{}, toolContext)
}

func toolContextFrom(ctx context.Context) (contracts.ToolContext, bool) {
	value, ok := ctx.Value(contextKey{}).(contracts.ToolContext)
	return value, ok
}

func parseArguments(input string, target any) error {
	if err := json.Unmarshal([]byte(input), target); err != nil {
		return fmtInvalidArgument(err)
	}
	return nil
}

func info(name, description string, parameters *schema.ParamsOneOf) *schema.ToolInfo {
	return &schema.ToolInfo{Name: name, Desc: description, ParamsOneOf: parameters}
}
