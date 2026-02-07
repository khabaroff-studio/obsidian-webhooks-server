package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
)

// KeyRepository defines the interface for key data access
type KeyRepository interface {
	// Validation
	ValidateWebhookKey(ctx context.Context, keyValue string) (bool, error)
	ValidateClientKey(ctx context.Context, keyValue string) (bool, error)

	// Get by value
	GetWebhookKeyByValue(ctx context.Context, keyValue string) (*models.WebhookKey, error)
	GetClientKeyByValue(ctx context.Context, keyValue string) (*models.ClientKey, error)
	GetKeyByID(ctx context.Context, keyID string) (*KeyInfo, error)

	// CRUD operations
	CreateWebhookKey(ctx context.Context, key *models.WebhookKey) error
	CreateClientKey(ctx context.Context, key *models.ClientKey) error
	UpdateKeyStatus(ctx context.Context, keyValue string, isActive bool) error
	DeleteKeyPair(ctx context.Context, webhookKeyValue string) error

	// Listing
	GetWebhookKeys(ctx context.Context) ([]*models.WebhookKey, error)
	GetClientKeys(ctx context.Context) ([]*models.ClientKey, error)

	// Key pair creation (transactional)
	CreateKeyPair(ctx context.Context, webhookKey *models.WebhookKey, clientKey *models.ClientKey) error
}

// EventRepository defines the interface for event data access
type EventRepository interface {
	Create(ctx context.Context, event *models.Event) error
	GetByID(ctx context.Context, eventID uuid.UUID) (*models.Event, error)
	GetUnprocessed(ctx context.Context, webhookKeyID uuid.UUID) ([]models.Event, error)
	GetByWebhookKey(ctx context.Context, webhookKeyID uuid.UUID, limit int) ([]models.Event, error)
	MarkAsProcessed(ctx context.Context, eventID uuid.UUID) error
	Delete(ctx context.Context, eventID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// AdminRepository defines the interface for admin data access
type AdminRepository interface {
	Create(ctx context.Context, admin *models.AdminUser) error
	GetByUsername(ctx context.Context, username string) (*models.AdminUser, error)
	UpdateLastLogin(ctx context.Context, adminID uuid.UUID) error
}

// KeyInfo represents basic key information for responses
type KeyInfo struct {
	ID       string
	KeyValue string
	KeyType  string
	IsActive bool
}
