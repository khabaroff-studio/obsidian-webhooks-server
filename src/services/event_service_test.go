package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/repositories/mock"
)

func TestEventService_CreateEvent(t *testing.T) {
	ctx := context.Background()
	webhookKeyID := uuid.New()
	path := "/test/path"
	data := []byte(`{"test": "data"}`)
	ttl := 24 * time.Hour

	t.Run("creates event successfully", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.CreateFunc = func(ctx context.Context, event *models.Event) error {
			return nil
		}

		service := NewEventServiceWithRepo(mockRepo)
		event, err := service.CreateEvent(ctx, webhookKeyID, path, data, ttl)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if event == nil {
			t.Fatal("expected event, got nil")
		}
		if event.WebhookKeyID != webhookKeyID {
			t.Errorf("expected webhook key ID %v, got %v", webhookKeyID, event.WebhookKeyID)
		}
		if event.Path != path {
			t.Errorf("expected path %s, got %s", path, event.Path)
		}
		if string(event.Data) != string(data) {
			t.Errorf("expected data %s, got %s", data, event.Data)
		}
		if event.Processed {
			t.Error("expected event to not be processed")
		}

		// Verify repository was called
		if len(mockRepo.Calls["Create"]) != 1 {
			t.Errorf("expected 1 call to Create, got %d", len(mockRepo.Calls["Create"]))
		}
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.CreateFunc = func(ctx context.Context, event *models.Event) error {
			return errors.New("database error")
		}

		service := NewEventServiceWithRepo(mockRepo)
		_, err := service.CreateEvent(ctx, webhookKeyID, path, data, ttl)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestEventService_GetEventByID(t *testing.T) {
	ctx := context.Background()
	eventID := uuid.New()

	t.Run("returns event when found", func(t *testing.T) {
		expectedEvent := &models.Event{
			ID:           eventID,
			WebhookKeyID: uuid.New(),
			Path:         "/test",
			Data:         []byte(`{}`),
			Processed:    false,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}

		mockRepo := mock.NewEventRepository()
		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.Event, error) {
			if id == eventID {
				return expectedEvent, nil
			}
			return nil, errors.New("not found")
		}

		service := NewEventServiceWithRepo(mockRepo)
		event, err := service.GetEventByID(ctx, eventID)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if event.ID != expectedEvent.ID {
			t.Errorf("expected event ID %v, got %v", expectedEvent.ID, event.ID)
		}
	})

	t.Run("returns error when not found", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.Event, error) {
			return nil, errors.New("not found")
		}

		service := NewEventServiceWithRepo(mockRepo)
		_, err := service.GetEventByID(ctx, eventID)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestEventService_GetUnprocessedEvents(t *testing.T) {
	ctx := context.Background()
	webhookKeyID := uuid.New()

	t.Run("returns unprocessed events", func(t *testing.T) {
		expectedEvents := []models.Event{
			{ID: uuid.New(), WebhookKeyID: webhookKeyID, Processed: false},
			{ID: uuid.New(), WebhookKeyID: webhookKeyID, Processed: false},
		}

		mockRepo := mock.NewEventRepository()
		mockRepo.GetUnprocessedFunc = func(ctx context.Context, wkID uuid.UUID) ([]models.Event, error) {
			if wkID == webhookKeyID {
				return expectedEvents, nil
			}
			return nil, nil
		}

		service := NewEventServiceWithRepo(mockRepo)
		events, err := service.GetUnprocessedEvents(ctx, webhookKeyID)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}
	})

	t.Run("returns empty slice when no events", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.GetUnprocessedFunc = func(ctx context.Context, wkID uuid.UUID) ([]models.Event, error) {
			return []models.Event{}, nil
		}

		service := NewEventServiceWithRepo(mockRepo)
		events, err := service.GetUnprocessedEvents(ctx, webhookKeyID)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})
}

func TestEventService_MarkEventAsProcessed(t *testing.T) {
	ctx := context.Background()
	eventID := uuid.New()

	t.Run("marks event as processed", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.MarkAsProcessedFunc = func(ctx context.Context, id uuid.UUID) error {
			return nil
		}

		service := NewEventServiceWithRepo(mockRepo)
		err := service.MarkEventAsProcessed(ctx, eventID)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(mockRepo.Calls["MarkAsProcessed"]) != 1 {
			t.Errorf("expected 1 call to MarkAsProcessed, got %d", len(mockRepo.Calls["MarkAsProcessed"]))
		}
	})

	t.Run("returns error when event not found", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.MarkAsProcessedFunc = func(ctx context.Context, id uuid.UUID) error {
			return errors.New("event not found")
		}

		service := NewEventServiceWithRepo(mockRepo)
		err := service.MarkEventAsProcessed(ctx, eventID)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestEventService_DeleteEvent(t *testing.T) {
	ctx := context.Background()
	eventID := uuid.New()

	t.Run("deletes event successfully", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.DeleteFunc = func(ctx context.Context, id uuid.UUID) error {
			return nil
		}

		service := NewEventServiceWithRepo(mockRepo)
		err := service.DeleteEvent(ctx, eventID)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.DeleteFunc = func(ctx context.Context, id uuid.UUID) error {
			return errors.New("delete failed")
		}

		service := NewEventServiceWithRepo(mockRepo)
		err := service.DeleteEvent(ctx, eventID)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestEventService_DeleteExpiredEvents(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes expired events and returns count", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.DeleteExpiredFunc = func(ctx context.Context) (int64, error) {
			return 5, nil
		}

		service := NewEventServiceWithRepo(mockRepo)
		count, err := service.DeleteExpiredEvents(ctx)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if count != 5 {
			t.Errorf("expected count 5, got %d", count)
		}
	})

	t.Run("returns zero when no expired events", func(t *testing.T) {
		mockRepo := mock.NewEventRepository()
		mockRepo.DeleteExpiredFunc = func(ctx context.Context) (int64, error) {
			return 0, nil
		}

		service := NewEventServiceWithRepo(mockRepo)
		count, err := service.DeleteExpiredEvents(ctx)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0, got %d", count)
		}
	})
}
