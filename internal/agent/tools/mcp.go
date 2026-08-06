package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// MCPClient 定义了指向某个 MCP 服务器的调用抽象，
// 由上层提供具体实现，MCPTool 负责把调用转发给它。
type MCPClient interface {
	Call(ctx context.Context, serverID string, toolName string, arguments json.RawMessage) (contracts.ToolResult, error)
}

// MCPToolStatusChecker 在调用 MCP 工具前查询其当前启用状态。
type MCPToolStatusChecker interface {
	IsEnabled(ctx context.Context, userID, serverID, toolName string) (bool, error)
}

// MCPTool 包装外部 MCP 服务器上的某个工具，
// 使其在内部以普通工具的身份注册到 Registry 中。
type MCPTool struct {
	client        MCPClient
	statusChecker MCPToolStatusChecker
	serverID      string
	toolName      string
	spec          contracts.ToolSpec
}

// NewMCPTool 根据服务器与工具的静态元数据创建一个 MCP 工具壳。
// 工具全名采用 "mcp.<serverID>.<toolName>" 的命名约定以避免与其他工具冲突。
func NewMCPTool(client MCPClient, statusChecker MCPToolStatusChecker, serverID string, toolName string, description string, inputSchema json.RawMessage, readOnly bool, enabled bool, networkRequired bool, timeout time.Duration, maxCalls int) (*MCPTool, error) {
	// 客户端与服务器、工具名均不可为空。
	if client == nil {
		return nil, errors.New("mcp client is required")
	}
	serverID = strings.TrimSpace(serverID)
	toolName = strings.TrimSpace(toolName)
	if serverID == "" || toolName == "" {
		return nil, errors.New("server_id and tool_name are required")
	}

	// 组装唯一工具名并固化规格描述。
	fullName := "mcp." + serverID + "." + toolName
	if statusChecker == nil {
		return nil, errors.New("mcp tool status checker is required")
	}
	return &MCPTool{
		client:        client,
		statusChecker: statusChecker,
		serverID:      serverID,
		toolName:      toolName,
		spec: contracts.ToolSpec{
			Name:            fullName,
			Description:     description,
			InputSchema:     inputSchema,
			Type:            contracts.ToolTypeMCP,
			ReadOnly:        readOnly,
			Enabled:         enabled,
			NetworkRequired: networkRequired,
			Timeout:         timeout,
			MaxCalls:        maxCalls,
		},
	}, nil
}

// Spec 返回该工具的规格描述。
func (t *MCPTool) Spec() contracts.ToolSpec {
	return t.spec
}

// Run 把一次工具调用转发给底层 MCP 客户端，
// 调用失败时统一映射为 MCP 调用失败错误码。
func (t *MCPTool) Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
	if t.statusChecker != nil {
		enabled, err := t.statusChecker.IsEnabled(ctx, string(toolContext.UserID), t.serverID, t.toolName)
		if err != nil {
			return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: "failed to check mcp tool status"}, nil
		}
		if !enabled {
			return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrForbidden, ErrorMessage: "mcp tool is disabled"}, nil
		}
	}

	res, err := t.client.Call(ctx, t.serverID, t.toolName, arguments)
	if err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrMCPCallFailed, ErrorMessage: "mcp call failed"}, nil
	}
	return res, nil
}
