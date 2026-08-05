package entity

// DocumentDirectory 表示文档目录实体，映射到 document_directories 数据库表。
// 支持最多 5 层的树形目录结构。
type DocumentDirectory struct {
	BaseEntity
	UserID          string  `gorm:"column:user_id" json:"user_id"`
	KnowledgeBaseID string  `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`
	ParentID        *string `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Name            string  `gorm:"column:name" json:"name"`
	Depth           int     `gorm:"column:depth" json:"depth"`
	SortOrder       int     `gorm:"column:sort_order" json:"sort_order"`
	IsDefault       bool    `gorm:"column:is_default" json:"is_default"`
}

// TableName 返回文档目录实体对应的数据库表名。
func (DocumentDirectory) TableName() string { return "document_directories" }
