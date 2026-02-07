package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuthService handles authentication logic and magic link lifecycle
type AuthService struct {
	db              *pgxpool.Pool
	jwtSecret       string
	magicLinkExpiry time.Duration
	baseURL         string
}

// NewAuthService creates a new authentication service
func NewAuthService(db *pgxpool.Pool, jwtSecret string, magicLinkExpirySeconds int, baseURL string) *AuthService {
	return &AuthService{
		db:              db,
		jwtSecret:       jwtSecret,
		magicLinkExpiry: time.Duration(magicLinkExpirySeconds) * time.Second,
		baseURL:         baseURL,
	}
}

// ErrTokenExpired indicates the magic link token has expired
var ErrTokenExpired = errors.New("magic link has expired")

// ErrTokenUsed indicates the magic link token was already used
var ErrTokenUsed = errors.New("magic link has already been used")

// ErrTokenInvalid indicates the magic link token is invalid
var ErrTokenInvalid = errors.New("invalid magic link token")

// GenerateMagicLinkToken generates a cryptographically secure token (32 bytes, base64url)
func (s *AuthService) GenerateMagicLinkToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// StoreMagicLinkToken stores a magic link token in the database for a given email
func (s *AuthService) StoreMagicLinkToken(ctx context.Context, email string, token string) error {
	expiresAt := time.Now().Add(s.magicLinkExpiry)

	query := `
		UPDATE api_keys
		SET magic_link_token = $1,
		    magic_link_expires_at = $2,
		    magic_link_used_at = NULL
		WHERE user_email = $3 AND key_type = 'webhook'
	`

	result, err := s.db.Exec(ctx, query, token, expiresAt, email)
	if err != nil {
		return fmt.Errorf("failed to store magic link token: %w", err)
	}

	rowsAffected := result.RowsAffected()

	if rowsAffected == 0 {
		return fmt.Errorf("no webhook key found for email: %s", email)
	}

	return nil
}

// VerifyMagicLinkToken verifies a magic link token and marks it as used
func (s *AuthService) VerifyMagicLinkToken(ctx context.Context, token string) (email string, err error) {
	var expiresAt, usedAt *time.Time

	query := `
		SELECT user_email, magic_link_expires_at, magic_link_used_at
		FROM api_keys
		WHERE magic_link_token = $1 AND key_type = 'webhook'
	`

	err = s.db.QueryRow(ctx, query, token).Scan(&email, &expiresAt, &usedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrTokenInvalid
		}
		return "", fmt.Errorf("failed to verify magic link token: %w", err)
	}

	// Check if already used
	if usedAt != nil {
		return "", ErrTokenUsed
	}

	// Check if expired
	if expiresAt == nil || time.Now().After(*expiresAt) {
		return "", ErrTokenExpired
	}

	// Mark token as used
	updateQuery := `
		UPDATE api_keys
		SET magic_link_used_at = NOW()
		WHERE magic_link_token = $1
	`

	if _, err := s.db.Exec(ctx, updateQuery, token); err != nil {
		return "", fmt.Errorf("failed to mark token as used: %w", err)
	}

	return email, nil
}

// GenerateSessionToken generates a JWT session token for authenticated users
func (s *AuthService) GenerateSessionToken(email string, expiryHours int) (string, error) {
	claims := jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(time.Duration(expiryHours) * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w", err)
	}

	return signedToken, nil
}

// VerifySessionToken verifies a JWT session token and returns the email
func (s *AuthService) VerifySessionToken(tokenString string) (email string, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse JWT token: %w", err)
	}

	if !token.Valid {
		return "", errors.New("invalid JWT token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid JWT claims")
	}

	email, ok = claims["email"].(string)
	if !ok {
		return "", errors.New("email not found in JWT claims")
	}

	return email, nil
}

// GetMagicLinkURL builds the full magic link URL with token
func (s *AuthService) GetMagicLinkURL(token string) string {
	return fmt.Sprintf("%s/auth/verify?token=%s", s.baseURL, token)
}

// CreateUserKeyPair creates a new webhook+client key pair for a user with email
func (s *AuthService) CreateUserKeyPair(ctx context.Context, email, name, language string) (webhookKey, clientKey string, err error) {
	// Default to English if language not specified
	if language == "" {
		language = "en"
	}

	// Generate webhook key
	webhookKey = "wh_" + generateRandomKey(24)
	clientKey = "ck_" + generateRandomKey(24)

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert webhook key with preferred language
	webhookKeyID := uuid.New()
	insertWebhookQuery := `
		INSERT INTO api_keys (id, key_value, key_type, pair_id, user_email, user_name, email_verified, preferred_language)
		VALUES ($1, $2, 'webhook', $1, $3, $4, true, $5)
	`

	if _, err := tx.Exec(ctx, insertWebhookQuery, webhookKeyID, webhookKey, email, name, language); err != nil {
		return "", "", fmt.Errorf("failed to insert webhook key: %w", err)
	}

	// Insert client key with same preferred language
	insertClientQuery := `
		INSERT INTO api_keys (key_value, key_type, pair_id, user_email, user_name, email_verified, preferred_language)
		VALUES ($1, 'client', $2, $3, $4, true, $5)
	`

	if _, err := tx.Exec(ctx, insertClientQuery, clientKey, webhookKeyID, email, name, language); err != nil {
		return "", "", fmt.Errorf("failed to insert client key: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return webhookKey, clientKey, nil
}

// GetUserLanguage retrieves user's preferred language by email
func (s *AuthService) GetUserLanguage(ctx context.Context, email string) (string, error) {
	query := `
		SELECT preferred_language
		FROM api_keys
		WHERE user_email = $1 AND key_type = 'webhook'
		LIMIT 1
	`

	var language string
	err := s.db.QueryRow(ctx, query, email).Scan(&language)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "en", nil // Default to English if user not found
		}
		return "en", fmt.Errorf("failed to get user language: %w", err)
	}

	return language, nil
}

// GetUserByEmail retrieves user information by email
func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (exists bool, emailVerified bool, err error) {
	query := `
		SELECT email_verified
		FROM api_keys
		WHERE user_email = $1 AND key_type = 'webhook'
		LIMIT 1
	`

	var verified bool
	err = s.db.QueryRow(ctx, query, email).Scan(&verified)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, false, nil
		}
		return false, false, fmt.Errorf("failed to query user by email: %w", err)
	}

	return true, verified, nil
}

// generateRandomKey generates a random key string for webhook/client keys
func generateRandomKey(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to UUID-based generation if crypto/rand fails
		return uuid.New().String()[:length]
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}
