package response

// DirectoryNode 表示目录树的节点。
type DirectoryNode struct {
	ID        string           `json:"id"`                  // ID 目录 ID
	Name      string           `json:"name"`                // Name 目录名称
	ParentID  *string          `json:"parent_id,omitempty"` // ParentID 父目录 ID，为空表示根目录
	Depth     int              `json:"depth"`               // Depth 目录层级深度（根目录为 0）
	SortOrder int              `json:"sort_order"`          // SortOrder 同级目录的排序序号
	IsDefault bool             `json:"is_default"`          // IsDefault 是否为知识库默认目录
	Children  []*DirectoryNode `json:"children"`            // Children 子目录节点列表
}
