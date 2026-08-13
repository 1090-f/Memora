package conversation

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册会话管理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	// 会话路由统一挂载认证中间件
	conversations := v1.Group("/conversations", authRequired)
	conversations.GET("", ctrl.List)
	conversations.GET("/:conversation_id", ctrl.Get)
	conversations.GET("/:conversation_id/messages", ctrl.ListMessages)
	conversations.PATCH("/:conversation_id", ctrl.Update)
	conversations.DELETE("/:conversation_id", ctrl.Delete)

	// 知识库下的会话创建路由
	kbConversations := v1.Group("/knowledge-bases/:kb_id/conversations", authRequired)
	kbConversations.POST("", ctrl.Create)
}
