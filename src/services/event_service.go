package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/repositories"
)

// EventService handles event-related operations
type EventService struct {
	pool      *pgxpool.Pool
	repo      repositories.EventRepository
	encryptor *Encryptor
}

// NewEventService creates a new event service
func NewEventService(pool *pgxpool.Pool) *EventService {
	return &EventService{pool: pool}
}

// NewEventServiceWithEncryption creates a new event service with encryption
func NewEventServiceWithEncryption(pool *pgxpool.Pool, encryptor *Encryptor) *EventService {
	return &EventService{pool: pool, encryptor: encryptor}
}

// NewEventServiceWithRepo creates a new event service with repository (for testing)
func NewEventServiceWithRepo(repo repositories.EventRepository) *EventService {
	return &EventService{repo: repo}
}

// decryptEventData decrypts the Data field of an event in-place
func (es *EventService) decryptEventData(event *models.Event) error {
	decrypted, err := es.encryptor.Decrypt(event.Data)
	if err != nil {
		return err
	}
	event.Data = decrypted
	return nil
}

// CreateEvent creates a new webhook event
func (es *EventService) CreateEvent(ctx context.Context, webhookKeyID uuid.UUID, path string, data []byte, ttl time.Duration) (*models.Event, error) {
	eventID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(ttl)

	// Encrypt data before storage
	storageData, err := es.encryptor.Encrypt(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt event data: %w", err)
	}

	event := &models.Event{
		ID:           eventID,
		WebhookKeyID: webhookKeyID,
		Path:         path,
		Data:         data, // keep plaintext in returned event
		Processed:    false,
		ProcessedAt:  nil,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
	}

	// Use repository if available (for testing)
	if es.repo != nil {
		// Store encrypted data in repo
		repoEvent := *event
		repoEvent.Data = storageData
		if err := es.repo.Create(ctx, &repoEvent); err != nil {
			return nil, fmt.Errorf("failed to create event: %w", err)
		}
		return event, nil
	}

	// Fallback to direct pool access (for backward compatibility)
	err = es.pool.QueryRow(ctx,
		`INSERT INTO events (id, webhook_key_id, path, data, processed, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, webhook_key_id, path, data, processed, processed_at, created_at, expires_at`,
		eventID, webhookKeyID, path, storageData, false, now, expiresAt,
	).Scan(&eventID, &webhookKeyID, &path, &storageData, nil, nil, &now, &expiresAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	return event, nil
}

// GetUnprocessedEvents retrieves unprocessed events for a webhook key
func (es *EventService) GetUnprocessedEvents(ctx context.Context, webhookKeyID uuid.UUID) ([]models.Event, error) {
	// Use repository if available (for testing)
	if es.repo != nil {
		return es.repo.GetUnprocessed(ctx, webhookKeyID)
	}

	// Fallback to direct pool access (for backward compatibility)
	rows, err := es.pool.Query(ctx,
		`SELECT id, webhook_key_id, path, data, processed, processed_at, created_at, expires_at
		 FROM events
		 WHERE webhook_key_id = $1 AND processed = false
		 ORDER BY created_at ASC`,
		webhookKeyID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		err := rows.Scan(&e.ID, &e.WebhookKeyID, &e.Path, &e.Data, &e.Processed, &e.ProcessedAt, &e.CreatedAt, &e.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		if err := es.decryptEventData(&e); err != nil {
			return nil, fmt.Errorf("failed to decrypt event data: %w", err)
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

// GetEventByID retrieves an event by ID
func (es *EventService) GetEventByID(ctx context.Context, eventID uuid.UUID) (*models.Event, error) {
	// Use repository if available (for testing)
	if es.repo != nil {
		event, err := es.repo.GetByID(ctx, eventID)
		if err != nil {
			return nil, fmt.Errorf("event not found: %w", err)
		}
		return event, nil
	}

	// Fallback to direct pool access (for backward compatibility)
	var e models.Event
	err := es.pool.QueryRow(ctx,
		`SELECT id, webhook_key_id, path, data, processed, processed_at, created_at, expires_at
		 FROM events WHERE id = $1`,
		eventID,
	).Scan(&e.ID, &e.WebhookKeyID, &e.Path, &e.Data, &e.Processed, &e.ProcessedAt, &e.CreatedAt, &e.ExpiresAt)

	if err != nil {
		return nil, fmt.Errorf("event not found: %w", err)
	}

	if err := es.decryptEventData(&e); err != nil {
		return nil, fmt.Errorf("failed to decrypt event data: %w", err)
	}

	return &e, nil
}

// MarkEventAsProcessed marks an event as processed
func (es *EventService) MarkEventAsProcessed(ctx context.Context, eventID uuid.UUID) error {
	// Use repository if available (for testing)
	if es.repo != nil {
		return es.repo.MarkAsProcessed(ctx, eventID)
	}

	// Fallback to direct pool access (for backward compatibility)
	now := time.Now()
	result, err := es.pool.Exec(ctx,
		`UPDATE events SET processed = true, processed_at = $1 WHERE id = $2`,
		now, eventID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("event not found")
	}

	return nil
}

// DeleteEvent deletes an event
func (es *EventService) DeleteEvent(ctx context.Context, eventID uuid.UUID) error {
	// Use repository if available (for testing)
	if es.repo != nil {
		return es.repo.Delete(ctx, eventID)
	}

	// Fallback to direct pool access (for backward compatibility)
	result, err := es.pool.Exec(ctx, "DELETE FROM events WHERE id = $1", eventID)

	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("event not found")
	}

	return nil
}

// GetEventsByWebhookKey retrieves events for a webhook key with limit
func (es *EventService) GetEventsByWebhookKey(ctx context.Context, webhookKeyID uuid.UUID, limit int) ([]models.Event, error) {
	// Use repository if available (for testing)
	if es.repo != nil {
		return es.repo.GetByWebhookKey(ctx, webhookKeyID, limit)
	}

	// Fallback to direct pool access (for backward compatibility)
	rows, err := es.pool.Query(ctx,
		`SELECT id, webhook_key_id, path, data, processed, processed_at, created_at, expires_at
		 FROM events
		 WHERE webhook_key_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		webhookKeyID, limit,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		err := rows.Scan(&e.ID, &e.WebhookKeyID, &e.Path, &e.Data, &e.Processed, &e.ProcessedAt, &e.CreatedAt, &e.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		if err := es.decryptEventData(&e); err != nil {
			return nil, fmt.Errorf("failed to decrypt event data: %w", err)
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

// DeleteExpiredEvents deletes events that have expired
func (es *EventService) DeleteExpiredEvents(ctx context.Context) (int64, error) {
	// Use repository if available (for testing)
	if es.repo != nil {
		return es.repo.DeleteExpired(ctx)
	}

	// Fallback to direct pool access (for backward compatibility)
	result, err := es.pool.Exec(ctx, "DELETE FROM events WHERE expires_at < NOW()")

	if err != nil {
		return 0, fmt.Errorf("failed to delete expired events: %w", err)
	}

	return result.RowsAffected(), nil
}

// CountUndeliveredEvents counts events not processed and older than the given duration
func (es *EventService) CountUndeliveredEvents(ctx context.Context, olderThan time.Duration) (int, error) {
	var count int
	err := es.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM events WHERE processed = false AND created_at < NOW() - make_interval(secs => $1)`,
		int(olderThan.Seconds()),
	).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count undelivered events: %w", err)
	}

	return count, nil
}

// CountEventsByWebhookKey counts total events for a webhook key
func (es *EventService) CountEventsByWebhookKey(ctx context.Context, webhookKeyID string) (int, error) {
	var count int
	err := es.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM events WHERE webhook_key_id = $1`,
		webhookKeyID,
	).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}

	return count, nil
}
