package mcp

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册 MCP Server 管理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	mcpGroup := v1.Group("/mcp", authRequired)
	servers := mcpGroup.Group("/servers")
	servers.POST("/import", ctrl.Import)                  // 通过json导入mcp服务
	servers.GET("", ctrl.List)                            // 获取mcp服务列表
	servers.GET("/:id", ctrl.GetDetail)                   // 获取单个mcp服务下的所有工具（查数据库，内置在导入中）
	servers.DELETE("/:id", ctrl.Delete)                   // 删除mcp服务及其工具
	servers.POST("/:id/test", ctrl.Test)                  // 测试mcp服务连通性
	servers.POST("/:id/discover", ctrl.Discover)          // 发现mcp服务下的工具（远程发现服务下的工具）
	servers.PATCH("/:id/status", ctrl.UpdateServerStatus) // 更新mcp服务启用状态

	tools := mcpGroup.Group("/tools")
	tools.PATCH("/:id", ctrl.UpdateToolStatus) // 更新mcp单个工具的可用状态
}
