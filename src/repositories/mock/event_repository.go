package mock

import (
	"context"

	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/repositories"
)

// EventRepository is a mock implementation of repositories.EventRepository
type EventRepository struct {
	// Function stubs that can be overridden in tests
	CreateFunc          func(ctx context.Context, event *models.Event) error
	GetByIDFunc         func(ctx context.Context, eventID uuid.UUID) (*models.Event, error)
	GetUnprocessedFunc  func(ctx context.Context, webhookKeyID uuid.UUID) ([]models.Event, error)
	GetByWebhookKeyFunc func(ctx context.Context, webhookKeyID uuid.UUID, limit int) ([]models.Event, error)
	MarkAsProcessedFunc func(ctx context.Context, eventID uuid.UUID) error
	DeleteFunc          func(ctx context.Context, eventID uuid.UUID) error
	DeleteExpiredFunc   func(ctx context.Context) (int64, error)

	// Call tracking
	Calls map[string][]interface{}
}

// NewEventRepository creates a new mock event repository
func NewEventRepository() *EventRepository {
	return &EventRepository{
		Calls: make(map[string][]interface{}),
	}
}

func (m *EventRepository) Create(ctx context.Context, event *models.Event) error {
	m.Calls["Create"] = append(m.Calls["Create"], event)
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, event)
	}
	return nil
}

func (m *EventRepository) GetByID(ctx context.Context, eventID uuid.UUID) (*models.Event, error) {
	m.Calls["GetByID"] = append(m.Calls["GetByID"], eventID)
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, eventID)
	}
	return nil, nil
}

func (m *EventRepository) GetUnprocessed(ctx context.Context, webhookKeyID uuid.UUID) ([]models.Event, error) {
	m.Calls["GetUnprocessed"] = append(m.Calls["GetUnprocessed"], webhookKeyID)
	if m.GetUnprocessedFunc != nil {
		return m.GetUnprocessedFunc(ctx, webhookKeyID)
	}
	return nil, nil
}

func (m *EventRepository) GetByWebhookKey(ctx context.Context, webhookKeyID uuid.UUID, limit int) ([]models.Event, error) {
	m.Calls["GetByWebhookKey"] = append(m.Calls["GetByWebhookKey"], []interface{}{webhookKeyID, limit})
	if m.GetByWebhookKeyFunc != nil {
		return m.GetByWebhookKeyFunc(ctx, webhookKeyID, limit)
	}
	return nil, nil
}

func (m *EventRepository) MarkAsProcessed(ctx context.Context, eventID uuid.UUID) error {
	m.Calls["MarkAsProcessed"] = append(m.Calls["MarkAsProcessed"], eventID)
	if m.MarkAsProcessedFunc != nil {
		return m.MarkAsProcessedFunc(ctx, eventID)
	}
	return nil
}

func (m *EventRepository) Delete(ctx context.Context, eventID uuid.UUID) error {
	m.Calls["Delete"] = append(m.Calls["Delete"], eventID)
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, eventID)
	}
	return nil
}

func (m *EventRepository) DeleteExpired(ctx context.Context) (int64, error) {
	m.Calls["DeleteExpired"] = append(m.Calls["DeleteExpired"], nil)
	if m.DeleteExpiredFunc != nil {
		return m.DeleteExpiredFunc(ctx)
	}
	return 0, nil
}

// Ensure EventRepository implements the interface
var _ repositories.EventRepository = (*EventRepository)(nil)
