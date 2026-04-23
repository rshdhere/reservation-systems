// Package store deals with all db-level stuff - 03
package store

import (
	"context"

	"github.com/rshdhere/bookmyShow/internal/model"
)

type Store interface {
	CreateUser(ctx context.Context, name, email, passwordHash string) (model.User, error)
	GetUserByEmail(ctx context.Context, email string) (model.User, bool, error)
	GetUserByID(ctx context.Context, id uint) (model.User, bool, error)
	CreateTodo(ctx context.Context, userID uint, title string) (model.Todo, error)
	GetTodoByID(ctx context.Context, id, userID uint) (model.Todo, bool, error)
	DeleteUser(ctx context.Context, id uint) (bool, error)
	Close() error
}
