package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/config"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/database"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/handlers"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/logging"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/middleware"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize structured logging
	logging.Setup(logging.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})

	log.Info().
		Int("port", cfg.Port).
		Str("log_level", cfg.LogLevel).
		Msg("starting server")

	// Initialize database
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	db, err := database.New(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer db.Close()

	log.Info().Msg("database connected")

	// Initialize JWT secret in middleware
	if err := middleware.SetJWTSecret(cfg.JWTSecret); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize JWT secret")
	}

	// Initialize encryption (optional — empty key disables)
	encryptor, err := services.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize encryption")
	}
	if encryptor != nil {
		log.Info().Msg("event data encryption enabled (AES-256-GCM)")
	} else {
		log.Info().Msg("event data encryption disabled (ENCRYPTION_KEY not set)")
	}

	// Initialize services
	keyService := services.NewKeyService(db.GetPool())
	eventService := services.NewEventServiceWithEncryption(db.GetPool(), encryptor)
	adminService := services.NewAdminService(db.GetPool())
	cleanupService := services.NewCleanupService(db.GetPool(), cfg.EnableAutoCleanup)

	// Auto-seed admin user on first run (if ADMIN_USERNAME and ADMIN_PASSWORD are set)
	if cfg.AdminUsername != "" && cfg.AdminPassword != "" {
		hasAdmins, err := adminService.HasAdmins(context.Background())
		if err != nil {
			log.Error().Err(err).Msg("failed to check for existing admin users")
		} else if !hasAdmins {
			if _, err := adminService.CreateAdminUser(context.Background(), cfg.AdminUsername, cfg.AdminPassword); err != nil {
				log.Error().Err(err).Msg("failed to create initial admin user")
			} else {
				log.Info().Str("username", cfg.AdminUsername).Msg("initial admin user created")
			}
		}
	}

	// Initialize email authentication services
	var emailService *services.EmailService
	var mailerliteService *services.MailerLiteService
	var authService *services.AuthService

	if cfg.MailgunAPIKey != "" && cfg.MailgunDomain != "" {
		emailService = services.NewEmailService(
			cfg.MailgunDomain,
			cfg.MailgunAPIKey,
			cfg.MailgunFromEmail,
			cfg.MailgunFromName,
		)
		log.Info().Str("domain", cfg.MailgunDomain).Msg("Mailgun email service initialized")
	} else {
		log.Warn().Msg("Mailgun credentials not configured - email authentication disabled")
	}

	if cfg.MailerLiteAPIKey != "" {
		mailerliteService = services.NewMailerLiteService(
			cfg.MailerLiteAPIKey,
			cfg.MailerLiteGroupSignups,
			cfg.MailerLiteGroupActive,
		)
		log.Info().Msg("MailerLite service initialized")
	} else {
		log.Warn().Msg("MailerLite API key not configured - marketing automation disabled")
	}

	authService = services.NewAuthService(
		db.GetPool(),
		cfg.JWTSecret,
		cfg.MagicLinkExpiry,
		cfg.MagicLinkBaseURL,
	)
	log.Info().Int("expiry_seconds", cfg.MagicLinkExpiry).Msg("Auth service initialized")

	// Initialize Analytics Service
	analyticsService, err := services.NewAnalyticsService(services.AnalyticsConfig{
		PostHogAPIKey: cfg.PostHogAPIKey,
		PostHogHost:   cfg.PostHogHost,
		Enabled:       cfg.PostHogEnabled,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize analytics service")
	}
	defer analyticsService.Close()

	if cfg.PostHogEnabled {
		log.Info().Str("host", cfg.PostHogHost).Msg("PostHog analytics enabled")
	} else {
		log.Info().Msg("PostHog analytics disabled")
	}

	// Start background services
	go cleanupService.Start(context.Background())

	// Create Gin router
	router := gin.New()

	// Add middleware
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.LoggingMiddleware())
	router.Use(gin.Recovery())

	// Add CORS middleware to allow Obsidian plugin (app://obsidian.md) and web browsers
	corsConfig := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// Allow Obsidian app origins
			if origin == "app://obsidian.md" || origin == "capacitor://localhost" {
				return true
			}
			// Allow localhost for development
			if origin == "http://localhost" || origin == "http://localhost:8080" || origin == "http://localhost:8081" {
				return true
			}
			// Allow production domain
			if origin == "https://obsidian-webhooks.khabaroff.studio" {
				return true
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))

	// Setup routes
	setupRoutes(router, db, keyService, eventService, adminService, analyticsService, emailService, mailerliteService, authService, cfg)

	// Create HTTP server with timeouts (G112: protect from Slowloris attack)
	srv := &http.Server{
		Addr:              ":" + formatPort(cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // защита от Slowloris
		ReadTimeout:       30 * time.Second, // общий timeout на чтение
		WriteTimeout:      30 * time.Second, // timeout на запись
	}

	// Start server in goroutine
	go func() {
		log.Info().Int("port", cfg.Port).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR2)
	sig := <-sigChan

	log.Info().Str("signal", sig.String()).Msg("received shutdown signal")

	// Stop cleanup service
	cleanupService.Stop()

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("server shutdown error")
	}

	log.Info().Msg("server shut down successfully")
}

