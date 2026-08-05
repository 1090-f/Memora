package entity

import "time"

// User 表示用户实体，映射到 users 数据库表。
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

// TableName 返回用户实体对应的数据库表名。
func (User) TableName() string { return "users" }
