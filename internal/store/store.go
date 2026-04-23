// Package store deals with all the db-level stuff in here
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `gorm:"not null" json:"name"`
	Password  string    `gorm:"not null" json:"password"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
}

var ErrEmailExists = errors.New("email already exists")

type Store interface {
	CreateUser(ctx context.Context, name, email, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, bool, error)
	GetUserById(ctx context.Context, id uint) (User, bool, error)
	DeleteUser(ctx context.Context, id uint) (User, bool, error)
	Close() error
}

type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("database_url is required")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := db.AutoMigrate(&User{}); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}

func (s *PostgresStore) CreateUser(
	ctx context.Context,
	name, email, passwordHash string,
) (User, error) {
	user := User{
		Name:     name,
		Email:    email,
		Password: passwordHash,
	}

	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailExists
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *PostgresStore) DeleteUser(
	ctx context.Context,
	id uint,
) (bool, error) {
	result := s.db.WithContext(ctx).Delete(&User{}, id)
	if result.Error != nil {
		return false, fmt.Errorf("deleted user: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}
