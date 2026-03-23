package domain

import (
	"context"
	"time"
)

// User represents a registered account.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	Plan         string    `json:"plan"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserRepo persists and retrieves user accounts.
type UserRepo interface {
	CreateUser(ctx context.Context, u *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
}
