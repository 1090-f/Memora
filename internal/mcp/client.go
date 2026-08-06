package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MCPServerTarget 统一描述两种传输的目标：
// - HTTP：URL + Headers
// - stdio：Command + Args + Env
type MCPServerTarget struct {
	Transport        string            `json:"transport"`
	URL              string            `json:"url,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	Command          string            `json:"command,omitempty"`
	Args             []string          `json:"args,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	MaxResponseBytes int               `json:"-"`
}

// MCPServerTool 是从 MCP Server 发现的工具元数据。
type MCPServerTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// MCPClient 定义了与 MCP Server Server 的交互抽象，
// 支持 Streamable HTTP 与 stdio 两种传输。
type MCPClient interface {
	// Initialize 发送 initialize 请求，建立会话。
	Initialize(ctx context.Context, target MCPServerTarget, timeout time.Duration) error
	// ListTools 发送 tools/list 请求，返回可用工具列表。
	ListTools(ctx context.Context, target MCPServerTarget, timeout time.Duration) ([]MCPServerTool, error)
	// CallTool 发送 tools/call 请求，执行指定工具。
	CallTool(ctx context.Context, target MCPServerTarget, toolName string, arguments json.RawMessage, timeout time.Duration) (json.RawMessage, error)
}

// mcpClient 是 MCPClient 的实现。
type mcpClient struct {
	httpClient *http.Client
}

// NewMCPClient 创建一个新的 MCP 客户端实例。
func NewMCPClient() MCPClient {
	return &mcpClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 10,
			},
		},
	}
}

// Initialize 向 MCP Server 发送 initialize 请求。
func (c *mcpClient) Initialize(ctx context.Context, target MCPServerTarget, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	initCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch target.Transport {
	case "streamable_http":
		return c.initializeHTTP(initCtx, target)
	case "stdio":
		return c.initializeStdio(initCtx, target)
	default:
		return fmt.Errorf("unsupported transport: %s", target.Transport)
	}
}

func (c *mcpClient) initializeHTTP(ctx context.Context, target MCPServerTarget) error {
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"roots": map[string]any{"listChanged": true},
			},
			"clientInfo": map[string]string{
				"name":    "Memora",
				"version": "1.0.0",
			},
		},
	}
	_, err := c.sendHTTPRequest(ctx, target, req)
	return err
}

func (c *mcpClient) initializeStdio(ctx context.Context, target MCPServerTarget) error {
	proc, err := StartStdioProcess(target, StdioProcessConfig{
		StartTimeout:   5 * time.Second,
		MaxOutputBytes: 1024 * 1024,
		KillTimeout:    2 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("start stdio process: %w", err)
	}
	defer proc.Close()

	req := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"roots": map[string]any{"listChanged": true},
		},
		"clientInfo": map[string]string{
			"name":    "Memora",
			"version": "1.0.0",
		},
	}
	_, err = proc.Request(ctx, "initialize", req)
	return err
}

// ListTools 向 MCP Server 发送 tools/list 请求。
func (c *mcpClient) ListTools(ctx context.Context, target MCPServerTarget, timeout time.Duration) ([]MCPServerTool, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	listCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch target.Transport {
	case "streamable_http":
		return c.listToolsHTTP(listCtx, target)
	case "stdio":
		return c.listToolsStdio(listCtx, target)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", target.Transport)
	}
}

func (c *mcpClient) listToolsHTTP(ctx context.Context, target MCPServerTarget) ([]MCPServerTool, error) {
	if err := c.initializeHTTP(ctx, target); err != nil {
		return nil, fmt.Errorf("initialize http server: %w", err)
	}
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  nil,
	}
	result, err := c.sendHTTPRequest(ctx, target, req)
	if err != nil {
		return nil, err
	}
	var listResponse struct {
		Tools []MCPServerTool `json:"tools"`
	}
	if err := json.Unmarshal(result, &listResponse); err != nil {
		return nil, fmt.Errorf("unmarshal tools list: %w", err)
	}
	return listResponse.Tools, nil
}

