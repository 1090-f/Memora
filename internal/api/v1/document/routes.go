package document

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册文档管理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	// 按文档 ID 的跨知识库操作（详情/删除）挂在根级路径下。
	docs := v1.Group("", authRequired)
	docs.GET("/documents/:document_id", ctrl.Get)
	docs.DELETE("/documents/:document_id", ctrl.Delete)

	// 知识库域下的文档集合操作（创建/列表）与导入上传分别成组。
	kbDocs := v1.Group("/knowledge-bases/:kb_id/documents", authRequired)
	kbDocs.POST("", ctrl.CreateManual)
	kbDocs.GET("", ctrl.List)

	imports := v1.Group("/knowledge-bases/:kb_id/imports", authRequired)
	imports.POST("/files", ctrl.UploadFiles)
	imports.POST("/url", ctrl.ImportURL)
}
