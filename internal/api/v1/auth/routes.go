package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册认证相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	auth := v1.Group("/auth")
	auth.POST("/login", ctrl.Login)
	auth.POST("/logout", authRequired, ctrl.Logout)
}
