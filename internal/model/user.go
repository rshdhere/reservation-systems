// Package model handles all the data-level stuff
package model

import "time"

type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `gorm:"not null" json:"name"`
	Password  string    `gorm:"not null" json:"password"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
}
