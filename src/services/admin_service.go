package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/repositories"
	"golang.org/x/crypto/bcrypt"
)

// AdminService handles admin user operations
type AdminService struct {
	pool *pgxpool.Pool
	repo repositories.AdminRepository
}

// NewAdminService creates a new admin service
func NewAdminService(pool *pgxpool.Pool) *AdminService {
	return &AdminService{pool: pool}
}

// NewAdminServiceWithRepo creates a new admin service with repository (for testing)
func NewAdminServiceWithRepo(repo repositories.AdminRepository) *AdminService {
	return &AdminService{repo: repo}
}

// CreateAdminUser creates a new admin user with hashed password
func (as *AdminService) CreateAdminUser(ctx context.Context, username, password string) (*models.AdminUser, error) {
	// Validate input
	if len(username) < 1 || len(username) > 255 {
		return nil, errors.New("username must be between 1 and 255 characters")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	// Hash password with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	admin := &models.AdminUser{
		ID:           uuid.New(),
		Username:     username,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
		IsActive:     true,
	}

	// Use repository if available (for testing)
	if as.repo != nil {
		if err := as.repo.Create(ctx, admin); err != nil {
			return nil, fmt.Errorf("failed to create admin user: %w", err)
		}
		return admin, nil
	}

	// Fallback to direct pool access (for backward compatibility)
	query := `
		INSERT INTO admin_users (id, username, password_hash, created_at, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id, username, password_hash, created_at, last_login, is_active
	`

	err = as.pool.QueryRow(ctx, query, admin.ID, username, string(hash), admin.CreatedAt).Scan(
		&admin.ID, &admin.Username, &admin.PasswordHash, &admin.CreatedAt, &admin.LastLogin, &admin.IsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	return admin, nil
}

// HasAdmins checks if any admin users exist in the database
func (as *AdminService) HasAdmins(ctx context.Context) (bool, error) {
	var count int
	err := as.pool.QueryRow(ctx, "SELECT COUNT(*) FROM admin_users").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check admin users: %w", err)
	}
	return count > 0, nil
}

// AuthenticateAdmin verifies username and password
func (as *AdminService) AuthenticateAdmin(ctx context.Context, username, password string) (*models.AdminUser, error) {
	var admin *models.AdminUser
	var err error

	// Use repository if available (for testing)
	if as.repo != nil {
		admin, err = as.repo.GetByUsername(ctx, username)
		if err != nil {
			return nil, errors.New("invalid credentials")
		}
	} else {
		// Fallback to direct pool access (for backward compatibility)
		query := `
			SELECT id, username, password_hash, created_at, last_login, is_active
			FROM admin_users
			WHERE username = $1 AND is_active = true
		`

		admin = &models.AdminUser{}
		err = as.pool.QueryRow(ctx, query, username).Scan(
			&admin.ID, &admin.Username, &admin.PasswordHash, &admin.CreatedAt, &admin.LastLogin, &admin.IsActive,
		)
		if err != nil {
			return nil, errors.New("invalid credentials")
		}
	}

	// Compare password hash
	err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Update last_login timestamp
	now := time.Now()
	if as.repo != nil {
		err = as.repo.UpdateLastLogin(ctx, admin.ID)
		if err != nil {
			log.Printf("Failed to update last_login for admin %s: %v", admin.Username, err)
		}
	} else {
		updateQuery := `UPDATE admin_users SET last_login = $1 WHERE id = $2`
		_, err = as.pool.Exec(ctx, updateQuery, now, admin.ID)
		if err != nil {
			log.Printf("Failed to update last_login for admin %s: %v", admin.Username, err)
		}
	}

	admin.LastLogin = &now
	return admin, nil
}

// GetAdminByUsername retrieves admin user by username
func (as *AdminService) GetAdminByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	// Use repository if available (for testing)
	if as.repo != nil {
		admin, err := as.repo.GetByUsername(ctx, username)
		if err != nil {
			return nil, fmt.Errorf("admin user not found: %w", err)
		}
		return admin, nil
	}

	// Fallback to direct pool access (for backward compatibility)
	query := `
		SELECT id, username, password_hash, created_at, last_login, is_active
		FROM admin_users
		WHERE username = $1
	`

	admin := &models.AdminUser{}
	err := as.pool.QueryRow(ctx, query, username).Scan(
		&admin.ID, &admin.Username, &admin.PasswordHash, &admin.CreatedAt, &admin.LastLogin, &admin.IsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("admin user not found: %w", err)
	}

	return admin, nil
}
