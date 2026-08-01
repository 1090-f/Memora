package contracts

import (
	"context"
	"encoding/json"
)

type ToolContext struct {
	UserID           ID       `json:"user_id"`
	KnowledgeBaseID  ID       `json:"knowledge_base_id"`
	AgentRunID       ID       `json:"agent_run_id"`
	PlanStepID       ID       `json:"plan_step_id,omitempty"`
	ReactRound       int      `json:"react_round,omitempty"`
	AllowedToolNames []string `json:"allowed_tool_names"`
	MaxResultBytes   int      `json:"max_result_bytes"`
}

type ToolCall struct {
	CallID    ID              `json:"call_id"`
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
}

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

type ToolExecutor interface {
	Execute(ctx context.Context, toolContext ToolContext, call ToolCall) (ToolResult, error)
}

type ToolRegistry interface {
	Has(name string) bool
	Names() []string
}
