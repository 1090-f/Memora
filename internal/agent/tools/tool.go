package tools

import (
	"context"
	"encoding/json"

	"github.com/1090-f/Memora/internal/contracts"
)

// Tool 定义了一个可被 Agent 调用的工具。
// 每个工具都提供一份描述其用途的参数规范（Spec），
// 并在 run 中根据调用参数实际执行业务逻辑。
type Tool interface {
	// Spec 返回该工具的元数据描述（名称、是否只读、超时等）。
	Spec() contracts.ToolSpec
	// Run 执行工具的调用：传入调用上下文与 JSON 参数，返回工具结果。
	Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error)
}
