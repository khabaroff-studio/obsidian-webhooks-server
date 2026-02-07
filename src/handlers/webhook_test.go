package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/database"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

func TestHandleWebhook_Success(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		// Create test webhook key
		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		reqBody := []byte(`{"test": "data"}`)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey+"?path=/test/path", bytes.NewReader(reqBody))
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", response["status"])
		}

		if response["event_id"] == nil || response["event_id"] == "" {
			t.Error("expected event_id to be set")
		}
	})
}

func TestHandleWebhook_MissingPath(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey, nil)
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "path query parameter is required" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleWebhook_PathTooLong(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		// Path longer than 512 characters
		longPath := strings.Repeat("a", 513)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey+"?path="+longPath, nil)
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "path too long (max 512 characters)" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleWebhook_PathTraversal(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey+"?path=../../etc/passwd", nil)
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "invalid path (path traversal not allowed)" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleWebhook_InvalidWebhookKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/wh_invalid?path=/test", nil)
		c.Params = gin.Params{
			{Key: "webhook_key", Value: "wh_invalid"},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "invalid webhook key" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleWebhook_PayloadTooLarge(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		// Create payload larger than 10MB
		largePayload := bytes.Repeat([]byte("a"), 11*1024*1024)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey+"?path=/test", bytes.NewReader(largePayload))
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected status 413, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "payload too large (max 10MB)" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}
