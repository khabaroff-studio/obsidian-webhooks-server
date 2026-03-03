package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBroadcastEvent_IsolatesByWebhookKeyID(t *testing.T) {
	handler := NewSSEHandler(nil, nil, "")

	userA_webhookKeyID := uuid.New()
	userB_webhookKeyID := uuid.New()

	chA := make(chan interface{}, 10)
	chB := make(chan interface{}, 10)

	handler.addClient("ck_user_a", userA_webhookKeyID, chA)
	handler.addClient("ck_user_b", userB_webhookKeyID, chB)

	// Broadcast event belonging to user A
	handler.BroadcastEvent(SSEEvent{
		EventID:      uuid.New(),
		WebhookKeyID: userA_webhookKeyID,
		Data:         `{"path":"secret.md"}`,
	})

	// User A should receive the event
	select {
	case <-chA:
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Fatal("user A did not receive their own event")
	}

	// User B must NOT receive the event
	select {
	case evt := <-chB:
		t.Fatalf("user B received event that belongs to user A: %v", evt)
	case <-time.After(100 * time.Millisecond):
		// ok — no leak
	}
}

func TestBroadcastEvent_BothReceiveOwnEvents(t *testing.T) {
	handler := NewSSEHandler(nil, nil, "")

	userA_webhookKeyID := uuid.New()
	userB_webhookKeyID := uuid.New()

	chA := make(chan interface{}, 10)
	chB := make(chan interface{}, 10)

	handler.addClient("ck_user_a", userA_webhookKeyID, chA)
	handler.addClient("ck_user_b", userB_webhookKeyID, chB)

	// Broadcast event for user A
	handler.BroadcastEvent(SSEEvent{
		EventID:      uuid.New(),
		WebhookKeyID: userA_webhookKeyID,
		Data:         `{"path":"a.md"}`,
	})

	// Broadcast event for user B
	handler.BroadcastEvent(SSEEvent{
		EventID:      uuid.New(),
		WebhookKeyID: userB_webhookKeyID,
		Data:         `{"path":"b.md"}`,
	})

	// Each user should have exactly 1 event
	if len(chA) != 1 {
		t.Errorf("user A: expected 1 event, got %d", len(chA))
	}
	if len(chB) != 1 {
		t.Errorf("user B: expected 1 event, got %d", len(chB))
	}
}
