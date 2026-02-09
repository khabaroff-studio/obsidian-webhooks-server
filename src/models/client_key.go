package models

import (
	"time"

	"github.com/google/uuid"
)

// ClientKey represents a client API key for SSE connections
type ClientKey struct {
	ID              uuid.UUID  `json:"id"`
	KeyValue        string     `json:"key_value"`
	WebhookKeyID    uuid.UUID  `json:"webhook_key_id"`
	Status          string     `json:"status"` // active or inactive
	CreatedAt       time.Time  `json:"created_at"`
	LastConnected   *time.Time `json:"last_connected,omitempty"`
	EventsDelivered int        `json:"events_delivered"`
}

// IsActive returns true if the client key is active
func (ck *ClientKey) IsActive() bool {
	return ck.Status == "active"
}
