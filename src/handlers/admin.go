package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/middleware"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

// AdminHandler handles admin operations
type AdminHandler struct {
	keyService   *services.KeyService
	adminService *services.AdminService
	eventService *services.EventService
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(keyService *services.KeyService, adminService *services.AdminService, eventService *services.EventService) *AdminHandler {
	return &AdminHandler{
		keyService:   keyService,
		adminService: adminService,
		eventService: eventService,
	}
}

// ActivateLicenseRequest represents the request body for activation
type ActivateLicenseRequest struct {
	WebhookKey string `json:"webhook_key"`
	ClientKey  string `json:"client_key"`
}

// DeactivateLicenseRequest represents the request body for deactivation
type DeactivateLicenseRequest struct {
	WebhookKey string `json:"webhook_key"`
	ClientKey  string `json:"client_key"`
}

// keyOperationFunc defines a function type for key activation/deactivation
type keyOperationFunc func(ctx context.Context, keyValue string, isWebhookKey bool) error

// handleKeyOperation is a helper that reduces duplication between activate/deactivate
func (ah *AdminHandler) handleKeyOperation(c *gin.Context, webhookKey, clientKey string, operation keyOperationFunc, action string) {
	// Process webhook key if provided
	if webhookKey != "" {
		if err := operation(c.Request.Context(), webhookKey, true); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to %s webhook key", action),
			})
			return
		}
	}

	// Process client key if provided
	if clientKey != "" {
		if err := operation(c.Request.Context(), clientKey, false); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to %s client key", action),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": fmt.Sprintf("%sd", action), // "activated" or "deactivated"
	})
}

// HandleActivateLicense activates a webhook and client key
func (ah *AdminHandler) HandleActivateLicense(c *gin.Context) {
	var req ActivateLicenseRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	ah.handleKeyOperation(c, req.WebhookKey, req.ClientKey, ah.keyService.ActivateKey, "activate")
}

// HandleDeactivateLicense deactivates a webhook and client key
func (ah *AdminHandler) HandleDeactivateLicense(c *gin.Context) {
	var req DeactivateLicenseRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	ah.handleKeyOperation(c, req.WebhookKey, req.ClientKey, ah.keyService.DeactivateKey, "deactivate")
}

// HandleListKeys lists all key pairs (webhook + client together)
func (ah *AdminHandler) HandleListKeys(c *gin.Context) {
	keys, err := ah.keyService.GetWebhookKeys(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list keys",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"keys": keys,
	})
}

// AdminLoginRequest represents the request body for admin login
type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AdminLoginResponse represents the response for successful login
type AdminLoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// HandleAdminLogin authenticates admin user and returns JWT token
func (ah *AdminHandler) HandleAdminLogin(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Authenticate admin
	admin, err := ah.adminService.AuthenticateAdmin(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid username or password",
		})
		return
	}

	// Generate JWT token
	token, err := middleware.GenerateAdminToken(admin.ID, admin.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to generate token",
		})
		return
	}

	// Set cookie
	expiresAt := time.Now().Add(24 * time.Hour)
	c.SetCookie(
		"admin_token",
		token,
		int(24*time.Hour.Seconds()),
		"/",
		"",
		true, // Secure
		true, // HttpOnly
	)

	c.JSON(http.StatusOK, AdminLoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
	})
}

// HandleAdminLogout clears the admin token cookie
func (ah *AdminHandler) HandleAdminLogout(c *gin.Context) {
	c.SetCookie(
		"admin_token",
		"",
		-1,
		"/",
		"",
		true, // Secure
		true, // HttpOnly
	)

	c.JSON(http.StatusOK, gin.H{
		"status": "logged out",
	})
}

// AdminStatusResponse represents the response for admin status check
type AdminStatusResponse struct {
	Authenticated bool   `json:"authenticated"`
	AdminID       string `json:"admin_id"`
	Username      string `json:"username"`
}

// HandleAdminStatus returns the current admin authentication status
func (ah *AdminHandler) HandleAdminStatus(c *gin.Context) {
	adminID, _ := c.Get("admin_id")
	username, _ := c.Get("username")

	c.JSON(http.StatusOK, AdminStatusResponse{
		Authenticated: true,
		AdminID:       adminID.(string),
		Username:      username.(string),
	})
}

// UserListResponse represents a list of users with total count
type UserListResponse struct {
	Users []services.User `json:"users"`
	Total int             `json:"total"`
}

// HandleListUsers returns all users with their key information
func (ah *AdminHandler) HandleListUsers(c *gin.Context) {
	users, err := ah.keyService.GetUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list users",
		})
		return
	}

	c.JSON(http.StatusOK, UserListResponse{
		Users: users,
		Total: len(users),
	})
}

// HandleUndeliveredAlerts returns count of undelivered events older than 1 hour
func (ah *AdminHandler) HandleUndeliveredAlerts(c *gin.Context) {
	count, err := ah.eventService.CountUndeliveredEvents(c.Request.Context(), 1*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to count undelivered events",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"undelivered_count": count,
		"threshold_hours":   1,
	})
}
