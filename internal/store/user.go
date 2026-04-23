// 05
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/rshdhere/bookmyShow/internal/model"
	"gorm.io/gorm"
)

func (s *PostgresStore) CreateUser(
	ctx context.Context,
	name, email, passwordHash string,
) (model.User, error) {
	user := model.User{
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
	}

	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrEmailExists
		}
		return model.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *PostgresStore) GetUserByEmail(
	ctx context.Context,
	email string,
) (model.User, bool, error) {
	var user model.User

	err := s.db.WithContext(ctx).
		Where("email = ?", email).
		First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, false, nil
	}

	if err != nil {
		return model.User{}, false, fmt.Errorf("get user by email: %w", err)
	}

	return user, true, nil
}

func (s *PostgresStore) GetUserByID(
	ctx context.Context,
	id uint,
) (model.User, bool, error) {
	var user model.User

	err := s.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, false, nil
	}

	if err != nil {
		return model.User{}, false, fmt.Errorf("get user by id: %w", err)
	}

	return user, true, nil
}

func (s *PostgresStore) DeleteUser(
	ctx context.Context,
	id uint,
) (bool, error) {
	result := s.db.WithContext(ctx).Delete(&model.User{}, id)

	if result.Error != nil {
		return false, fmt.Errorf("delete user: %w", result.Error)
	}

	return result.RowsAffected > 0, nil
}
