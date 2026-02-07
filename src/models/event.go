package models

import (
	"time"

	"github.com/google/uuid"
)

// Event represents a webhook event
type Event struct {
	ID           uuid.UUID  `json:"id"`
	WebhookKeyID uuid.UUID  `json:"webhook_key_id"`
	Path         string     `json:"path"`
	Data         []byte     `json:"data"`
	Processed    bool       `json:"processed"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
}

// IsProcessed returns true if the event has been processed
func (e *Event) IsProcessed() bool {
	return e.Processed
}

// MarkProcessed marks the event as processed
func (e *Event) MarkProcessed() {
	e.Processed = true
	now := time.Now()
	e.ProcessedAt = &now
}
