package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	TransportStreamableHTTP = "streamable_http"
	TransportStdio          = "stdio"
)

// MCPServerTarget 统一描述两种传输的目标：HTTP 使用 URL/Headers，stdio 使用 Command/Args/Env/CWD。
type MCPServerTarget struct {
	Transport        string            `json:"transport"`
	URL              string            `json:"url,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	Command          string            `json:"command,omitempty"`
	Args             []string          `json:"args,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	CWD              string            `json:"cwd,omitempty"`
	MaxResponseBytes int               `json:"-"`
}

// MCPServerTool 是从 MCP Server 发现的工具元数据，InputSchema 可直接转换为 Eino JSON Schema。
type MCPServerTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// MCPClient 定义 MCP 连接、发现和调用能力；底层协议由 mcp-go 负责。
type MCPClient interface {
	Initialize(ctx context.Context, target MCPServerTarget, timeout time.Duration) error
	ListTools(ctx context.Context, target MCPServerTarget, timeout time.Duration) ([]MCPServerTool, error)
	CallTool(ctx context.Context, target MCPServerTarget, toolName string, arguments json.RawMessage, timeout time.Duration) (json.RawMessage, error)
}

type mcpClient struct{}

// NewMCPClient 创建无状态客户端。每次操作建立并关闭独立 MCP 会话，避免进程和会话泄漏。
func NewMCPClient() MCPClient { return &mcpClient{} }

func (c *mcpClient) Initialize(ctx context.Context, target MCPServerTarget, timeout time.Duration) error {
	ctx, cancel := withTimeout(ctx, timeout, 5*time.Second)
	defer cancel()
	cli, err := c.open(ctx, target)
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.Initialize(ctx, initializeRequest())
	return err
}

func (c *mcpClient) ListTools(ctx context.Context, target MCPServerTarget, timeout time.Duration) ([]MCPServerTool, error) {
	ctx, cancel := withTimeout(ctx, timeout, 10*time.Second)
	defer cancel()
	cli, err := c.open(ctx, target)
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	if _, err = cli.Initialize(ctx, initializeRequest()); err != nil {
		return nil, fmt.Errorf("initialize MCP server: %w", err)
	}
	result, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list MCP tools: %w", err)
	}
	tools := make([]MCPServerTool, 0, len(result.Tools))
	for _, item := range result.Tools {
		schema, err := json.Marshal(item.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("marshal input schema for %q: %w", item.Name, err)
		}
		tools = append(tools, MCPServerTool{Name: item.Name, Description: item.Description, InputSchema: schema})
	}
	return tools, nil
}

func (c *mcpClient) CallTool(ctx context.Context, target MCPServerTarget, toolName string, arguments json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	ctx, cancel := withTimeout(ctx, timeout, 30*time.Second)
	defer cancel()
	cli, err := c.open(ctx, target)
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	if _, err = cli.Initialize(ctx, initializeRequest()); err != nil {
		return nil, fmt.Errorf("initialize MCP server: %w", err)
	}
	result, err := cli.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{Name: toolName, Arguments: json.RawMessage(arguments)}})
	if err != nil {
		return nil, fmt.Errorf("call MCP tool: %w", err)
	}
	if result.IsError {
		return nil, fmt.Errorf("MCP tool returned an error")
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal MCP tool result: %w", err)
	}
	if target.MaxResponseBytes > 0 && len(data) > target.MaxResponseBytes {
		return nil, fmt.Errorf("response exceeds maximum size of %d bytes", target.MaxResponseBytes)
	}
	return data, nil
}

func (c *mcpClient) open(ctx context.Context, target MCPServerTarget) (*client.Client, error) {
	var cli *client.Client
	var err error
	switch target.Transport {
	case TransportStreamableHTTP:
		tr, trErr := transport.NewStreamableHTTP(target.URL, transport.WithHTTPHeaders(target.Headers))
		if trErr != nil {
			return nil, fmt.Errorf("create HTTP MCP transport: %w", trErr)
		}
		cli = client.NewClient(tr)
	case TransportStdio:
		env := make([]string, 0, len(target.Env))
		for key, value := range target.Env {
			env = append(env, key+"="+value)
		}
		tr := transport.NewStdioWithOptions(target.Command, env, target.Args, transport.WithCommandFunc(func(commandCtx context.Context, command string, env []string, args []string) (*exec.Cmd, error) {
			cmd := exec.CommandContext(commandCtx, command, args...)
			cmd.Env = append([]string{}, env...)
			cmd.Dir = target.CWD
			return cmd, nil
		}))
		cli = client.NewClient(tr)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", target.Transport)
	}
	if err = cli.Start(ctx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("start MCP client: %w", err)
	}
	return cli, nil
}

func initializeRequest() mcp.InitializeRequest {
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcp.Implementation{Name: "Memora", Version: "1.0.0"}
	request.Params.Capabilities.Roots = &struct {
		ListChanged bool `json:"listChanged,omitempty"`
	}{ListChanged: true}
	return request
}

func withTimeout(ctx context.Context, timeout, fallback time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = fallback
	}
	return context.WithTimeout(ctx, timeout)
}

// TestConnection 测试 MCP Server 是否可以完成 initialize 握手。
func TestConnection(ctx context.Context, client MCPClient, target MCPServerTarget, timeout time.Duration) error {
	return client.Initialize(ctx, target, timeout)
}

// DiscoverTools 发现 MCP 工具，返回的 InputSchema 供 Eino 工具适配器使用。
func DiscoverTools(ctx context.Context, client MCPClient, target MCPServerTarget, timeout time.Duration) ([]MCPServerTool, error) {
	return client.ListTools(ctx, target, timeout)
}
