package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/database"
)

func TestHandleHealth_Success(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

		db := database.NewDatabaseFromPool(tdb.Pool)
		handler := NewHealthHandler(db)

		// Execute
		handler.HandleHealth(c)

		// Assert
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", response["status"])
		}

		if response["database"] != "connected" {
			t.Errorf("expected database 'connected', got %v", response["database"])
		}

		if _, ok := response["db_latency"]; !ok {
			t.Error("expected db_latency field")
		}

		if _, ok := response["uptime"]; !ok {
			t.Error("expected uptime field")
		}
	})
}

func TestHandleHealth_DBError(t *testing.T) {
	// Setup - без активного DB connection
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	db := database.NewDatabaseFromPool(nil) // nil pool = DB error
	handler := NewHealthHandler(db)

	// Execute
	handler.HandleHealth(c)

	// Assert
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %v", response["status"])
	}

	if response["database"] != "disconnected" {
		t.Errorf("expected database 'disconnected', got %v", response["database"])
	}

	if _, ok := response["error"]; !ok {
		t.Error("expected error field")
	}
}

func TestHandleInfo(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/info", nil)

		db := database.NewDatabaseFromPool(tdb.Pool)
		handler := NewHealthHandler(db)

		// Execute
		handler.HandleInfo(c)

		// Assert
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["service"] != "obsidian-webhooks-selfhosted" {
			t.Errorf("expected service name, got %v", response["service"])
		}

		if _, ok := response["version"]; !ok {
			t.Error("expected version field")
		}

		if _, ok := response["uptime"]; !ok {
			t.Error("expected uptime field")
		}
	})
}

func TestHandleReady(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		// Setup
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

		db := database.NewDatabaseFromPool(tdb.Pool)
		handler := NewHealthHandler(db)

		// Execute
		handler.HandleReady(c)

		// Assert
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["ready"] != true {
			t.Errorf("expected ready true, got %v", response["ready"])
		}
	})
}

func TestHandleReady_DBError(t *testing.T) {
	// Setup - без DB connection
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	db := database.NewDatabaseFromPool(nil)
	handler := NewHealthHandler(db)

	// Execute
	handler.HandleReady(c)

	// Assert
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["ready"] != false {
		t.Errorf("expected ready false, got %v", response["ready"])
	}
}
