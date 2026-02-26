package config

import (
	cryptoRand "crypto/rand"
	"os"
	"strconv"
	"time"
)

// Config holds application configuration
type Config struct {
	Port                               int
	DatabaseURL                        string
	JWTSecret                          string
	EventTTL                           time.Duration
	EnableAutoCleanup                  bool
	ExternalHost                       string
	WebhookSecret                      string
	EnableWebhookSignatureVerification bool
	AllowedOrigins                     string
	LogLevel                           string
	LogFormat                          string

	// PostHog Analytics settings
	PostHogAPIKey  string
	PostHogHost    string
	PostHogEnabled bool

	// Email Authentication settings
	MailgunDomain         string
	MailgunAPIKey         string
	MailgunFromEmail      string
	MailgunFromName       string
	MailerLiteAPIKey      string
	MailerLiteGroupSignups string
	MailerLiteGroupActive  string
	MagicLinkExpiry        int    // seconds
	MagicLinkBaseURL       string

	// Encryption at rest
	EncryptionKey string // 64 hex chars = 32 bytes AES-256 key; empty = disabled

	// Admin auto-seed (first run only)
	AdminUsername string
	AdminPassword string
}

// Load loads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		Port:                               getEnvInt("PORT", 8080),
		DatabaseURL:                        getEnv("DATABASE_URL", "postgres://user:password@localhost/obsidian_webhooks"),
		JWTSecret:                          getEnv("JWT_SECRET", ""),
		EventTTL:                           time.Duration(getEnvInt("EVENT_TTL_DAYS", 30)) * 24 * time.Hour,
		EnableAutoCleanup:                  getEnvBool("ENABLE_AUTO_CLEANUP", true),
		ExternalHost:                       getEnv("EXTERNAL_HOST", "http://localhost:8080"),
		WebhookSecret:                      getEnv("WEBHOOK_SECRET", ""),
		EnableWebhookSignatureVerification: getEnvBool("ENABLE_WEBHOOK_SIGNATURE_VERIFICATION", false),
		AllowedOrigins:                     getEnv("ALLOWED_ORIGINS", ""),
		LogLevel:                           getEnv("LOG_LEVEL", "info"),
		LogFormat:                          getEnv("LOG_FORMAT", "json"),

		// PostHog Analytics
		PostHogAPIKey:  getEnv("POSTHOG_API_KEY", ""),
		PostHogHost:    getEnv("POSTHOG_HOST", "https://eu.i.posthog.com"),
		PostHogEnabled: getEnvBool("POSTHOG_ENABLED", false),

		// Email Authentication
		MailgunDomain:          getEnv("MAILGUN_DOMAIN", ""),
		MailgunAPIKey:          getEnv("MAILGUN_API_KEY", ""),
		MailgunFromEmail:       getEnv("MAILGUN_FROM_EMAIL", "noreply@obsidian-webhooks.khabaroff.studio"),
		MailgunFromName:        getEnv("MAILGUN_FROM_NAME", "Khabaroff Studio: Obsidian Webhooks"),
		MailerLiteAPIKey:       getEnv("MAILERLITE_API_KEY", ""),
		MailerLiteGroupSignups: getEnv("MAILERLITE_GROUP_SIGNUPS", ""),
		MailerLiteGroupActive:  getEnv("MAILERLITE_GROUP_ACTIVE", ""),
		MagicLinkExpiry:        getEnvInt("MAGIC_LINK_EXPIRY", 3600), // 1 hour default
		MagicLinkBaseURL:       getEnv("MAGIC_LINK_BASE_URL", "http://localhost:8080"),

		// Encryption
		EncryptionKey: getEnv("ENCRYPTION_KEY", ""),

		// Admin auto-seed
		AdminUsername: getEnv("ADMIN_USERNAME", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
	}

	// Generate JWT secret if not provided
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = generateRandomSecret(32)
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// generateRandomSecret generates a cryptographically secure random secret for JWT signing
func generateRandomSecret(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	if _, err := cryptoRand.Read(result); err != nil {
		panic("failed to generate random secret: " + err.Error())
	}
	for i := range result {
		result[i] = charset[result[i]%byte(len(charset))]
	}
	return string(result)
}

