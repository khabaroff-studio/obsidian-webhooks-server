package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	authService       *services.AuthService
	emailService      *services.EmailService
	mailerliteService *services.MailerLiteService
	analyticsService  *services.AnalyticsService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	authService *services.AuthService,
	emailService *services.EmailService,
	mailerliteService *services.MailerLiteService,
	analyticsService *services.AnalyticsService,
) *AuthHandler {
	return &AuthHandler{
		authService:       authService,
		emailService:      emailService,
		mailerliteService: mailerliteService,
		analyticsService:  analyticsService,
	}
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required,min=1,max=255"`
	Language string `json:"language" binding:"omitempty,oneof=en ru"`
}

// RequestLoginRequest represents a login request
type RequestLoginRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// HandleRegister handles POST /auth/register - new user registration
func (h *AuthHandler) HandleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid email or name format",
			"details": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Check if user already exists
	exists, emailVerified, err := h.authService.GetUserByEmail(ctx, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to check user existence",
		})
		return
	}

	if exists && emailVerified {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "user_exists",
			"message": "An account with this email already exists. Please use 'Request Login' instead.",
		})
		return
	}

	// Create key pair for new user
	language := req.Language
	if language == "" {
		language = "en" // Default to English
	}
	webhookKey, clientKey, err := h.authService.CreateUserKeyPair(ctx, req.Email, req.Name, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to create user account",
		})
		return
	}

	// Generate magic link token
	token, err := h.authService.GenerateMagicLinkToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to generate authentication token",
		})
		return
	}

	// Store token in database
	if err := h.authService.StoreMagicLinkToken(ctx, req.Email, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to store authentication token",
		})
		return
	}

	// Send magic link email
	magicLink := h.authService.GetMagicLinkURL(token)
	// language already defined above
	if err := h.emailService.SendMagicLinkEmail(ctx, req.Email, req.Name, magicLink, 60, language); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to send verification email. Please try again.",
		})
		return
	}

	// Track registration_started
	if h.analyticsService != nil {
		h.analyticsService.TrackRegistrationStarted(ctx, services.HashEmail(req.Email))
	}

	// Add subscriber to MailerLite with language tag (async, non-blocking)
	if h.mailerliteService != nil {
		go func() {
			bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer bgCancel()
			_ = h.mailerliteService.AddSubscriber(bgCtx, req.Email, req.Name, language)
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Registration successful! Please check your email for a magic link to sign in.",
		"email":        req.Email,
		"webhook_key":  webhookKey,
		"client_key":   clientKey,
		"expires_in":   "60 minutes",
	})
}

// HandleRequestLogin handles POST /auth/request-login - existing user login
func (h *AuthHandler) HandleRequestLogin(c *gin.Context) {
	var req RequestLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid email format",
			"details": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Check if user exists
	exists, emailVerified, err := h.authService.GetUserByEmail(ctx, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to verify email address",
		})
		return
	}

	if !exists || !emailVerified {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "user_not_found",
			"message": "No account found with this email. Please register first.",
		})
		return
	}

	// Generate magic link token
	token, err := h.authService.GenerateMagicLinkToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to generate authentication token",
		})
		return
	}

	// Store token in database
	if err := h.authService.StoreMagicLinkToken(ctx, req.Email, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to store authentication token",
		})
		return
	}

	// Send magic link email
	magicLink := h.authService.GetMagicLinkURL(token)
	// Read user's preferred language from database
	language, err := h.authService.GetUserLanguage(ctx, req.Email)
	if err != nil {
		language = "en" // Fallback to English on error
	}
	if err := h.emailService.SendMagicLinkEmail(ctx, req.Email, "", magicLink, 60, language); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to send login email. Please try again.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Magic link sent! Please check your email to sign in.",
		"email":      req.Email,
		"expires_in": "60 minutes",
	})
}

// HandleVerifyMagicLink handles GET /auth/verify?token=... - magic link verification
func (h *AuthHandler) HandleVerifyMagicLink(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Missing token parameter",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Verify token and get email
	email, err := h.authService.VerifyMagicLinkToken(ctx, token)
	if err != nil {
		if errors.Is(err, services.ErrTokenExpired) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "token_expired",
				"message": "This magic link has expired. Please request a new one.",
			})
			return
		}
		if errors.Is(err, services.ErrTokenUsed) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "token_used",
				"message": "This magic link has already been used. Please request a new one.",
			})
			return
		}
		if errors.Is(err, services.ErrTokenInvalid) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "token_invalid",
				"message": "Invalid magic link. Please request a new one.",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to verify magic link",
		})
		return
	}

	// Generate session token (JWT)
	sessionToken, err := h.authService.GenerateSessionToken(email, 24*30) // 30 days
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "server_error",
			"message": "Failed to create session",
		})
		return
	}

	// Set HTTP-only cookie
	c.SetCookie(
		"session_token",
		sessionToken,
		30*24*3600, // 30 days
		"/",
		"",
		true,  // Secure (HTTPS only)
		true,  // HttpOnly
	)

	// Track account_activated
	if h.analyticsService != nil {
		h.analyticsService.TrackAccountActivated(ctx, services.HashEmail(email))
	}

	// Move user to active group in MailerLite (async, non-blocking)
	if h.mailerliteService != nil {
		go func() {
			bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer bgCancel()
			_ = h.mailerliteService.MoveToActiveGroup(bgCtx, email)
		}()
	}

	// Send welcome email (async, non-blocking)
	go func() {
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer bgCancel()
		// Read user's preferred language from database
		language, err := h.authService.GetUserLanguage(bgCtx, email)
		if err != nil {
			language = "en" // Fallback to English on error
		}
		if err := h.emailService.SendWelcomeEmail(bgCtx, email, "", language); err != nil {
			// Log error but don't fail the verification
			// TODO: Add proper logging
		}
	}()

	// Redirect to dashboard
	c.Redirect(http.StatusFound, "/dashboard")
}
