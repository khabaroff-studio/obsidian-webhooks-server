package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// WebhookSignatureMiddleware validates HMAC-SHA256 webhook signatures
func WebhookSignatureMiddleware(secret string, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If signature verification is disabled, just log a warning and continue
		if !enabled {
			log.Printf("[WARNING] Webhook signature verification is disabled. Enable it in production.")
			c.Next()
			return
		}

		// Get signature from header
		signature := c.GetHeader("X-Webhook-Signature")
		if signature == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing X-Webhook-Signature header",
			})
			c.Abort()
			return
		}

		// Get request body
		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "failed to read request body",
			})
			c.Abort()
			return
		}

		// Verify signature
		if !verifyWebhookSignature(body, signature, secret) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid webhook signature",
			})
			c.Abort()
			return
		}

		// Restore body for next handlers
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}

// verifyWebhookSignature verifies HMAC-SHA256 signature
func verifyWebhookSignature(body []byte, signature, secret string) bool {
	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	// Calculate expected signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// Compare signatures
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
