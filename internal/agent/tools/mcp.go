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
	enabled  bool
	timeout  time.Duration
}

// NewMCPReadOnlyTool 创建 MCP 只读适配器。enabled 由服务端授权/配置传入，模型不能改变它。
func NewMCPReadOnlyTool(client internalmcp.MCPClient, target internalmcp.MCPServerTarget, metadata internalmcp.MCPServerTool, enabled bool, timeout time.Duration) *MCPReadOnlyTool {
	return &MCPReadOnlyTool{client: client, target: target, metadata: metadata, enabled: enabled, timeout: timeout}
}

// Spec 固化 MCP 工具的只读、联网和启用边界，不包含服务端地址或凭证。
func (t *MCPReadOnlyTool) Spec() contracts.ToolSpec {
	return contracts.ToolSpec{Name: t.metadata.Name, Description: t.metadata.Description, InputSchema: t.metadata.InputSchema, Type: contracts.ToolTypeMCP, ReadOnly: true, Enabled: t.enabled, NetworkRequired: true, Timeout: t.timeout, MaxCalls: 10}
}

// Info 将 MCP 的 JSON Schema 转为 Eino schema，模型参数只代表 MCP arguments。
func (t *MCPReadOnlyTool) Info(context.Context) (*schema.ToolInfo, error) {
	if len(t.metadata.InputSchema) == 0 {
		return info(t.metadata.Name, t.metadata.Description, nil), nil
	}
	var inputSchema jsonschema.Schema
	if err := json.Unmarshal(t.metadata.InputSchema, &inputSchema); err != nil {
		return nil, fmt.Errorf("invalid MCP input schema: %w", err)
	}
	return info(t.metadata.Name, t.metadata.Description, schema.NewParamsOneOfByJSONSchema(&inputSchema)), nil
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
	text, err := t.InvokableRun(ctx, string(call.Arguments))
	if err != nil {
		code := contracts.ErrMCPCallFailed
		if _, ok := err.(*argumentError); ok {
			code = contracts.ErrInvalidArgument
		}
		return contracts.ToolResult{CallID: call.CallID, ToolName: t.metadata.Name, Success: false, ErrorCode: code, ErrorMessage: err.Error()}, err
	}
	return contracts.ToolResult{CallID: call.CallID, ToolName: t.metadata.Name, Text: text, StructuredData: json.RawMessage(text), Success: true}, nil
}

var _ Tool = (*MCPReadOnlyTool)(nil)
