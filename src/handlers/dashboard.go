package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

// DashboardHandler handles dashboard API requests
type DashboardHandler struct {
	keyService   *services.KeyService
	eventService *services.EventService
	db           *pgxpool.Pool
	authService  *services.AuthService
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(keyService *services.KeyService, eventService *services.EventService) *DashboardHandler {
	return &DashboardHandler{
		keyService:   keyService,
		eventService: eventService,
	}
}

// NewDashboardHandlerWithAuth creates a new dashboard handler with auth service
func NewDashboardHandlerWithAuth(db *pgxpool.Pool, authService *services.AuthService, keyService *services.KeyService) *DashboardHandler {
	return &DashboardHandler{
		db:          db,
		authService: authService,
		keyService:  keyService,
	}
}

// UserDashboardData represents the user's dashboard data
type UserDashboardData struct {
	Email string             `json:"email"`
	Name  string             `json:"name"`
	Keys  []services.KeyPair `json:"keys"`
}

// HandleDashboardPage serves the dashboard HTML page
func (dh *DashboardHandler) HandleDashboardPage(c *gin.Context) {
	c.File("./src/templates/dashboard.html")
}

// HandleGetUserData returns the authenticated user's data (GET /dashboard/api/me)
func (dh *DashboardHandler) HandleGetUserData(c *gin.Context) {
	cookie, err := c.Cookie("session_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	email, err := dh.authService.VerifySessionToken(cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get user name
	var name string
	_ = dh.db.QueryRow(ctx, `
		SELECT COALESCE(user_name, '') FROM api_keys
		WHERE user_email = $1 AND key_type = 'webhook' LIMIT 1
	`, email).Scan(&name)

	// Get all key pairs
	keys, err := dh.keyService.GetUserKeyPairs(ctx, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user data"})
		return
	}
	if keys == nil {
		keys = []services.KeyPair{}
	}

	c.JSON(http.StatusOK, UserDashboardData{
		Email: email,
		Name:  name,
		Keys:  keys,
	})
}

// HandleLogout logs out the user by clearing the session cookie
func (dh *DashboardHandler) HandleLogout(c *gin.Context) {
	// Clear session cookie
	c.SetCookie(
		"session_token",
		"",
		-1, // Expire immediately
		"/",
		"",
		true, // Secure (HTTPS only)
		true, // HttpOnly
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// HandleGetEvents returns paginated events
func (dh *DashboardHandler) HandleGetEvents(c *gin.Context) {
	clientKey := c.Query("client_key")
	if clientKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "client_key query parameter is required",
		})
		return
	}

	// Validate client key
	ck, err := dh.keyService.GetClientKeyByValue(c.Request.Context(), clientKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid client key",
		})
		return
	}

	// Get events (default limit 100)
	limit := 100
	events, err := dh.eventService.GetEventsByWebhookKey(c.Request.Context(), ck.WebhookKeyID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get events",
		})
		return
	}

	if events == nil {
		events = []models.Event{}
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"count":  len(events),
	})
}

// HandleDeleteEvent deletes a specific event
func (dh *DashboardHandler) HandleDeleteEvent(c *gin.Context) {
	clientKey := c.Query("client_key")
	if clientKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "client_key query parameter is required",
		})
		return
	}

	eventIDStr := c.Param("event_id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid event_id format",
		})
		return
	}

	// Validate client key
	ck, err := dh.keyService.GetClientKeyByValue(c.Request.Context(), clientKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid client key",
		})
		return
	}

	// Get event to verify it belongs to the client
	event, err := dh.eventService.GetEventByID(c.Request.Context(), eventID)
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

	// Delete event
	err = dh.eventService.DeleteEvent(c.Request.Context(), eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete event",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "deleted",
		"event_id": eventID,
	})
}

// HandleGetLogs returns webhook logs for the authenticated user (GET /dashboard/api/logs)
func (dh *DashboardHandler) HandleGetLogs(c *gin.Context) {
	cookie, err := c.Cookie("session_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	email, err := dh.authService.VerifySessionToken(cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logs, total, err := dh.keyService.GetUserWebhookLogEntries(ctx, email, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get logs"})
		return
	}

	if logs == nil {
		logs = []services.WebhookLogEntry{}
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
	})
}

// HandleRevokeKeys deactivates a specific key pair (POST /dashboard/api/revoke)
func (dh *DashboardHandler) HandleRevokeKeys(c *gin.Context) {
	cookie, err := c.Cookie("session_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	email, err := dh.authService.VerifySessionToken(cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		PairID string `json:"pair_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pair_id is required"})
		return
	}

	pairID, err := uuid.Parse(req.PairID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pair_id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := dh.keyService.DeactivateKeyPairByID(ctx, pairID, email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// HandleCreateNewKeyPair creates a new key pair for the user (POST /dashboard/api/keys/new)
func (dh *DashboardHandler) HandleCreateNewKeyPair(c *gin.Context) {
	cookie, err := c.Cookie("session_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	email, err := dh.authService.VerifySessionToken(cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get user info from existing keys
	var name, language string
	err = dh.db.QueryRow(ctx, `
		SELECT COALESCE(user_name, ''), COALESCE(preferred_language, 'en')
		FROM api_keys WHERE user_email = $1 AND key_type = 'webhook' LIMIT 1
	`, email).Scan(&name, &language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user info"})
		return
	}

	// Create new pair (old keys stay as-is)
	webhookKey, clientKey, err := dh.authService.CreateUserKeyPair(ctx, email, name, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create new keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"webhook_key": webhookKey,
		"client_key":  clientKey,
	})
}
