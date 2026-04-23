package model

import "time"

type Todo struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	User      User      `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
	Title     string    `gorm:"not null" json:"title"`
}
