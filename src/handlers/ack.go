package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

// ACKHandler handles event acknowledgment
type ACKHandler struct {
	keyService   *services.KeyService
	eventService *services.EventService
}

// NewACKHandler creates a new ACK handler
func NewACKHandler(keyService *services.KeyService, eventService *services.EventService) *ACKHandler {
	return &ACKHandler{
		keyService:   keyService,
		eventService: eventService,
	}
}

// HandleACK marks an event as acknowledged
func (ah *ACKHandler) HandleACK(c *gin.Context) {
	clientKey := c.Param("client_key")
	eventIDStr := c.Param("event_id")

	// Parse event ID
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid event_id format",
		})
		return
	}

	// Validate client key
	ck, err := ah.keyService.GetClientKeyByValue(c.Request.Context(), clientKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid client key",
		})
		return
	}

	// Get event
	event, err := ah.eventService.GetEventByID(c.Request.Context(), eventID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "event not found",
		})
		return
	}

	// Verify event belongs to the correct webhook key
	if event.WebhookKeyID != ck.WebhookKeyID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "event does not belong to this client",
		})
		return
	}

	// Mark event as processed (idempotent)
	err = ah.eventService.MarkEventAsProcessed(c.Request.Context(), eventID)
	if err != nil {
		// If event already processed, return 200 anyway (idempotent)
		c.JSON(http.StatusOK, gin.H{
			"status":   "acknowledged",
			"event_id": eventID,
		})
		return
	}

	// Update webhook log to acked
	_ = ah.keyService.UpdateWebhookLogAcked(c.Request.Context(), eventID)

	c.JSON(http.StatusOK, gin.H{
		"status":   "acknowledged",
		"event_id": eventID,
	})
}