func (c *mcpClient) listToolsStdio(ctx context.Context, target MCPServerTarget) ([]MCPServerTool, error) {
	proc, err := StartStdioProcess(target, StdioProcessConfig{
		StartTimeout:   5 * time.Second,
		MaxOutputBytes: 1024 * 1024,
		KillTimeout:    2 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("start stdio process: %w", err)
	}
	defer proc.Close()

	initializeParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"roots": map[string]any{"listChanged": true},
		},
		"clientInfo": map[string]string{
			"name":    "Memora",
			"version": "1.0.0",
		},
	}
	if _, err := proc.Request(ctx, "initialize", initializeParams); err != nil {
		return nil, fmt.Errorf("initialize stdio process: %w", err)
	}

	result, err := proc.Request(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var listResponse struct {
		Tools []MCPServerTool `json:"tools"`
	}
	if err := json.Unmarshal(result, &listResponse); err != nil {
		return nil, fmt.Errorf("unmarshal tools list: %w", err)
	}
	return listResponse.Tools, nil
}

// CallTool 向 MCP Server 发送 tools/call 请求。
func (c *mcpClient) CallTool(ctx context.Context, target MCPServerTarget, toolName string, arguments json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch target.Transport {
	case "streamable_http":
		return c.callToolHTTP(callCtx, target, toolName, arguments)
	case "stdio":
		return c.callToolStdio(callCtx, target, toolName, arguments)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", target.Transport)
	}
}

func (c *mcpClient) callToolHTTP(ctx context.Context, target MCPServerTarget, toolName string, arguments json.RawMessage) (json.RawMessage, error) {
	if err := c.initializeHTTP(ctx, target); err != nil {
		return nil, fmt.Errorf("initialize http server: %w", err)
	}
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      toolName,
			"arguments": json.RawMessage(arguments),
		},
	}
	return c.sendHTTPRequest(ctx, target, req)
}

func (c *mcpClient) callToolStdio(ctx context.Context, target MCPServerTarget, toolName string, arguments json.RawMessage) (json.RawMessage, error) {
	proc, err := StartStdioProcess(target, StdioProcessConfig{
		StartTimeout:   5 * time.Second,
		MaxOutputBytes: 1024 * 1024,
		KillTimeout:    2 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("start stdio process: %w", err)
	}
	defer proc.Close()

	initializeParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"roots": map[string]any{"listChanged": true},
		},
		"clientInfo": map[string]string{
			"name":    "Memora",
			"version": "1.0.0",
		},
	}
	if _, err := proc.Request(ctx, "initialize", initializeParams); err != nil {
		return nil, fmt.Errorf("initialize stdio process: %w", err)
	}

	params := map[string]any{
		"name":      toolName,
		"arguments": json.RawMessage(arguments),
	}
	return proc.Request(ctx, "tools/call", params)
}

func (c *mcpClient) sendHTTPRequest(ctx context.Context, target MCPServerTarget, req mcpRequest) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", target.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range target.Headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	maxResponseBytes := target.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = 1024 * 1024
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxResponseBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(respBody) > maxResponseBytes {
		return nil, fmt.Errorf("response exceeds maximum size of %d bytes", maxResponseBytes)
	}
	var mcpResp mcpResponse
	if err := json.Unmarshal(respBody, &mcpResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if mcpResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}
	return mcpResp.Result, nil
}

// JSON-RPC 2.0 类型

type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TestConnection 测试与 MCP Server 的连接是否可用。
func TestConnection(ctx context.Context, client MCPClient, target MCPServerTarget, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.Initialize(testCtx, target, timeout)
}

// DiscoverTools 从 MCP Server 发现可用工具列表。
func DiscoverTools(ctx context.Context, client MCPClient, target MCPServerTarget, timeout time.Duration) ([]MCPServerTool, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	discoverCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.ListTools(discoverCtx, target, timeout)
}
