package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// limiterEntry holds a rate limiter with last used timestamp
type limiterEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

// keyRateLimiter manages per-key rate limiters with automatic cleanup
type keyRateLimiter struct {
	limiters map[string]*limiterEntry
	mu       sync.RWMutex
	limit    rate.Limit
	burst    int
	stopCh   chan struct{}
}

func newKeyRateLimiter(limit rate.Limit, burst int) *keyRateLimiter {
	k := &keyRateLimiter{
		limiters: make(map[string]*limiterEntry),
		limit:    limit,
		burst:    burst,
		stopCh:   make(chan struct{}),
	}
	// Start cleanup goroutine
	go k.cleanupLoop()
	return k
}

func (k *keyRateLimiter) getLimiter(key string) *rate.Limiter {
	k.mu.RLock()
	entry, ok := k.limiters[key]
	k.mu.RUnlock()
	if ok {
		// Update last used time
		k.mu.Lock()
		entry.lastUsed = time.Now()
		k.mu.Unlock()
		return entry.limiter
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	// Double-check under write lock
	if entry, ok = k.limiters[key]; ok {
		entry.lastUsed = time.Now()
		return entry.limiter
	}
	limiter := rate.NewLimiter(k.limit, k.burst)
	k.limiters[key] = &limiterEntry{
		limiter:  limiter,
		lastUsed: time.Now(),
	}
	return limiter
}

// cleanupLoop removes stale entries every 5 minutes
func (k *keyRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			k.cleanup()
		case <-k.stopCh:
			return
		}
	}
}

// cleanup removes entries not used in the last 10 minutes
func (k *keyRateLimiter) cleanup() {
	k.mu.Lock()
	defer k.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for key, entry := range k.limiters {
		if entry.lastUsed.Before(cutoff) {
			delete(k.limiters, key)
		}
	}
}

// Stop terminates the cleanup goroutine
func (k *keyRateLimiter) Stop() {
	close(k.stopCh)
}

// RateLimitConfig defines configuration for the rate limiting middleware
type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
}

// NewRateLimitingMiddleware creates a Gin middleware that enforces per-webhook_key limits
func NewRateLimitingMiddleware(cfg RateLimitConfig) gin.HandlerFunc {
	// Default sensible values if not provided
	if cfg.RequestsPerMinute <= 0 {
		cfg.RequestsPerMinute = 100
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 20
	}

	limit := rate.Every(time.Minute / time.Duration(cfg.RequestsPerMinute))
	limiter := newKeyRateLimiter(limit, cfg.Burst)

	return func(c *gin.Context) {
		webhookKey := c.Param("webhook_key")
		if webhookKey == "" {
			// Fallback: no key -> apply single shared limiter
			webhookKey = "__global__"
		}

		l := limiter.getLimiter(webhookKey)
		if !l.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":  "rate limit exceeded",
				"detail": "Too many requests for this webhook key. Try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// NewIPRateLimitingMiddleware creates a Gin middleware that enforces per-IP limits
// Useful for authentication endpoints to prevent abuse
func NewIPRateLimitingMiddleware(cfg RateLimitConfig) gin.HandlerFunc {
	// Default values: 3 requests per minute for auth endpoints
	if cfg.RequestsPerMinute <= 0 {
		cfg.RequestsPerMinute = 3
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 1
	}

	limit := rate.Every(time.Minute / time.Duration(cfg.RequestsPerMinute))
	limiter := newKeyRateLimiter(limit, cfg.Burst)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		l := limiter.getLimiter(ip)
		if !l.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests. Please try again later.",
				"retry_after": "60s",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthRateLimitMiddleware is a pre-configured middleware for authentication endpoints
// Allows 3 requests per minute per IP address
func AuthRateLimitMiddleware() gin.HandlerFunc {
	return NewIPRateLimitingMiddleware(RateLimitConfig{
		RequestsPerMinute: 3,
		Burst:             1,
	})
}
