package modelconfig

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册模型配置相关的路由。
func (ctrl *Controller) RegisterRoutes(rg *gin.RouterGroup, authRequired gin.HandlerFunc) {
	modelConfigs := rg.Group("/model-configs")
	modelConfigs.Use(authRequired)
	{
		modelConfigs.GET("", ctrl.ListModelConfigs)
		modelConfigs.POST("", ctrl.CreateModelConfig)
		modelConfigs.PATCH("/:id", ctrl.UpdateModelConfig)
		modelConfigs.DELETE("/:id", ctrl.DeleteModelConfig)
	}
}
