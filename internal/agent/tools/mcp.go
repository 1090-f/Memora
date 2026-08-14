package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	internalmcp "github.com/1090-f/Memora/internal/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
)

// MCPReadOnlyTool 将已授权的 MCPServerTool 包装成 Eino 只读工具；它只调用传入的 MCPClient。
type MCPReadOnlyTool struct {
	client   internalmcp.MCPClient
	target   internalmcp.MCPServerTarget
	metadata internalmcp.MCPServerTool
	serverID string // 所属 MCP Server ID，用于调用前的动态可用性检查（第一层/第二层拦截）
	enabled  bool
	timeout  time.Duration
}

// NewMCPReadOnlyTool 创建 MCP 只读适配器。enabled 由服务端授权/配置传入，模型不能改变它；
// serverID 为工具所属 MCP Server 的唯一标识，供 Executor 在调用前动态复核启用状态。
func NewMCPReadOnlyTool(client internalmcp.MCPClient, target internalmcp.MCPServerTarget, metadata internalmcp.MCPServerTool, serverID string, enabled bool, timeout time.Duration) *MCPReadOnlyTool {
	return &MCPReadOnlyTool{client: client, target: target, metadata: metadata, serverID: serverID, enabled: enabled, timeout: timeout}
}

// Spec 固化 MCP 工具的只读、联网和启用边界，不包含服务端地址或凭证。
// SourceID 携带所属 Server ID，供 Executor 在调用前做动态可用性检查。
// Name 使用复合格式 "serverID::toolName" 避免不同 Server 的同名工具冲突。
func (t *MCPReadOnlyTool) Spec() contracts.ToolSpec {
	// 使用复合名称格式避免工具名冲突
	compositeName := fmt.Sprintf("%s::%s", t.serverID, t.metadata.Name)
	return contracts.ToolSpec{Name: compositeName, Description: t.metadata.Description, InputSchema: t.metadata.InputSchema, Type: contracts.ToolTypeMCP, ReadOnly: true, Enabled: t.enabled, SourceID: t.serverID, NetworkRequired: true, Timeout: t.timeout, MaxCalls: 10}
}

// Info 将 MCP 的 JSON Schema 转为 Eino schema，模型参数只代表 MCP arguments。
// 使用复合名称以保持与 Spec() 的一致性。
func (t *MCPReadOnlyTool) Info(context.Context) (*schema.ToolInfo, error) {
	compositeName := fmt.Sprintf("%s::%s", t.serverID, t.metadata.Name)
	if len(t.metadata.InputSchema) == 0 {
		return info(compositeName, t.metadata.Description, nil), nil
	}
	var inputSchema jsonschema.Schema
	if err := json.Unmarshal(t.metadata.InputSchema, &inputSchema); err != nil {
		return nil, fmt.Errorf("invalid MCP input schema: %w", err)
	}
	return info(compositeName, t.metadata.Description, schema.NewParamsOneOfByJSONSchema(&inputSchema)), nil
}

// InvokableRun 校验并原样转发模型 JSON，绝不向 MCP arguments 注入用户或知识库身份。
func (t *MCPReadOnlyTool) InvokableRun(ctx context.Context, input string, _ ...tool.Option) (string, error) {
	if !json.Valid([]byte(input)) {
		return "", invalidArgument("tool arguments must be valid JSON")
	}
	result, err := t.client.CallTool(ctx, t.target, t.metadata.Name, json.RawMessage(input), t.timeout)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// Execute 调用 MCPClient 并把 MCP 原始响应同时作为文本和结构化数据保留。
func (t *MCPReadOnlyTool) Execute(ctx context.Context, _ contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	compositeName := fmt.Sprintf("%s::%s", t.serverID, t.metadata.Name)
	text, err := t.InvokableRun(ctx, string(call.Arguments))
	if err != nil {
		code := contracts.ErrMCPCallFailed
		if _, ok := err.(*argumentError); ok {
			code = contracts.ErrInvalidArgument
		}
		return contracts.ToolResult{CallID: call.CallID, ToolName: compositeName, Success: false, ErrorCode: code, ErrorMessage: err.Error()}, err
	}
	return contracts.ToolResult{CallID: call.CallID, ToolName: compositeName, Text: text, StructuredData: json.RawMessage(text), Success: true}, nil
}

var _ Tool = (*MCPReadOnlyTool)(nil)
