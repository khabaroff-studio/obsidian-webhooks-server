package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/database"
)

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		middleware := RequestIDMiddleware()

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			requestID := GetRequestID(c)
			if requestID == "" {
				t.Error("expected request_id to be set in context")
			}
			c.JSON(http.StatusOK, gin.H{"request_id": requestID})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		// Check response header
		responseID := w.Header().Get("X-Request-ID")
		if responseID == "" {
			t.Error("expected X-Request-ID header in response")
		}

		// Should be 8 characters (short UUID)
		if len(responseID) != 8 {
			t.Errorf("expected request_id length 8, got %d", len(responseID))
		}
	})
}

func TestRequestIDMiddleware_UsesExistingID(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		middleware := RequestIDMiddleware()

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			requestID := GetRequestID(c)
			c.JSON(http.StatusOK, gin.H{"request_id": requestID})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.Header.Set("X-Request-ID", "custom-id")
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		// Check response header uses custom ID
		responseID := w.Header().Get("X-Request-ID")
		if responseID != "custom-id" {
			t.Errorf("expected X-Request-ID 'custom-id', got %s", responseID)
		}
	})
}

func TestGetRequestID_ReturnsEmpty(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

		// No request_id in context
		requestID := GetRequestID(c)
		if requestID != "" {
			t.Errorf("expected empty request_id, got %s", requestID)
		}
	})
}