func setupRoutes(router *gin.Engine, db *database.Database, keyService *services.KeyService, eventService *services.EventService, adminService *services.AdminService, analyticsService *services.AnalyticsService, emailService *services.EmailService, mailerliteService *services.MailerLiteService, authService *services.AuthService, cfg *config.Config) {
	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db)
	webhookHandler := handlers.NewWebhookHandler(keyService, eventService, analyticsService)
	sseHandler := handlers.NewSSEHandler(keyService, eventService, cfg.AllowedOrigins)
	ackHandler := handlers.NewACKHandler(keyService, eventService)

	// Wire up SSE broadcaster for real-time event delivery
	webhookHandler.SetBroadcaster(sseHandler)
	dashboardHandler := handlers.NewDashboardHandler(keyService, eventService)
	adminHandler := handlers.NewAdminHandler(keyService, adminService, eventService)
	// Email authentication handlers (only if services are configured)
	var authHandler *handlers.AuthHandler
	var dashboardHandlerNew *handlers.DashboardHandler
	if emailService != nil && authService != nil {
		authHandler = handlers.NewAuthHandler(authService, emailService, mailerliteService, analyticsService)
		dashboardHandlerNew = handlers.NewDashboardHandlerWithAuth(db.GetPool(), authService, keyService)
		log.Info().Msg("Email authentication handlers initialized")
	}

	// Landing pages (English)
	router.GET("/", func(c *gin.Context) {
		c.File("./src/templates/index.html")
	})

	// Guides (English)
	router.GET("/guides/", func(c *gin.Context) {
		c.File("./src/templates/guides/index.html")
	})
	router.GET("/guides", func(c *gin.Context) {
		c.Redirect(301, "/guides/")
	})
	router.GET("/guides/how-it-works", func(c *gin.Context) {
		c.File("./src/templates/guides/how-it-works.html")
	})
	router.GET("/guides/receive-data-obsidian", func(c *gin.Context) {
		c.File("./src/templates/guides/receive-data-obsidian.html")
	})
	router.GET("/guides/self-hosted-webhooks-setup", func(c *gin.Context) {
		c.File("./src/templates/guides/self-hosted-webhooks-setup.html")
	})
	router.GET("/guides/webhook-recipes", func(c *gin.Context) {
		c.File("./src/templates/guides/webhook-recipes.html")
	})
	router.GET("/guides/ai-agents-obsidian", func(c *gin.Context) {
		c.File("./src/templates/guides/ai-agents-obsidian.html")
	})
	router.GET("/guides/rest-api-vs-webhooks", func(c *gin.Context) {
		c.File("./src/templates/guides/rest-api-vs-webhooks.html")
	})

	// Landing pages (Russian)
	router.GET("/ru/", func(c *gin.Context) {
		c.File("./src/templates/index_ru.html")
	})
	router.GET("/ru", func(c *gin.Context) {
		c.Redirect(301, "/ru/")
	})

	// Guides (Russian)
	router.GET("/ru/guides/", func(c *gin.Context) {
		c.File("./src/templates/guides/index_ru.html")
	})
	router.GET("/ru/guides", func(c *gin.Context) {
		c.Redirect(301, "/ru/guides/")
	})
	router.GET("/ru/guides/how-it-works", func(c *gin.Context) {
		c.File("./src/templates/guides/how-it-works_ru.html")
	})
	router.GET("/ru/guides/receive-data-obsidian", func(c *gin.Context) {
		c.File("./src/templates/guides/receive-data-obsidian_ru.html")
	})
	router.GET("/ru/guides/self-hosted-webhooks-setup", func(c *gin.Context) {
		c.File("./src/templates/guides/self-hosted-webhooks-setup_ru.html")
	})
	router.GET("/ru/guides/webhook-recipes", func(c *gin.Context) {
		c.File("./src/templates/guides/webhook-recipes_ru.html")
	})
	router.GET("/ru/guides/ai-agents-obsidian", func(c *gin.Context) {
		c.File("./src/templates/guides/ai-agents-obsidian_ru.html")
	})
	router.GET("/ru/guides/rest-api-vs-webhooks", func(c *gin.Context) {
		c.File("./src/templates/guides/rest-api-vs-webhooks_ru.html")
	})
	router.GET("/ru/login", func(c *gin.Context) {
		c.File("./src/templates/user_login_ru.html")
	})

	// Health check endpoints
	router.GET("/health", healthHandler.HandleHealth)
	router.GET("/ready", healthHandler.HandleReady)
	router.GET("/info", healthHandler.HandleInfo)

	// Webhook endpoint
	router.POST("/webhook/:webhook_key",
		middleware.ValidateWebhookKey(keyService),
		middleware.NewRateLimitingMiddleware(middleware.RateLimitConfig{
			RequestsPerMinute: 100,
			Burst:             20,
		}),
		middleware.WebhookSignatureMiddleware(cfg.WebhookSecret, cfg.EnableWebhookSignatureVerification),
		webhookHandler.HandleWebhook)

	// SSE endpoint (for streaming and polling)
	router.GET("/events/:client_key", middleware.ValidateClientKey(keyService), sseHandler.HandleSSE)

	// Test endpoint (plugin uses this to verify full flow via client key)
	router.POST("/test/:client_key", middleware.ValidateClientKey(keyService), webhookHandler.HandleTestWebhook)

	// ACK endpoint
	router.POST("/ack/:client_key/:event_id", middleware.ValidateClientKey(keyService), ackHandler.HandleACK)

	// Dashboard endpoints (require admin authentication)
	router.GET("/dashboard/events", middleware.AdminAuthMiddleware(), dashboardHandler.HandleGetEvents)
	router.DELETE("/dashboard/events/:event_id", middleware.AdminAuthMiddleware(), dashboardHandler.HandleDeleteEvent)

	// Admin authentication endpoints
	router.POST("/admin/login", adminHandler.HandleAdminLogin)
	router.POST("/admin/logout", middleware.AdminAuthMiddleware(), adminHandler.HandleAdminLogout)
	router.GET("/admin/status", middleware.AdminAuthMiddleware(), adminHandler.HandleAdminStatus)

	// Admin endpoints (all require authentication)
	router.POST("/admin/activate", middleware.AdminAuthMiddleware(), adminHandler.HandleActivateLicense)
	router.POST("/admin/deactivate", middleware.AdminAuthMiddleware(), adminHandler.HandleDeactivateLicense)
	router.GET("/admin/keys", middleware.AdminAuthMiddleware(), adminHandler.HandleListKeys)
	router.GET("/admin/users", middleware.AdminAuthMiddleware(), adminHandler.HandleListUsers)
	router.GET("/admin/alerts/undelivered", middleware.AdminAuthMiddleware(), adminHandler.HandleUndeliveredAlerts)
	// Admin dashboard (serve static files and admin panel HTML)
	router.Static("/static", "./static")
	router.Static("/assets", "./src/templates/assets")
	router.Static("/plugin", "./plugin/release")
	router.StaticFile("/login", "./src/templates/user_login.html")
	router.StaticFile("/admin", "./src/templates/admin.html")
	router.StaticFile("/robots.txt", "./static/robots.txt")
	router.StaticFile("/sitemap.xml", "./static/sitemap.xml")

	// Email authentication routes (only if handlers are configured)
	if authHandler != nil && dashboardHandlerNew != nil {
		// Auth endpoints with rate limiting (3 requests per minute per IP)
		authGroup := router.Group("/auth")
		authGroup.Use(middleware.AuthRateLimitMiddleware())
		{
			authGroup.POST("/register", authHandler.HandleRegister)
			authGroup.POST("/request-login", authHandler.HandleRequestLogin)
		}

		// Magic link verification (no rate limit - single-use tokens)
		router.GET("/auth/verify", authHandler.HandleVerifyMagicLink)

		// Logout endpoint
		router.POST("/auth/logout", dashboardHandlerNew.HandleLogout)

		// Dashboard routes
		router.GET("/dashboard", dashboardHandlerNew.HandleDashboardPage)
		router.GET("/dashboard/api/me", dashboardHandlerNew.HandleGetUserData)
		router.GET("/dashboard/api/logs", dashboardHandlerNew.HandleGetLogs)
		router.POST("/dashboard/api/revoke", dashboardHandlerNew.HandleRevokeKeys)
		router.POST("/dashboard/api/keys/new", dashboardHandlerNew.HandleCreateNewKeyPair)

		log.Info().Msg("Email authentication routes registered")
	}
}

func formatPort(port int) string {
	return fmt.Sprintf("%d", port)
}
