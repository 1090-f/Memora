package importtask

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册导入任务与文档处理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	// 导入任务：知识库维度列表/清理 + 任务维度详情/重试。
	tasks := v1.Group("", authRequired)
	tasks.GET("/knowledge-bases/:kb_id/import-tasks", ctrl.List)
	tasks.DELETE("/knowledge-bases/:kb_id/import-tasks", ctrl.Cleanup)
	tasks.GET("/import-tasks/:task_id", ctrl.Get)
	tasks.POST("/import-tasks/:task_id/retry", ctrl.Retry)
	tasks.POST("/import-tasks/:task_id/start", ctrl.Start)
	tasks.POST("/import-tasks/:task_id/scan", ctrl.Scan)
	tasks.POST("/import-tasks/:task_id/attachments", ctrl.Attachments)

	// 文档处理：按文档 ID 查询处理状态、重试与重新索引。
	docs := v1.Group("/documents/:document_id", authRequired)
	docs.GET("/processing", ctrl.GetProcessing)
	docs.POST("/retry-processing", ctrl.RetryProcessing)
	docs.POST("/reindex", ctrl.Reindex)
	docs.GET("/index-versions", ctrl.ListIndexVersions)
}
