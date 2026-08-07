package entity

import (
	"time"

	"gorm.io/gorm"
)

// BaseEntity 是所有业务实体的嵌入基础，包含 UUID 主键、创建时间、更新时间和软删除时间。
type BaseEntity struct {
	ID        string         `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
