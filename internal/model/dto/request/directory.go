package request

// CreateDirectoryRequest 表示创建目录的请求。
type CreateDirectoryRequest struct {
	Name      string  `json:"name" binding:"required,max=128"` // Name 目录名称，必填，最长 128 字符
	ParentID  *string `json:"parent_id" binding:"omitempty"`   // ParentID 父目录 ID，为空则创建为根目录
	SortOrder *int    `json:"sort_order" binding:"omitempty"`  // SortOrder 同级目录排序序号，为空使用默认值
}
