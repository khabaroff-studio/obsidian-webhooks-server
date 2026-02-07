package models

import (
	"time"

	"github.com/google/uuid"
)

// AdminUser represents an admin user account
type AdminUser struct {
	ID           uuid.UUID  `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"` // never expose
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login"`
	IsActive     bool       `json:"is_active"`
}
