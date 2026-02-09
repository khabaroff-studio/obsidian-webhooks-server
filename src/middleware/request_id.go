package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDKey is the context key for request ID
const RequestIDKey = "request_id"

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for existing request ID in header
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate short UUID for readability
			requestID = uuid.New().String()[:8]
		}

		// Store in context
		c.Set(RequestIDKey, requestID)

		// Add to response header
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// GetRequestID retrieves the request ID from context
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(RequestIDKey); exists {
		return id.(string)
	}
	return ""
}
