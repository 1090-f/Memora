package entity

// DocumentDirectory 表示文档目录实体，映射到 document_directories 数据库表。
// 支持最多 5 层的树形目录结构。
type DocumentDirectory struct {
	BaseEntity
	UserID          string  `gorm:"column:user_id" json:"user_id"`                     // UserID 所属用户 ID
	KnowledgeBaseID string  `gorm:"column:knowledge_base_id" json:"knowledge_base_id"` // KnowledgeBaseID 所属知识库 ID
	ParentID        *string `gorm:"column:parent_id" json:"parent_id,omitempty"`       // ParentID 父目录 ID，为空表示根目录
	Name            string  `gorm:"column:name" json:"name"`                           // Name 目录名称
	Depth           int     `gorm:"column:depth" json:"depth"`                         // Depth 目录层级深度（根目录为 0）
	SortOrder       int     `gorm:"column:sort_order" json:"sort_order"`               // SortOrder 同级目录的排序序号
	IsDefault       bool    `gorm:"column:is_default" json:"is_default"`               // IsDefault 是否为知识库默认目录
}

// TableName 返回文档目录实体对应的数据库表名。
func (DocumentDirectory) TableName() string { return "document_directories" }
