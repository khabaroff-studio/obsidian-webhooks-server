package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/database"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

// Test helpers for dashboard_test.go
func setupDashboardHandler(tdb *database.TestDB) *DashboardHandler {
	gin.SetMode(gin.TestMode)
	db := database.NewDatabaseFromPool(tdb.Pool)
	keyService := services.NewKeyService(db.GetPool())
	eventService := services.NewEventService(db.GetPool())
	return NewDashboardHandler(keyService, eventService)
}

func TestHandleGetEvents_Success(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		webhookKeyID, _, _, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		// Create test events
		_, err = tdb.CreateTestEvent(webhookKeyID, "/test1", []byte(`{"data":"1"}`))
		if err != nil {
			t.Fatalf("failed to create test event 1: %v", err)
		}

		_, err = tdb.CreateTestEvent(webhookKeyID, "/test2", []byte(`{"data":"2"}`))
		if err != nil {
			t.Fatalf("failed to create test event 2: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewDashboardHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/events?client_key="+clientKey, nil)

		handler.HandleGetEvents(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		events, ok := response["events"].([]interface{})
		if !ok {
			t.Fatal("expected events array")
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}

		count, ok := response["count"].(float64)
		if !ok || int(count) != 2 {
			t.Errorf("expected count 2, got %v", response["count"])
		}
	})
}

func TestHandleGetEvents_MissingClientKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		handler := setupDashboardHandler(tdb)
		w, c := createTestContext()
		c.Request = httptest.NewRequest(http.MethodGet, "/events", nil)

		handler.HandleGetEvents(c)

		assertStatusCode(t, w, http.StatusBadRequest)
		assertJSONError(t, w, "client_key query parameter is required")
	})
}

func TestHandleGetEvents_InvalidClientKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		handler := setupDashboardHandler(tdb)
		w, c := createTestContext()
		c.Request = httptest.NewRequest(http.MethodGet, "/events?client_key=invalid-key", nil)

		handler.HandleGetEvents(c)

		assertStatusCode(t, w, http.StatusUnauthorized)
		assertJSONError(t, w, "invalid client key")
	})
}

func TestHandleGetEvents_EmptyList(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, _, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewDashboardHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/events?client_key="+clientKey, nil)

		handler.HandleGetEvents(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		events, ok := response["events"].([]interface{})
		if !ok {
			t.Fatal("expected events array")
		}

		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}

		count, ok := response["count"].(float64)
		if !ok || int(count) != 0 {
			t.Errorf("expected count 0, got %v", response["count"])
		}
	})
}

func TestHandleDeleteEvent_Success(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		webhookKeyID, _, _, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		eventID, err := tdb.CreateTestEvent(webhookKeyID, "/test", []byte(`{"test":"data"}`))
		if err != nil {
			t.Fatalf("failed to create test event: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewDashboardHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/events/%s?client_key=%s", eventID, clientKey), nil)
		c.Params = gin.Params{
			{Key: "event_id", Value: eventID},
		}

		handler.HandleDeleteEvent(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["status"] != "deleted" {
			t.Errorf("expected status 'deleted', got %v", response["status"])
		}

		// Verify event is actually deleted
		_, err = eventService.GetEventByID(c.Request.Context(), uuid.MustParse(eventID))
		if err == nil {
			t.Error("expected event to be deleted")
		}
	})
}

func TestHandleDeleteEvent_MissingClientKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewDashboardHandler(keyService, eventService)

		eventID := uuid.New().String()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/events/"+eventID, nil)
		c.Params = gin.Params{
			{Key: "event_id", Value: eventID},
		}

		handler.HandleDeleteEvent(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "client_key query parameter is required" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleDeleteEvent_InvalidEventIDFormat(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, _, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewDashboardHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/events/invalid-uuid?client_key="+clientKey, nil)
		c.Params = gin.Params{
			{Key: "event_id", Value: "invalid-uuid"},
		}

		handler.HandleDeleteEvent(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "invalid event_id format" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleDeleteEvent_InvalidClientKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewDashboardHandler(keyService, eventService)

		eventID := uuid.New().String()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/events/"+eventID+"?client_key=invalid-key", nil)
		c.Params = gin.Params{
			{Key: "event_id", Value: eventID},
		}

		handler.HandleDeleteEvent(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "invalid client key" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleDeleteEvent_EventNotFound(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, _, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewDashboardHandler(keyService, eventService)

		nonExistentEventID := uuid.New().String()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/events/"+nonExistentEventID+"?client_key="+clientKey, nil)
		c.Params = gin.Params{
			{Key: "event_id", Value: nonExistentEventID},
		}

		handler.HandleDeleteEvent(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "event not found" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleDeleteEvent_Forbidden(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		// Create first user with event
		webhookKeyID1, _, _, _, err := tdb.CreateTestKeyPair(111111, "user1")
		if err != nil {
			t.Fatalf("failed to create test key pair 1: %v", err)
		}

		eventID, err := tdb.CreateTestEvent(webhookKeyID1, "/test", []byte(`{"test":"data"}`))
		if err != nil {
			t.Fatalf("failed to create test event: %v", err)
		}

		// Create second user (different client key)
		_, _, _, clientKey2, err := tdb.CreateTestKeyPair(222222, "user2")
		if err != nil {
			t.Fatalf("failed to create test key pair 2: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewDashboardHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/events/"+eventID+"?client_key="+clientKey2, nil)
		c.Params = gin.Params{
			{Key: "event_id", Value: eventID},
		}

		// Execute - user2 trying to delete user1's event
		handler.HandleDeleteEvent(c)

		// Assert
		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "event does not belong to this client" {
			t.Errorf("unexpected error: %v", response["error"])
		}

		// Verify event still exists
		_, err = eventService.GetEventByID(c.Request.Context(), uuid.MustParse(eventID))
		if err != nil {
			t.Error("expected event to still exist")
		}
	})
}

// Suppress unused import warning
var _ = models.Event{}
