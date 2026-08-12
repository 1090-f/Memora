package memory

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册长期记忆管理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	// 记忆路由统一挂载认证中间件，未携带有效凭证直接返回 401。
	memories := v1.Group("/memories", authRequired)
	memories.GET("", ctrl.List)
	memories.GET("/:memory_id", ctrl.Get)
	memories.PATCH("/:memory_id/status", ctrl.UpdateStatus)
	memories.DELETE("/:memory_id", ctrl.Delete)
}
