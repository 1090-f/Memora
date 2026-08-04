package contracts

import (
	"context"
	"encoding/json"
)

// ToolContext 提供工具调用的执行上下文。
type ToolContext struct {
	UserID           ID       `json:"user_id"`
	KnowledgeBaseID  ID       `json:"knowledge_base_id"`
	AgentRunID       ID       `json:"agent_run_id"`
	PlanStepID       ID       `json:"plan_step_id,omitempty"`
	ReactRound       int      `json:"react_round,omitempty"`
	AllowedToolNames []string `json:"allowed_tool_names"`
	MaxResultBytes   int      `json:"max_result_bytes"`
}

// ToolCall 表示使用参数调用特定工具的请求。
type ToolCall struct {
	CallID    ID              `json:"call_id"`
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult 表示工具调用的结果。
type ToolResult struct {
	CallID         ID              `json:"call_id"`
	ToolName       string          `json:"tool_name"`
	Text           string          `json:"text,omitempty"`
	StructuredData json.RawMessage `json:"structured_data,omitempty"`
	Citations      []Citation      `json:"citations,omitempty"`
	Truncated      bool            `json:"truncated"`
	Success        bool            `json:"success"`
	ErrorCode      ErrorCode       `json:"error_code,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
}

// ToolExecutor 定义执行工具调用的接口。
type ToolExecutor interface {
	// Execute 在给定上下文中运行工具调用并返回结果。
	Execute(ctx context.Context, toolContext ToolContext, call ToolCall) (ToolResult, error)
}

// ToolRegistry 提供可用工具的信息。
type ToolRegistry interface {
	// Has 检查指定名称的工具是否存在。
	Has(name string) bool
	// Names 返回所有可用工具的名称列表。
	Names() []string
}
