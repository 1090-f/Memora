package request

// CreateDirectoryRequest 表示创建目录的请求。
type CreateDirectoryRequest struct {
	Name      string  `json:"name" binding:"required,max=128"`
	ParentID  *string `json:"parent_id" binding:"omitempty"`
	SortOrder *int    `json:"sort_order" binding:"omitempty"`
}
