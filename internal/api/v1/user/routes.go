package user

import "github.com/gin-gonic/gin"

func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	users := v1.Group("/users", authRequired)
	users.GET("/me", ctrl.Me)
	users.PATCH("/me", ctrl.UpdateMe)
	users.PATCH("/me/password", ctrl.ChangePassword)
}
