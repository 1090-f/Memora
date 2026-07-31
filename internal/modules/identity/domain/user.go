// Package domain contains the identity module's business entities and errors.
package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrUserNotFound       = errors.New("user not found")
)

// User is the authenticated user known to the identity module.
type User struct {
	ID           string
	Username     string
	Nickname     string
	Email        string
	AvatarURL    *string
	Bio          string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DisplayNickname returns the API nickname, falling back to the username for
// the current minimal users schema.
func (u User) DisplayNickname() string {
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Username
}
