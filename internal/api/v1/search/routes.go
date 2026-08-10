package search

import "github.com/gin-gonic/gin"

func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	routes := v1.Group("/knowledge-bases/:kb_id/search", authRequired)
	routes.POST("", ctrl.Search)
	routes.POST("/test", ctrl.Test)
}
