package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"github.com/posthog/posthog-go"
	"github.com/rs/zerolog/log"
)

// HashEmail returns a hex-encoded SHA-256 hash of the email for use as PostHog distinct ID
func HashEmail(email string) string {
	h := sha256.Sum256([]byte(email))
	return fmt.Sprintf("%x", h)
}

// AnalyticsService handles all product analytics tracking
type AnalyticsService struct {
	client  posthog.Client
	enabled bool
}

type posthogLogger struct{}

func (l posthogLogger) Success(m posthog.APIMessage) {
	log.Info().Str("type", fmt.Sprintf("%T", m)).Msg("PostHog event delivered")
}

func (l posthogLogger) Failure(m posthog.APIMessage, err error) {
	log.Error().Err(err).Str("type", fmt.Sprintf("%T", m)).Msg("PostHog delivery failed")
}

// AnalyticsConfig holds analytics configuration
type AnalyticsConfig struct {
	PostHogAPIKey string
	PostHogHost   string
	Enabled       bool
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(cfg AnalyticsConfig) (*AnalyticsService, error) {
	if !cfg.Enabled {
		return &AnalyticsService{enabled: false}, nil
	}

	if cfg.PostHogAPIKey == "" {
		return &AnalyticsService{enabled: false}, nil
	}

	client, err := posthog.NewWithConfig(
		cfg.PostHogAPIKey,
		posthog.Config{
			Endpoint:  cfg.PostHogHost,
			Interval:  30 * time.Second,
			BatchSize: 100,
			Callback: posthogLogger{},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostHog client: %w", err)
	}

	return &AnalyticsService{
		client:  client,
		enabled: true,
	}, nil
}

// Close flushes pending events and closes client
func (s *AnalyticsService) Close() error {
	if !s.enabled {
		return nil
	}
	return s.client.Close()
}

// getEnvironment returns current environment (production, staging, development)
func getEnvironment() string {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		return "production"
	}
	return env
}

// TrackEvent captures a generic event
func (s *AnalyticsService) TrackEvent(ctx context.Context, distinctID, event string, properties map[string]interface{}) {
	if !s.enabled {
		return
	}

	// Add common properties
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["timestamp"] = time.Now().Unix()
	properties["environment"] = getEnvironment()

	if err := s.client.Enqueue(posthog.Capture{
		DistinctId: distinctID,
		Event:      event,
		Properties: properties,
	}); err != nil {
		log.Error().Err(err).Str("event", event).Msg("PostHog enqueue failed")
	} else {
		log.Info().Str("event", event).Str("distinct_id", distinctID).Msg("PostHog event enqueued")
	}
}

// Identify sets user properties
func (s *AnalyticsService) Identify(ctx context.Context, distinctID string, properties map[string]interface{}) {
	if !s.enabled {
		return
	}

	s.client.Enqueue(posthog.Identify{
		DistinctId: distinctID,
		Properties: properties,
	})
}

// Alias merges two user identities
func (s *AnalyticsService) Alias(ctx context.Context, distinctID, alias string) {
	if !s.enabled {
		return
	}

	s.client.Enqueue(posthog.Alias{
		DistinctId: distinctID,
		Alias:      alias,
	})
}

// TrackRegistrationStarted tracks when a user submits the registration form
func (s *AnalyticsService) TrackRegistrationStarted(ctx context.Context, emailHash string) {
	s.TrackEvent(ctx, "email_"+emailHash, "registration_started", nil)
}

// TrackAccountActivated tracks when a user verifies their magic link
func (s *AnalyticsService) TrackAccountActivated(ctx context.Context, emailHash string) {
	s.TrackEvent(ctx, "email_"+emailHash, "account_activated", nil)
}

// TrackWebhookReceived tracks incoming webhook
func (s *AnalyticsService) TrackWebhookReceived(ctx context.Context, emailHash string, payloadSize int) {
	s.TrackEvent(ctx, "email_"+emailHash, "webhook_received", map[string]interface{}{
		"payload_size_bytes": payloadSize,
	})
}
