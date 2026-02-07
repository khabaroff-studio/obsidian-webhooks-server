package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

// formatEventToJSON formats an event to JSON string including data field
func formatEventToJSON(event *models.Event) string {
	dataStr := string(event.Data)
	dataJSON, _ := json.Marshal(dataStr)

	return fmt.Sprintf(`{"id":"%s","path":"%s","data":%s,"created_at":"%s"}`,
		event.ID, event.Path, string(dataJSON), event.CreatedAt.Format(time.RFC3339))
}

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	keyService     *services.KeyService
	eventService   *services.EventService
	clients        map[string]chan interface{}
	mu             sync.RWMutex
	allowedOrigins string
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(keyService *services.KeyService, eventService *services.EventService, allowedOrigins string) *SSEHandler {
	return &SSEHandler{
		keyService:     keyService,
		eventService:   eventService,
		clients:        make(map[string]chan interface{}),
		allowedOrigins: allowedOrigins,
	}
}

// addClient safely adds a client to the map
func (sh *SSEHandler) addClient(key string, ch chan interface{}) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.clients[key] = ch
}

// removeClient safely removes a client from the map
func (sh *SSEHandler) removeClient(key string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	delete(sh.clients, key)
}

// HandleSSE handles SSE connections and polling
func (sh *SSEHandler) HandleSSE(c *gin.Context) {
	clientKey := c.Param("client_key")

	// Check if polling mode
	if c.Query("poll") == "true" {
		sh.handlePolling(c, clientKey)
		return
	}

	sh.handleSSEStream(c, clientKey)
}

// handleSSEStream handles real-time SSE connections
func (sh *SSEHandler) handleSSEStream(c *gin.Context, clientKey string) {
	// Get client key info
	ck, err := sh.keyService.GetClientKeyByValue(c.Request.Context(), clientKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid client key",
		})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Set CORS header based on configuration
	if sh.allowedOrigins != "" {
		c.Header("Access-Control-Allow-Origin", sh.allowedOrigins)
	}

	// Send initial comment and flush to establish connection immediately
	_, _ = c.Writer.WriteString(": connected\n\n") // Ignore error, connection will fail anyway
	c.Writer.Flush()

	// Get unprocessed events
	events, err := sh.eventService.GetUnprocessedEvents(c.Request.Context(), ck.WebhookKeyID)
	if err != nil {
		log.Printf("Error getting unprocessed events: %v", err)
	}

	// Send existing unprocessed events
	for _, event := range events {
		// Marshal event data properly
		eventJSON := formatEventToJSON(&event)
		data := fmt.Sprintf("data: %s\n\n", eventJSON)
		_, _ = c.Writer.WriteString(data) // Ignore error, connection will fail anyway
		c.Writer.Flush()
		// Update webhook log to delivered
		_ = sh.keyService.UpdateWebhookLogDelivered(c.Request.Context(), event.ID, event.WebhookKeyID, ck.ID)
	}

	// Create channel for new events
	eventChan := make(chan interface{}, 10)
	sh.addClient(clientKey, eventChan)
	defer sh.removeClient(clientKey)

	// Setup heartbeat
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			log.Printf("SSE client disconnected: %s", clientKey)
			return
		case <-ticker.C:
			// Send heartbeat
			_, _ = c.Writer.WriteString(": heartbeat\n\n") // Ignore error, connection will fail anyway
			c.Writer.Flush()
		case rawEvent := <-eventChan:
			if rawEvent != nil {
				var eventStr string
				if sseEvent, ok := rawEvent.(SSEEvent); ok {
					eventStr = fmt.Sprintf("data: %s\n\n", sseEvent.Data)
					_ = sh.keyService.UpdateWebhookLogDelivered(c.Request.Context(), sseEvent.EventID, sseEvent.WebhookKeyID, ck.ID)
				} else {
					eventStr = fmt.Sprintf("data: %v\n\n", rawEvent)
				}
				_, _ = c.Writer.WriteString(eventStr) // Ignore error, connection will fail anyway
				c.Writer.Flush()
			}
		}
	}
}

// handlePolling handles polling requests
func (sh *SSEHandler) handlePolling(c *gin.Context, clientKey string) {
	// Set CORS headers for Obsidian plugin (app:// protocol)
	if sh.allowedOrigins != "" {
		c.Header("Access-Control-Allow-Origin", sh.allowedOrigins)
	}

	// Get client key info
	ck, err := sh.keyService.GetClientKeyByValue(c.Request.Context(), clientKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid client key",
		})
		return
	}

	// Get unprocessed events
	events, err := sh.eventService.GetUnprocessedEvents(c.Request.Context(), ck.WebhookKeyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get events",
		})
		return
	}

	// Format events with proper data field for plugin consumption
	formattedEvents := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		dataStr := string(event.Data)
		formattedEvent := map[string]interface{}{
			"id":         event.ID,
			"path":       event.Path,
			"data":       dataStr,
			"created_at": event.CreatedAt.Format(time.RFC3339),
		}
		formattedEvents = append(formattedEvents, formattedEvent)
	}

	// Return array directly (not wrapped in object) - plugin expects WebhookEvent[]
	c.JSON(http.StatusOK, formattedEvents)
}

// BroadcastEvent broadcasts an event to all connected SSE clients
func (sh *SSEHandler) BroadcastEvent(event interface{}) {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	for _, ch := range sh.clients {
		select {
		case ch <- event:
		default:
			log.Println("Event channel full, dropping event")
		}
	}
}
