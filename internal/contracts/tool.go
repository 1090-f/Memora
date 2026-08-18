package contracts

import (
	"context"
	"encoding/json"
	"time"
)

// ToolContext 提供工具调用的执行上下文。
type ToolContext struct {
	UserID           ID       `json:"user_id"`               // 发起工具调用的用户
	KnowledgeBaseID  ID       `json:"knowledge_base_id"`     // 关联的知识库
	AgentRunID       ID       `json:"agent_run_id"`          // 所属的 Agent 运行 ID
	ReactRound       int      `json:"react_round,omitempty"` // 可选：ReAct 当前轮次
	AllowedToolNames []string `json:"allowed_tool_names"`    // 允许调用的工具名白名单
	NetworkEnabled   bool     `json:"network_enabled"`       // 是否允许联网工具
	MaxResultBytes   int      `json:"max_result_bytes"`      // 工具结果最大字节数限制
	ChatModelID      string   `json:"chat_model_id"`         // 用于无工具步骤的 LLM 推理
}

// ToolCall 表示使用参数调用特定工具的请求。
type ToolCall struct {
	CallID    ID              `json:"call_id"`   // 调用 ID，用于关联结果
	ToolName  string          `json:"tool_name"` // 工具名称
	Arguments json.RawMessage `json:"arguments"` // 工具参数（原始 JSON）
}

// ToolResult 表示工具调用的结果。
type ToolResult struct {
	CallID         ID              `json:"call_id"`                   // 对应的调用 ID
	ToolName       string          `json:"tool_name"`                 // 工具名称
	Text           string          `json:"text,omitempty"`            // 文本形式的结果
	StructuredData json.RawMessage `json:"structured_data,omitempty"` // 可选：结构化数据
	Citations      []Citation      `json:"citations,omitempty"`       // 可选：结果引用的来源
	Truncated      bool            `json:"truncated"`                 // 结果是否因超限被截断
	Success        bool            `json:"success"`                   // 是否成功
	ErrorCode      ErrorCode       `json:"error_code,omitempty"`      // 失败时的错误码
	ErrorMessage   string          `json:"error_message,omitempty"`   // 失败时的错误信息
}

type ToolType string

const (
	ToolTypeBuiltin ToolType = "builtin"
	ToolTypeMCP     ToolType = "mcp"
)

// ToolSpec 是工具的静态规格描述，注册时固化，
// 供 Executor 在执行前做启用、只读、联网、超时等校验。
type ToolSpec struct {
	Name            string          `json:"name"`                   // 工具名称，注册表中的唯一标识
	Description     string          `json:"description"`            // 工具用途说明，供模型理解何时调用
	InputSchema     json.RawMessage `json:"input_schema,omitempty"` // 入参 JSON Schema（用于参数校验）
	Type            ToolType        `json:"type"`                   // 工具类型：内置（builtin）或 MCP
	ReadOnly        bool            `json:"read_only"`              // 是否只读（非只读工具一律禁止调用）
	Enabled         bool            `json:"enabled"`                // 是否启用（注册时的快照，用于快速路径校验）
	SourceID        string          `json:"source_id,omitempty"`    // 工具所属资源标识（MCP 工具为 Server ID），供调用前动态可用性检查使用；内置工具为空
	MCPToolID       string          `json:"mcp_tool_id,omitempty"`  // MCP 工具在 mcp_tools 表中的 ID（仅 MCP 工具）；内置工具为空
	NetworkRequired bool            `json:"network_required"`       // 是否需要联网（联网被禁用时不可调用）
	Timeout         time.Duration   `json:"timeout"`                // 单次调用超时时间
	MaxCalls        int             `json:"max_calls"`              // 单次运行内允许的最大调用次数
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
	//	Specs 返回全部已注册工具的规格描述
	Specs() []ToolSpec
}
