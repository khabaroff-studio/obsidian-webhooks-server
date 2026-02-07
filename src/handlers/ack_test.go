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
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

func TestHandleACK_Success(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		// Create test user with key pair
		webhookKeyID, clientKeyID, webhookKey, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		// Create test event
		eventID, err := tdb.CreateTestEvent(webhookKeyID, "/test", []byte(`{"test":"data"}`))
		if err != nil {
			t.Fatalf("failed to create test event: %v", err)
		}

		// Create services and handler
		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewACKHandler(keyService, eventService)

		// Setup HTTP request
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/ack/%s/%s", clientKey, eventID), nil)
		c.Params = gin.Params{
			{Key: "client_key", Value: clientKey},
			{Key: "event_id", Value: eventID},
		}

		// Execute
		handler.HandleACK(c)

		// Assert
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["status"] != "acknowledged" {
			t.Errorf("expected status 'acknowledged', got %v", response["status"])
		}

		if response["event_id"] != eventID {
			t.Errorf("expected event_id %s, got %v", eventID, response["event_id"])
		}

		// Verify event is marked as processed
		event, err := eventService.GetEventByID(c.Request.Context(), uuid.MustParse(eventID))
		if err != nil {
			t.Fatalf("failed to get event: %v", err)
		}

		if !event.Processed {
			t.Error("expected event to be marked as processed")
		}

		// Prevent unused variable warnings
		_ = webhookKey
		_ = clientKeyID
	})
}

func TestHandleACK_InvalidEventIDFormat(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, _, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewACKHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/ack/"+clientKey+"/invalid-uuid", nil)
		c.Params = gin.Params{
			{Key: "client_key", Value: clientKey},
			{Key: "event_id", Value: "invalid-uuid"},
		}

		// Execute
		handler.HandleACK(c)

		// Assert
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "invalid event_id format" {
			t.Errorf("expected error 'invalid event_id format', got %v", response["error"])
		}
	})
}

func TestHandleACK_InvalidClientKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewACKHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		fakeEventID := uuid.New().String()
		c.Request = httptest.NewRequest(http.MethodPost, "/ack/invalid-key/"+fakeEventID, nil)
		c.Params = gin.Params{
			{Key: "client_key", Value: "invalid-key"},
			{Key: "event_id", Value: fakeEventID},
		}

		// Execute
		handler.HandleACK(c)

		// Assert
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "invalid client key" {
			t.Errorf("expected error 'invalid client key', got %v", response["error"])
		}
	})
}

func TestHandleACK_EventNotFound(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, _, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewACKHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		nonExistentEventID := uuid.New().String()
		c.Request = httptest.NewRequest(http.MethodPost, "/ack/"+clientKey+"/"+nonExistentEventID, nil)
		c.Params = gin.Params{
			{Key: "client_key", Value: clientKey},
			{Key: "event_id", Value: nonExistentEventID},
		}

		// Execute
		handler.HandleACK(c)

		// Assert
		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "event not found" {
			t.Errorf("expected error 'event not found', got %v", response["error"])
		}
	})
}

func TestHandleACK_Forbidden(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
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
		handler := NewACKHandler(keyService, eventService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/ack/"+clientKey2+"/"+eventID, nil)
		c.Params = gin.Params{
			{Key: "client_key", Value: clientKey2},
			{Key: "event_id", Value: eventID},
		}

		// Execute - user2 trying to ACK user1's event
		handler.HandleACK(c)

		// Assert
		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "event does not belong to this client" {
			t.Errorf("expected error 'event does not belong to this client', got %v", response["error"])
		}
	})
}

func TestHandleACK_Idempotent(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
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
		handler := NewACKHandler(keyService, eventService)

		// First ACK
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/ack/%s/%s", clientKey, eventID), nil)
		c1.Params = gin.Params{
			{Key: "client_key", Value: clientKey},
			{Key: "event_id", Value: eventID},
		}
		handler.HandleACK(c1)

		if w1.Code != http.StatusOK {
			t.Fatalf("first ACK failed: %d", w1.Code)
		}

		// Second ACK (idempotent)
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/ack/%s/%s", clientKey, eventID), nil)
		c2.Params = gin.Params{
			{Key: "client_key", Value: clientKey},
			{Key: "event_id", Value: eventID},
		}
		handler.HandleACK(c2)

		// Assert - second ACK should also return 200
		if w2.Code != http.StatusOK {
			t.Errorf("expected status 200 for idempotent ACK, got %d", w2.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w2.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["status"] != "acknowledged" {
			t.Errorf("expected status 'acknowledged', got %v", response["status"])
		}
	})
}
