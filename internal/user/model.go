package user

import (
	"context"
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type Store interface {
	GetUser(ctx context.Context, id string) (*User, error)
}
