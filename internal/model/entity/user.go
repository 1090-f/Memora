package entity

import "time"

type User struct {
	BaseEntity
	Username     string     `gorm:"column:username" json:"username"`
	Email        string     `gorm:"column:email" json:"email"`
	PasswordHash string     `gorm:"column:password_hash" json:"-"`
	Nickname     *string    `gorm:"column:nickname" json:"nickname"`
	AvatarURL    *string    `gorm:"column:avatar_url" json:"avatar_url"`
	Bio          *string    `gorm:"column:bio" json:"bio"`
	Status       string     `gorm:"column:status" json:"status"`
	LastLoginAt  *time.Time `gorm:"column:last_login_at" json:"last_login_at,omitempty"`
}

func (User) TableName() string { return "users" }
