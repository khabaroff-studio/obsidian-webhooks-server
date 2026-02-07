package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

// validateKey is a generic key validation helper
func validateKey(c *gin.Context, paramName string, contextKey string, keyType string, validateFunc func(c *gin.Context, key string) (bool, error)) {
	key := c.Param(paramName)
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("%s parameter is required", paramName),
		})
		c.Abort()
		return
	}

	// Validate key exists and is active
	isValid, err := validateFunc(c, key)
	if err != nil {
		if errors.Is(err, services.ErrKeyNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": fmt.Sprintf("invalid %s", keyType),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to validate %s", keyType),
			})
		}
		c.Abort()
		return
	}

	if !isValid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": fmt.Sprintf("invalid or inactive %s", keyType),
		})
		c.Abort()
		return
	}

	// Store in context for later use
	c.Set(contextKey, key)
	c.Next()
}

// ValidateWebhookKey validates webhook key from URL parameter
func ValidateWebhookKey(keyService *services.KeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		validateKey(c, "webhook_key", "webhook_key", "webhook key", func(ctx *gin.Context, key string) (bool, error) {
			return keyService.ValidateWebhookKey(ctx.Request.Context(), key)
		})
	}
}

// ValidateClientKey validates client key from URL parameter
func ValidateClientKey(keyService *services.KeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		validateKey(c, "client_key", "client_key", "client key", func(ctx *gin.Context, key string) (bool, error) {
			return keyService.ValidateClientKey(ctx.Request.Context(), key)
		})
	}
}

// ValidateBearerToken validates JWT bearer token
func ValidateBearerToken(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format",
			})
			c.Abort()
			return
		}

		// Token validation would be done here with JWT package
		// For now, just pass the token through
		c.Set("token", parts[1])
		c.Next()
	}
}

// JWTSecret should be loaded from environment via config
var JWTSecret string

// SetJWTSecret initializes the JWT secret from config
func SetJWTSecret(secret string) error {
	if secret == "" {
		return fmt.Errorf("JWT_SECRET cannot be empty")
	}
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}
	JWTSecret = secret
	return nil
}

// AdminClaims represents JWT claims for admin users
type AdminClaims struct {
	AdminID  string `json:"admin_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateAdminToken creates a JWT token for admin user
func GenerateAdminToken(adminID uuid.UUID, username string) (string, error) {
	claims := AdminClaims{
		AdminID:  adminID.String(),
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "obsidian-webhooks",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWTSecret))
}

// ValidateAdminToken verifies JWT token and returns claims
func ValidateAdminToken(tokenString string) (*AdminClaims, error) {
	claims := &AdminClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return claims, nil
}

// AdminAuthMiddleware checks for valid JWT token in Cookie or Authorization header
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// Try to get token from cookie first
		cookie, err := c.Cookie("admin_token")
		if err == nil {
			token = cookie
		}

		// Fall back to Authorization header
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					token = parts[1]
				}
			}
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authentication token"})
			c.Abort()
			return
		}

		claims, err := ValidateAdminToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// Store admin info in context
		c.Set("admin_id", claims.AdminID)
		c.Set("username", claims.Username)
		c.Next()
	}
}
