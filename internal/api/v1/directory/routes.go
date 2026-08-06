package directory

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册文档目录相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	dirs := v1.Group("/knowledge-bases/:kb_id/directories", authRequired)
	dirs.GET("/tree", ctrl.Tree)
	dirs.POST("", ctrl.Create)
}
