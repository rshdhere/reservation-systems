// Package store deals with all db-level stuff
package store

import (
	"context"

	"github.com/rshdhere/bookmyShow/internal/model"
)

type Store interface {
	CreateUser(ctx context.Context, name, email, passwordHash string) (model.User, error)
	GetUser(ctx context.Context, email string) (model.User, bool, error)
	UpdateUser(ctx context.Context, id uint, name, email, passwordHash *string) (model.User, bool, error)
	DeleteUser(ctx context.Context, id uint) (bool, error)
	Close() error
}
