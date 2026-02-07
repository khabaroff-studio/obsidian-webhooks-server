package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// UserClaims contains JWT claims for user authentication
type UserClaims struct {
	WebhookKeyID string `json:"webhook_key_id"`
	jwt.RegisteredClaims
}

// GenerateUserToken creates a JWT token for a user (valid 24 hours)
// Uses the global JWTSecret initialized at startup
func GenerateUserToken(webhookKeyID string) (string, error) {
	if JWTSecret == "" {
		return "", fmt.Errorf("JWT secret not initialized")
	}

	claims := UserClaims{
		WebhookKeyID: webhookKeyID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "obsidian-webhooks",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWTSecret))
}

// ValidateUserToken parses and validates a JWT token
// Uses the global JWTSecret initialized at startup
func ValidateUserToken(tokenString string) (*UserClaims, error) {
	if JWTSecret == "" {
		return nil, fmt.Errorf("JWT secret not initialized")
	}

	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// UserAuthMiddleware validates user JWT token from Authorization header or cookie
func UserAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get token from Authorization header
		tokenString := c.GetHeader("Authorization")
		if tokenString != "" && len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		} else {
			// Try to get from cookie
			var err error
			tokenString, err = c.Cookie("user_token")
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "missing or invalid token",
				})
				c.Abort()
				return
			}
		}

		// Validate token
		claims, err := ValidateUserToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			c.Abort()
			return
		}

		// Store in context
		c.Set("webhook_key_id", claims.WebhookKeyID)
		c.Next()
	}
}
