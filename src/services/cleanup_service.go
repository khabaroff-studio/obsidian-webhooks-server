package services

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CleanupService handles automatic cleanup of old events
type CleanupService struct {
	pool     *pgxpool.Pool
	enabled  bool
	interval time.Duration
	done     chan bool
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(pool *pgxpool.Pool, enabled bool) *CleanupService {
	return &CleanupService{
		pool:     pool,
		enabled:  enabled,
		interval: 24 * time.Hour, // Run daily
		done:     make(chan bool),
	}
}

// Start starts the cleanup service
func (cs *CleanupService) Start(ctx context.Context) {
	if !cs.enabled {
		log.Println("Cleanup service is disabled")
		return
	}

	go func() {
		ticker := time.NewTicker(cs.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("Cleanup service stopped")
				return
			case <-cs.done:
				log.Println("Cleanup service stopped")
				return
			case <-ticker.C:
				cs.cleanup(ctx)
			}
		}
	}()

	log.Println("Cleanup service started")
}

// Stop stops the cleanup service
func (cs *CleanupService) Stop() {
	cs.done <- true
}

// cleanup performs the actual cleanup
func (cs *CleanupService) cleanup(ctx context.Context) {
	result, err := cs.pool.Exec(ctx, "DELETE FROM events WHERE expires_at < NOW()")

	if err != nil {
		log.Printf("Cleanup error: %v", err)
		return
	}

	rowsDeleted := result.RowsAffected()
	if rowsDeleted > 0 {
		log.Printf("Cleanup completed: deleted %d expired events", rowsDeleted)
	}
}

// DeleteOldEvents manually deletes old events (called by cleanup)
func (cs *CleanupService) DeleteOldEvents(ctx context.Context, ttl time.Duration) (int64, error) {
	eventService := NewEventService(cs.pool)
	return eventService.DeleteExpiredEvents(ctx)
}
