package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

type MCPClient interface {
	Call(ctx context.Context, serverID string, toolName string, arguments json.RawMessage) (contracts.ToolResult, error)
}

type MCPTool struct {
	client   MCPClient
	serverID string
	toolName string
	spec     contracts.ToolSpec
}

func NewMCPTool(client MCPClient, serverID string, toolName string, description string, inputSchema json.RawMessage, readOnly bool, enabled bool, networkRequired bool, timeout time.Duration, maxCalls int) (*MCPTool, error) {
	if client == nil {
		return nil, errors.New("mcp client is required")
	}
	serverID = strings.TrimSpace(serverID)
	toolName = strings.TrimSpace(toolName)
	if serverID == "" || toolName == "" {
		return nil, errors.New("server_id and tool_name are required")
	}

	fullName := "mcp." + serverID + "." + toolName
	return &MCPTool{
		client:   client,
		serverID: serverID,
		toolName: toolName,
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

func (t *MCPTool) Spec() contracts.ToolSpec {
	return t.spec
}

func (t *MCPTool) Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
	res, err := t.client.Call(ctx, t.serverID, t.toolName, arguments)
	if err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrMCPCallFailed, ErrorMessage: "mcp call failed"}, nil
	}
	return res, nil
}
