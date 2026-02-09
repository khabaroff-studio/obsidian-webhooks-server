package mock

import (
	"context"

	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/repositories"
)

// AdminRepository is a mock implementation of repositories.AdminRepository
type AdminRepository struct {
	// Function stubs that can be overridden in tests
	CreateFunc          func(ctx context.Context, admin *models.AdminUser) error
	GetByUsernameFunc   func(ctx context.Context, username string) (*models.AdminUser, error)
	UpdateLastLoginFunc func(ctx context.Context, adminID uuid.UUID) error

	// Call tracking
	Calls map[string][]interface{}
}

// NewAdminRepository creates a new mock admin repository
func NewAdminRepository() *AdminRepository {
	return &AdminRepository{
		Calls: make(map[string][]interface{}),
	}
}

func (m *AdminRepository) Create(ctx context.Context, admin *models.AdminUser) error {
	m.Calls["Create"] = append(m.Calls["Create"], admin)
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, admin)
	}
	return nil
}

func (m *AdminRepository) GetByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	m.Calls["GetByUsername"] = append(m.Calls["GetByUsername"], username)
	if m.GetByUsernameFunc != nil {
		return m.GetByUsernameFunc(ctx, username)
	}
	return nil, nil
}

func (m *AdminRepository) UpdateLastLogin(ctx context.Context, adminID uuid.UUID) error {
	m.Calls["UpdateLastLogin"] = append(m.Calls["UpdateLastLogin"], adminID)
	if m.UpdateLastLoginFunc != nil {
		return m.UpdateLastLoginFunc(ctx, adminID)
	}
	return nil
}

// Ensure AdminRepository implements the interface
var _ repositories.AdminRepository = (*AdminRepository)(nil)
