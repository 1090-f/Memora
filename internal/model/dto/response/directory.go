package response

// DirectoryNode 表示目录树的节点。
type DirectoryNode struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	ParentID  *string          `json:"parent_id,omitempty"`
	Depth     int              `json:"depth"`
	SortOrder int              `json:"sort_order"`
	IsDefault bool             `json:"is_default"`
	Children  []*DirectoryNode `json:"children"`
}
