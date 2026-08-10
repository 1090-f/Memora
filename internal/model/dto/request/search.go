package request

// SearchRequest 是知识库检索 API 请求，身份和知识库由服务端注入。
type SearchRequest struct {
	Query       string   `json:"query" binding:"required,max=4096"`
	Mode        string   `json:"mode" binding:"omitempty,oneof=keyword vector semantic hybrid"`
	DocumentIDs []string `json:"document_ids" binding:"omitempty,max=100,dive,uuid"`
	TopK        int      `json:"top_k" binding:"omitempty,min=1,max=20"`
}
