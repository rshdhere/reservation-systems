// 17
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/rshdhere/bookmyShow/internal/model"
	"gorm.io/gorm"
)

func (s *PostgresStore) CreateTodo(
	ctx context.Context,
	userID uint,
	title string,
) (model.Todo, error) {
	todo := model.Todo{
		UserID: userID,
		Title:  title,
	}

	if err := s.db.WithContext(ctx).Create(&todo).Error; err != nil {
		return model.Todo{}, fmt.Errorf("create todo: %w", err)
	}

	return todo, nil
}

func (s *PostgresStore) GetTodoByID(
	ctx context.Context,
	id, userID uint,
) (model.Todo, bool, error) {
	var todo model.Todo

	err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&todo).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Todo{}, false, nil
	}

	if err != nil {
		return model.Todo{}, false, fmt.Errorf("get todo by id: %w", err)
	}

	return todo, true, nil
}
