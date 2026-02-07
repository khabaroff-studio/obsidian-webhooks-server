package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
	"github.com/rs/zerolog/log"
)

// SSEEvent carries event data with metadata for delivery tracking
type SSEEvent struct {
	EventID      uuid.UUID
	WebhookKeyID uuid.UUID
	Data         string
}

const (
	maxPathLength = 512
	maxBodySize   = 10 * 1024 * 1024 // 10MB
)

// EventBroadcaster is an interface for broadcasting events to connected clients
type EventBroadcaster interface {
	BroadcastEvent(event interface{})
}

// WebhookHandler handles webhook POST requests
type WebhookHandler struct {
	keyService       *services.KeyService
	eventService     *services.EventService
	broadcaster      EventBroadcaster
	analyticsService *services.AnalyticsService
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(keyService *services.KeyService, eventService *services.EventService, analyticsService *services.AnalyticsService) *WebhookHandler {
	return &WebhookHandler{
		keyService:       keyService,
		eventService:     eventService,
		analyticsService: analyticsService,
	}
}

// SetBroadcaster sets the event broadcaster for real-time delivery
func (wh *WebhookHandler) SetBroadcaster(broadcaster EventBroadcaster) {
	wh.broadcaster = broadcaster
}

// HandleTestWebhook creates a test event using the client key's paired webhook key.
// This allows the plugin to test the full flow without knowing the webhook key.
func (wh *WebhookHandler) HandleTestWebhook(c *gin.Context) {
	clientKey := c.Param("client_key")

	ck, err := wh.keyService.GetClientKeyByValue(c.Request.Context(), clientKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid client key"})
		return
	}

	testData := []byte(`{"test":true,"source":"obsidian-plugin"}`)

	event, err := wh.eventService.CreateEvent(
		c.Request.Context(),
		ck.WebhookKeyID,
		"_test/connection-test.md",
		testData,
		1*time.Hour,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create test event"})
		return
	}

	if wh.broadcaster != nil {
		wh.broadcaster.BroadcastEvent(SSEEvent{
			EventID:      event.ID,
			WebhookKeyID: ck.WebhookKeyID,
			Data:         formatEventForSSE(event),
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "event_id": event.ID})
}

// formatEventForSSE formats an event for SSE delivery
func formatEventForSSE(event *models.Event) string {
	// Escape data for JSON - need to properly encode bytes to string
	dataStr := string(event.Data)
	// Use json.Marshal to properly escape special characters
	dataJSON, _ := json.Marshal(dataStr)

	return fmt.Sprintf(`{"id":"%s","path":"%s","data":%s,"created_at":"%s"}`,
		event.ID, event.Path, string(dataJSON), event.CreatedAt.Format(time.RFC3339))
}

// HandleWebhook processes incoming webhook requests
func (wh *WebhookHandler) HandleWebhook(c *gin.Context) {
	webhookKey := c.Param("webhook_key")
	path := c.Query("path")

	// Validate path parameter - required
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "path query parameter is required",
		})
		return
	}

	// Validate path length
	if len(path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "path too long (max 512 characters)",
		})
		return
	}

	// Validate path - no traversal attacks
	if strings.Contains(path, "..") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid path (path traversal not allowed)",
		})
		return
	}

	// Get webhook key info
	wk, err := wh.keyService.GetWebhookKeyByValue(c.Request.Context(), webhookKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid webhook key",
		})
		return
	}

	// Read request body with size limit (regardless of Content-Length header)
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodySize+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to read request body",
		})
		return
	}

	// Check if body exceeded limit
	if len(body) > maxBodySize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "payload too large (max 10MB)",
		})
		return
	}

	// Create event
	event, err := wh.eventService.CreateEvent(
		c.Request.Context(),
		wk.ID,
		path,
		body,
		24*365*time.Hour, // Default TTL
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create event",
		})
		return
	}

	// Create webhook log entry (pending)
	if err := wh.keyService.CreateWebhookLog(c.Request.Context(), event.ID, wk.ID, http.StatusOK); err != nil {
		log.Warn().Err(err).Str("event_id", event.ID.String()).Msg("failed to create webhook log")
	}

	// Update usage stats (last_used + usage_count)
	if err := wh.keyService.UpdateKeyUsageStats(c.Request.Context(), wk.ID); err != nil {
		log.Warn().Err(err).Str("webhook_key_id", wk.ID.String()).Msg("failed to update usage stats")
	}

	// Track webhook received event with email hash
	if wh.analyticsService != nil {
		email, err := wh.keyService.GetEmailByWebhookKeyValue(c.Request.Context(), webhookKey)
		if err == nil && email != "" {
			wh.analyticsService.TrackWebhookReceived(
				c.Request.Context(),
				services.HashEmail(email),
				len(body),
			)
		}
	}

	// Broadcast event to connected SSE clients for real-time delivery
	if wh.broadcaster != nil {
		wh.broadcaster.BroadcastEvent(SSEEvent{
			EventID:      event.ID,
			WebhookKeyID: wk.ID,
			Data:         formatEventForSSE(event),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"event_id": event.ID,
	})
}
