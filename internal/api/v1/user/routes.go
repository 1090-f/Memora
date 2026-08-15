package user

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册用户管理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	v1.GET("/users/:id/avatar", ctrl.Avatar)
	users := v1.Group("/users", authRequired)
	users.GET("/me", ctrl.Me)
	users.PATCH("/me", ctrl.UpdateMe)
	users.POST("/me/avatar", ctrl.UploadAvatar)
	users.PATCH("/me/password", ctrl.ChangePassword)
}
