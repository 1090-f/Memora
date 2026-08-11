package document

import "github.com/gin-gonic/gin"

// RegisterRoutes 在指定的路由组上注册文档管理相关路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup, authRequired gin.HandlerFunc) {
	// 按文档 ID 的跨知识库操作（详情/删除）挂在根级路径下。
	docs := v1.Group("", authRequired)
	docs.GET("/documents/:document_id", ctrl.Get)
	docs.GET("/documents/:document_id/preview", ctrl.Preview)
	docs.GET("/documents/:document_id/original", ctrl.Original)
	docs.DELETE("/documents/:document_id", ctrl.Delete)

	// 资产下载不走 Bearer 认证：浏览器 <img> 无法携带 header，
	// 改为 HMAC 签名 URL（Preview 重写时生成），路由内校验 exp/sig。
	assets := v1.Group("/documents/:document_id/assets")
	assets.GET("/:asset_id", ctrl.Asset)

	// 知识库域下的文档集合操作（创建/列表）与导入上传分别成组。
	kbDocs := v1.Group("/knowledge-bases/:kb_id/documents", authRequired)
	kbDocs.POST("", ctrl.CreateManual)
	kbDocs.GET("", ctrl.List)
	kbDocs.GET("/:document_id/content", ctrl.ReadContent)

	imports := v1.Group("/knowledge-bases/:kb_id/imports", authRequired)
	imports.POST("/files", ctrl.UploadFiles)
	imports.POST("/url", ctrl.ImportURL)
}
