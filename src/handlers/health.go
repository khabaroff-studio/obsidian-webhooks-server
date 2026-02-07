package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/database"
)

var startTime = time.Now()

// HealthHandler handles health check requests
type HealthHandler struct {
	db *database.Database
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *database.Database) *HealthHandler {
	return &HealthHandler{
		db: db,
	}
}

// HandleHealth returns health status with DB check
func (hh *HealthHandler) HandleHealth(c *gin.Context) {
	start := time.Now()
	err := hh.db.Health(c.Request.Context())
	dbLatency := time.Since(start)

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "unhealthy",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"database":   "connected",
		"db_latency": dbLatency.String(),
		"uptime":     time.Since(startTime).String(),
	})
}

// HandleInfo returns service information
func (hh *HealthHandler) HandleInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "obsidian-webhooks-selfhosted",
		"version": "2.0.0",
		"status":  "running",
		"uptime":  time.Since(startTime).String(),
	})
}

// HandleReady returns readiness status (for load balancers)
func (hh *HealthHandler) HandleReady(c *gin.Context) {
	err := hh.db.Health(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready": false,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ready": true,
	})
}
