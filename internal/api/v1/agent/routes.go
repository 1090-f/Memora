// Package agent 注册 Agent 运行管理的 HTTP 路由。
package agent

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定路由组上注册 Agent 运行管理的所有端点。
// auth 是认证中间件，所有端点均需要用户认证。
//
// 端点列表：
//
//	POST /api/v1/agent/runs 创建运行
//	GET /api/v1/agent/runs 分页查询运行记录
//	GET /api/v1/agent/runs/:id 获取运行详情
//	GET /api/v1/agent/runs/:id/events SSE 流式订阅运行事件
//	POST /api/v1/agent/runs/:id/cancel 取消运行
//	POST /api/v1/agent/runs/:id/retry 重试运行
func RegisterRoutes(rg *gin.RouterGroup, auth gin.HandlerFunc, ctrl *Controller) {
	// agent 路由组，所有端点均需认证。
	agent := rg.Group("/agent", auth)

	// 创建运行（POST /api/v1/agent/runs）。
	agent.POST("/runs", ctrl.CreateRun)
	// 分页查询运行记录（GET /api/v1/agent/runs）。
	agent.GET("/runs", ctrl.ListRuns)
	// 获取运行详情（GET /api/v1/agent/runs/:id）。
	agent.GET("/runs/:id", ctrl.GetRun)
	// SSE 流式订阅运行事件（GET /api/v1/agent/runs/:id/events）。
	agent.GET("/runs/:id/events", ctrl.SubscribeEvents)
	// 取消运行（POST /api/v1/agent/runs/:id/cancel）。
	agent.POST("/runs/:id/cancel", ctrl.CancelRun)
	// 重试运行（POST /api/v1/agent/runs/:id/retry）。
	agent.POST("/runs/:id/retry", ctrl.RetryRun)
}
