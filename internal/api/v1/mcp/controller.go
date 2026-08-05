package mcp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/audit"
	apperrors "github.com/1090-f/Memora/pkg/errors"
	"github.com/1090-f/Memora/pkg/response"
	"github.com/gin-gonic/gin"
)

// Controller 是 MCP Server 管理接口的 HTTP 处理器。
type Controller struct{ mcp service.ImportService }

// NewController 创建 MCP Controller 实例。
func NewController(mcp service.ImportService) *Controller { return &Controller{mcp: mcp} }

// Import 处理 POST /servers/import，接收 mcpServers JSON 进行一键导入。
func (ctrl *Controller) Import(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.MCPImportRequest
	limited := io.LimitReader(c.Request.Body, 64*1024+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	if len(body) > 64*1024 {
		response.Failure(c, apperrors.ErrPayloadTooLarge)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.mcp.Import(c.Request.Context(), user.ID, &req)
	if err != nil {
		response.Failure(c, mapServiceError(err))
		return
	}
	audit.Record("mcp.servers.import", user.ID, "mcp_servers", middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, result)
}

// List 处理 GET /servers，获取用户已导入的 MCP Server 列表。
func (ctrl *Controller) List(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.mcp.List(c.Request.Context(), user.ID)
	if err != nil {
		response.Failure(c, mapServiceError(err))
		return
	}
	response.Success(c, http.StatusOK, result)
}

// GetDetail 处理 GET /servers/:id，获取单个 MCP Server 详情及工具列表。
func (ctrl *Controller) GetDetail(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.mcp.GetDetail(c.Request.Context(), user.ID, c.Param("id"))
	if err != nil {
		response.Failure(c, mapServiceError(err))
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Delete 处理 DELETE /servers/:id，删除 MCP Server 及其关联工具。
func (ctrl *Controller) Delete(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	if err := ctrl.mcp.Delete(c.Request.Context(), user.ID, c.Param("id")); err != nil {
		response.Failure(c, mapServiceError(err))
		return
	}
	audit.Record("mcp.servers.delete", user.ID, "mcp_server:"+c.Param("id"), middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// Test 处理 POST /servers/:id/test，测试 MCP Server 连接。
func (ctrl *Controller) Test(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.mcp.TestConnection(c.Request.Context(), user.ID, c.Param("id"))
	if err != nil {
		response.Failure(c, mapServiceError(err))
		return
	}
	audit.Record("mcp.servers.test", user.ID, "mcp_server:"+c.Param("id"), middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, result)
}

// Discover 处理 POST /servers/:id/discover，手动触发工具发现。
func (ctrl *Controller) Discover(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.mcp.DiscoverTools(c.Request.Context(), user.ID, c.Param("id"))
	if err != nil {
		response.Failure(c, mapServiceError(err))
		return
	}
	audit.Record("mcp.servers.discover", user.ID, "mcp_server:"+c.Param("id"), middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, result)
}

// UpdateToolStatus 处理 PATCH /tools/:id，更新 MCP 工具启用/停用状态。
func (ctrl *Controller) UpdateToolStatus(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.UpdateToolEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	if err := ctrl.mcp.UpdateToolStatus(c.Request.Context(), user.ID, c.Param("id"), req.Enabled); err != nil {
		response.Failure(c, mapServiceError(err))
		return
	}
	audit.Record("mcp.tools.update", user.ID, "mcp_tool:"+c.Param("id"), middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, gin.H{"enabled": req.Enabled})
}

// mapServiceError 将 Service 层错误映射为统一错误响应。
func mapServiceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrMCPServerNotFound) {
		return apperrors.ErrNotFound
	}
	if errors.Is(err, repository.ErrDuplicateResource) {
		return apperrors.ErrConflict
	}
	return apperrors.New(apperrors.CodeInternal, http.StatusInternalServerError, err)
}
