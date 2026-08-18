package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	internalmcp "github.com/1090-f/Memora/internal/mcp"
)

// MCPToolProvider 定义获取用户已启用 MCP 工具列表的接口。
// 由上层服务（如 MCP ImportService）实现，供工具注册表动态刷新使用。
type MCPToolProvider interface {
	// ListEnabledToolsForRegistry 返回用户已启用的 MCP 工具元数据，
	// 包含 Server 连接信息和工具 Schema，供注册表实时加载。
	ListEnabledToolsForRegistry(ctx context.Context, userID string) ([]MCPToolMetadata, error)
}

// MCPToolMetadata 封装单个 MCP 工具的完整元数据，供注册表构造工具实例。
type MCPToolMetadata struct {
	ServerID      string                      // MCP Server ID
	ServerTarget  internalmcp.MCPServerTarget // Server 连接目标（URL、headers 等）
	ToolMetadata  internalmcp.MCPServerTool   // 工具元数据（name、description、schema）
	ToolID        string                      // MCP 工具在 mcp_tools 表中的 ID
	Enabled       bool                        // 是否启用
	CallTimeoutMs int                         // 调用超时（毫秒）
}

// MCPToolRefresher 负责在 Agent 启动前将用户已启用的 MCP 工具动态加载到注册表。
// 这是双层校验的第一层：只有已启用的工具才会被注册到工具列表供模型选择。
type MCPToolRefresher struct {
	registry *Registry
	provider MCPToolProvider
	client   internalmcp.MCPClient
}

// NewMCPToolRefresher 创建 MCP 工具刷新器实例。
func NewMCPToolRefresher(registry *Registry, provider MCPToolProvider, client internalmcp.MCPClient) *MCPToolRefresher {
	return &MCPToolRefresher{
		registry: registry,
		provider: provider,
		client:   client,
	}
}

// RefreshForUser 为指定用户刷新 MCP 工具列表。
// 第一层校验：在 Agent 启动前，从数据库查询用户已启用的 MCP Server 和 Tool，
// 过滤掉被禁用的工具，只把已启用的工具注册到注册表，供模型可见。
// 这样可以避免模型尝试调用已被禁用的工具。
func (r *MCPToolRefresher) RefreshForUser(ctx context.Context, userID string) error {
	if r.registry == nil || r.provider == nil {
		return fmt.Errorf("registry or provider is nil")
	}

	// 1. 清空注册表中所有 MCP 类型的工具，准备重新加载
	r.registry.UnregisterByType(contracts.ToolTypeMCP)

	// 2. 从数据层获取用户当前已启用的所有 MCP 工具元数据
	enabledTools, err := r.provider.ListEnabledToolsForRegistry(ctx, userID)
	if err != nil {
		return fmt.Errorf("获取已启用 MCP 工具列表失败: %w", err)
	}

	// 3. 为每个已启用的工具创建包装器并注册到工具表
	for _, meta := range enabledTools {
		// 将超时从毫秒转换为 time.Duration
		timeout := time.Duration(meta.CallTimeoutMs) * time.Millisecond
		if timeout <= 0 {
			timeout = 120 * time.Second // 默认 120 秒
		}

		// 创建 MCP 只读工具包装器
		mcpTool := NewMCPReadOnlyTool(
			r.client,
			meta.ServerTarget,
			meta.ToolMetadata,
			meta.ServerID,
			meta.ToolID,
			meta.Enabled,
			timeout,
		)

		// 注册或更新工具到注册表（幂等操作）
		if err := r.registry.RegisterOrUpdate(mcpTool); err != nil {
			return fmt.Errorf("注册 MCP 工具 %q 失败: %w", meta.ToolMetadata.Name, err)
		}
	}

	return nil
}

// RefreshForUserWithTimeout 是 RefreshForUser 的带超时版本，防止刷新操作阻塞过久。
func (r *MCPToolRefresher) RefreshForUserWithTimeout(parentCtx context.Context, userID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	return r.RefreshForUser(ctx, userID)
}
