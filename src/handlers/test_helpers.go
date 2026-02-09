package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Test helpers for handler tests

// createTestContext creates a test Gin context with recorder
func createTestContext() (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return w, c
}

// assertStatusCode checks if response status code matches expected
func assertStatusCode(t *testing.T, w *httptest.ResponseRecorder, expectedCode int) {
	t.Helper()
	if w.Code != expectedCode {
		t.Errorf("expected status %d, got %d: %s", expectedCode, w.Code, w.Body.String())
	}
}

// assertJSONError checks if response contains expected error message
func assertJSONError(t *testing.T, w *httptest.ResponseRecorder, expectedError string) {
	t.Helper()
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response["error"] != expectedError {
		t.Errorf("expected error '%s', got '%v'", expectedError, response["error"])
	}
}
