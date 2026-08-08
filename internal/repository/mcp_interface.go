package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
)

// MCPServerRepository 定义了 MCP Server 的数据访问接口。
type MCPServerRepository interface {
	// FindActiveByName 根据 user_id 和 name 查找未删除的 MCP Server。
	FindActiveByName(ctx context.Context, userID, name string) (*entity.MCPServer, error)
	// FindActiveByID 根据 user_id 和 id 查找未删除的 MCP Server。
	FindActiveByID(ctx context.Context, userID, serverID string) (*entity.MCPServer, error)
	// ListByUser 列出用户下所有未删除的 MCP Server。
	ListByUser(ctx context.Context, userID string) ([]entity.MCPServer, error)
	// Create 创建一个 MCP Server。
	Create(ctx context.Context, server *entity.MCPServer) error
	// UpdateStatus 更新 MCP Server 的连接状态与错误信息。
	UpdateStatus(ctx context.Context, serverID, status string, lastErr *string) error
	// UpdateEnabled 更新 MCP Server 的启用状态。
	UpdateEnabled(ctx context.Context, userID, serverID string, enabled bool) error
	// Delete 软删除 MCP Server（设置 deleted_at）。
	Delete(ctx context.Context, userID, serverID string) error
}

// MCPToolRepository 定义了 MCP Tool 的数据访问接口。
type MCPToolRepository interface {
	// FindByServer 查找指定 Server 下的所有工具。
	FindByServer(ctx context.Context, serverID string) ([]entity.MCPTool, error)
	// BatchCreate 批量创建工具。
	BatchCreate(ctx context.Context, tools []entity.MCPTool) error
	// DeleteByServer 删除指定 Server 下的所有工具。
	DeleteByServer(ctx context.Context, serverID string) error
	// UpdateEnabledByUser 更新工具启用状态，并校验工具属于用户。
	UpdateEnabledByUser(ctx context.Context, userID, toolID string, enabled bool) error
	// IsEnabled 查询用户拥有的 MCP Server 及其工具是否均处于启用状态。
	IsEnabled(ctx context.Context, userID, serverID, toolName string) (bool, error)
	// ListEnabledByUser 返回用户拥有的、Server 与 Tool 均启用的工具列表（含所属 Server ID），
	// 供 Agent 准备阶段过滤模型可见工具集合（第一层拦截）。
	ListEnabledByUser(ctx context.Context, userID string) ([]entity.MCPTool, error)
	// UpdateSchema 更新工具 Schema（Schema 变更时）。
	UpdateSchema(ctx context.Context, toolID, schemaHash string, schema []byte) error
}
