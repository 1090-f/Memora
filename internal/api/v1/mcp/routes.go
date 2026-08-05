package mcp

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册 MCP Server 管理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	mcpGroup := v1.Group("/mcp", authRequired)
	servers := mcpGroup.Group("/servers")
	servers.POST("/import", ctrl.Import)
	servers.GET("", ctrl.List)
	servers.GET("/:id", ctrl.GetDetail)
	servers.DELETE("/:id", ctrl.Delete)
	servers.POST("/:id/test", ctrl.Test)
	servers.POST("/:id/discover", ctrl.Discover)

	tools := mcpGroup.Group("/tools")
	tools.PATCH("/:id", ctrl.UpdateToolStatus)
}
