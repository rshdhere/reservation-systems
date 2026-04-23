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
		Name:     name,
		Email:    email,
		Password: passwordHash,
	}

	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrEmailExists
		}
		return model.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *PostgresStore) GetUser(
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
		return model.User{}, false, fmt.Errorf("get user: %w", err)
	}

	return user, true, nil
}

func (s *PostgresStore) UpdateUser(
	ctx context.Context,
	id uint,
	name, email, passwordHash *string,
) (model.User, bool, error) {
	var user model.User

	err := s.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, fmt.Errorf("get user before update: %w", err)
	}

	updates := map[string]interface{}{}

	if name != nil {
		updates["name"] = *name
	}
	if email != nil {
		updates["email"] = *email
	}
	if passwordHash != nil {
		updates["password"] = *passwordHash
	}

	if len(updates) == 0 {
		return user, true, nil
	}

	if err := s.db.WithContext(ctx).
		Model(&user).
		Updates(updates).Error; err != nil {

		if isUniqueViolation(err) {
			return model.User{}, false, ErrEmailExists
		}

		return model.User{}, false, fmt.Errorf("update user: %w", err)
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
