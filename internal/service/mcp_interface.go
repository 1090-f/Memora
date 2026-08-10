package service

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/model/dto/response"
)

// ImportService 定义了 MCP Server 导入的管理接口。
type ImportService interface {
	Import(ctx context.Context, userID string, req *request.MCPImportRequest) (*response.MCPImportResponse, error)
	List(ctx context.Context, userID string) (*response.MCPServerListResponse, error)
	GetDetail(ctx context.Context, userID string, serverID string) (*response.MCPServerDetailResponse, error)
	Delete(ctx context.Context, userID string, serverID string) error
	TestConnection(ctx context.Context, userID string, serverID string) (*response.MCPTestResult, error)
	DiscoverTools(ctx context.Context, userID string, serverID string) (*response.MCPDiscoverResult, error)
	UpdateToolStatus(ctx context.Context, userID string, toolID string, enabled bool) error
	UpdateServerEnabled(ctx context.Context, userID string, serverID string, enabled bool) error
	// ListEnabledTools 返回当前用户已启用（Server 与 Tool 均启用）的 MCP 工具，
	// 供 Agent 准备阶段过滤模型可见工具集合（第一层拦截）。
	ListEnabledTools(ctx context.Context, userID string) ([]response.MCPEnabledTool, error)
}
