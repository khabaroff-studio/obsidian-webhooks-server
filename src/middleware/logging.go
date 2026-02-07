package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// LoggingMiddleware logs all HTTP requests with structured fields
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)
		status := c.Writer.Status()

		// Build log event
		event := log.Info()
		if status >= 500 {
			event = log.Error()
		} else if status >= 400 {
			event = log.Warn()
		}

		// Add fields
		event.
			Str("request_id", GetRequestID(c)).
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", status).
			Dur("duration", duration).
			Int("bytes", c.Writer.Size()).
			Str("client_ip", c.ClientIP())

		// Add query if present
		if query != "" {
			event.Str("query", query)
		}

		// Add error if present
		if len(c.Errors) > 0 {
			event.Str("error", c.Errors.String())
		}

		// Log message based on status
		switch {
		case status >= 500:
			event.Msg("server error")
		case status >= 400:
			event.Msg("client error")
		default:
			event.Msg("request")
		}
	}
}
