package entities

import "time"

type User struct {
	ID        string    `json:"id"`
	Login     string    `json:"login"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
