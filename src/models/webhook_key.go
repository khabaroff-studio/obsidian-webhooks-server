package models

import (
	"time"

	"github.com/google/uuid"
)

// WebhookKey represents a webhook API key
type WebhookKey struct {
	ID             uuid.UUID  `json:"id"`
	KeyValue       string     `json:"key_value"`
	Status         string     `json:"status"` // active or inactive
	CreatedAt      time.Time  `json:"created_at"`
	LastUsed       *time.Time `json:"last_used,omitempty"`
	EventsCount    int        `json:"events_count"`
	ClientKeyValue string     `json:"client_key,omitempty"`
}

// IsActive returns true if the webhook key is active
func (wk *WebhookKey) IsActive() bool {
	return wk.Status == "active"
}
