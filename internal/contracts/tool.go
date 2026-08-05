package contracts

import (
	"context"
	"encoding/json"
	"time"
)

// ToolContext 是工具（Tool）执行时所需的上下文信息，用于权限隔离与用量限制。
type ToolContext struct {
	UserID           ID       `json:"user_id"`                // 发起工具调用的用户
	KnowledgeBaseID  ID       `json:"knowledge_base_id"`      // 关联的知识库
	AgentRunID       ID       `json:"agent_run_id"`           // 所属的 Agent 运行 ID
	PlanStepID       ID       `json:"plan_step_id,omitempty"` // 可选：执行的计划步骤（plan 模式下）
	ReactRound       int      `json:"react_round,omitempty"`  // 可选：ReAct 当前轮次
	AllowedToolNames []string `json:"allowed_tool_names"`     // 允许调用的工具名白名单
	NetworkEnabled   bool     `json:"network_enabled"`        // 是否允许联网工具
	MaxResultBytes   int      `json:"max_result_bytes"`       // 工具结果最大字节数限制
}

// ToolCall 表示一次工具调用请求。
type ToolCall struct {
	CallID    ID              `json:"call_id"`   // 调用 ID，用于关联结果
	ToolName  string          `json:"tool_name"` // 工具名称
	Arguments json.RawMessage `json:"arguments"` // 工具参数（原始 JSON）
}

// ToolResult 表示工具执行的结果。
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
	Enabled         bool            `json:"enabled"`                // 是否启用
	NetworkRequired bool            `json:"network_required"`       // 是否需要联网（联网被禁用时不可调用）
	Timeout         time.Duration   `json:"timeout"`                // 单次调用超时时间
	MaxCalls        int             `json:"max_calls"`              // 单次运行内允许的最大调用次数
}

// ToolExecutor 抽象工具执行能力。
type ToolExecutor interface {
	// Execute 执行一次工具调用并返回结果。
	Execute(ctx context.Context, toolContext ToolContext, call ToolCall) (ToolResult, error)
}

// ToolRegistry 提供工具注册表的查询能力，用于校验工具是否被允许。
type ToolRegistry interface {
	// Has 判断指定名称的工具是否存在。
	Has(name string) bool
	//	Get 获取指定名称的工具的规格描述
	Get(name string) (ToolSpec, bool)
	// Names 返回全部已注册工具的名称。
	Names() []string
	//	Specs 返回全部已注册工具的规格描述
	Specs() []ToolSpec
}
