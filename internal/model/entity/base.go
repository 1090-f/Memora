package entity

import "time"

type BaseEntity struct {
	ID        string     `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}
