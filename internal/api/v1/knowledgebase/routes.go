package knowledgebase

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册知识库管理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	kbs := v1.Group("/knowledge-bases", authRequired)
	kbs.POST("", ctrl.Create)
	kbs.GET("", ctrl.List)
	kbs.GET("/:kb_id", ctrl.Get)
	kbs.PATCH("/:kb_id", ctrl.Update)
	kbs.DELETE("/:kb_id", ctrl.Delete)
	kbs.GET("/:kb_id/search-config", ctrl.GetSearchConfig)
	kbs.PUT("/:kb_id/search-config", ctrl.UpdateSearchConfig)
}
