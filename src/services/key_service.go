package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
)

// User represents a user with their key information
type User struct {
	UserEmail       string     `json:"user_email"`
	UserName        string     `json:"user_name"`
	IsActive        bool       `json:"is_active"`
	WebhookKeyCount int        `json:"webhook_key_count"`
	ClientKeyCount  int        `json:"client_key_count"`
	WebhookKey      string     `json:"webhook_key"`
	ClientKey       string     `json:"client_key"`
	UsageCount      int        `json:"usage_count"`
	CreatedAt       time.Time  `json:"created_at"`
	LastUsed        *time.Time `json:"last_used"`
}

// WebhookLog represents a webhook delivery log entry
type WebhookLog struct {
	ID             string
	EventID        string
	WebhookKeyID   string
	ClientKeyID    *string
	DeliveryStatus string
	StatusCode     *int
	ErrorMessage   *string
	AttemptedAt    time.Time
	DeliveredAt    *time.Time
	AckedAt        *time.Time
	ClientIP       *string
	CreatedAt      time.Time
}

// KeyInfo represents basic key information for responses
type KeyInfo struct {
	ID       string
	KeyValue string
	KeyType  string
	IsActive bool
}

// KeyService handles key-related operations
type KeyService struct {
	pool *pgxpool.Pool
}

// NewKeyService creates a new key service
func NewKeyService(pool *pgxpool.Pool) *KeyService {
	return &KeyService{pool: pool}
}

// validateKey checks if a key exists and is active
// This is a private helper method to avoid duplication between ValidateWebhookKey and ValidateClientKey
func (ks *KeyService) validateKey(ctx context.Context, keyValue string, keyType models.KeyType) (bool, error) {
	var isActive bool

	query := `SELECT is_active FROM api_keys WHERE key_value = $1 AND key_type = $2`
	err := ks.pool.QueryRow(ctx, query, keyValue, string(keyType)).Scan(&isActive)

	if err != nil {
		return false, ErrKeyNotFound
	}

	return isActive, nil
}

// ValidateWebhookKey checks if webhook key exists and is active
func (ks *KeyService) ValidateWebhookKey(ctx context.Context, keyValue string) (bool, error) {
	return ks.validateKey(ctx, keyValue, models.KeyTypeWebhook)
}

// ValidateClientKey checks if client key exists and is active
func (ks *KeyService) ValidateClientKey(ctx context.Context, keyValue string) (bool, error) {
	return ks.validateKey(ctx, keyValue, models.KeyTypeClient)
}

// GetWebhookKeyByValue retrieves a webhook key by its value
func (ks *KeyService) GetWebhookKeyByValue(ctx context.Context, keyValue string) (*models.WebhookKey, error) {
	var wk models.WebhookKey
	err := ks.pool.QueryRow(ctx,
		"SELECT id, key_value, status, created_at, last_used, events_count FROM webhook_keys WHERE key_value = $1",
		keyValue,
	).Scan(&wk.ID, &wk.KeyValue, &wk.Status, &wk.CreatedAt, &wk.LastUsed, &wk.EventsCount)

	if err != nil {
		return nil, fmt.Errorf("webhook key not found: %w", err)
	}

	return &wk, nil
}

// GetEmailByWebhookKeyValue returns the user email associated with a webhook key value
func (ks *KeyService) GetEmailByWebhookKeyValue(ctx context.Context, keyValue string) (string, error) {
	var email string
	err := ks.pool.QueryRow(ctx,
		"SELECT COALESCE(user_email, '') FROM api_keys WHERE key_value = $1 AND key_type = 'webhook'",
		keyValue,
	).Scan(&email)
	if err != nil {
		return "", err
	}
	return email, nil
}

// GetClientKeyByValue retrieves a client key by its value
func (ks *KeyService) GetClientKeyByValue(ctx context.Context, keyValue string) (*models.ClientKey, error) {
	var ck models.ClientKey
	err := ks.pool.QueryRow(ctx,
		"SELECT id, key_value, webhook_key_id, status, created_at, last_connected, events_delivered FROM client_keys WHERE key_value = $1",
		keyValue,
	).Scan(&ck.ID, &ck.KeyValue, &ck.WebhookKeyID, &ck.Status, &ck.CreatedAt, &ck.LastConnected, &ck.EventsDelivered)

	if err != nil {
		return nil, fmt.Errorf("client key not found: %w", err)
	}

	return &ck, nil
}

// updateKeyStatus updates the active status of a key in the base table
// This is a private helper method to avoid duplication between activateKeyInTable and deactivateKeyInTable
func (ks *KeyService) updateKeyStatus(ctx context.Context, keyValue string, table string, isActive bool) error {
	// Determine key type from table name
	var keyType models.KeyType
	switch table {
	case models.TableWebhookKeys:
		keyType = models.KeyTypeWebhook
	case models.TableClientKeys:
		keyType = models.KeyTypeClient
	default:
		return fmt.Errorf("invalid table name: %s", table)
	}

	// Views are read-only, update base table instead
	query := "UPDATE api_keys SET is_active = $1 WHERE key_value = $2 AND key_type = $3"
	result, err := ks.pool.Exec(ctx, query, isActive, keyValue, string(keyType))
	if err != nil {
		return fmt.Errorf("failed to update key status: %w", err)
	}

	// Check if key was actually updated
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("key not found: %s", keyValue)
	}

	return nil
}

// activateKeyInTable activates a key in the specified table
func (ks *KeyService) activateKeyInTable(ctx context.Context, keyValue string, table string) error {
	return ks.updateKeyStatus(ctx, keyValue, table, true)
}

// ActivateKey activates a webhook or client key
func (ks *KeyService) ActivateKey(ctx context.Context, keyValue string, isWebhookKey bool) error {
	table := models.TableWebhookKeys
	if !isWebhookKey {
		table = models.TableClientKeys
	}

	return ks.activateKeyInTable(ctx, keyValue, table)
}

// deactivateKeyInTable deactivates a key in the specified table
func (ks *KeyService) deactivateKeyInTable(ctx context.Context, keyValue string, table string) error {
	return ks.updateKeyStatus(ctx, keyValue, table, false)
}

// DeactivateKey deactivates a webhook or client key
func (ks *KeyService) DeactivateKey(ctx context.Context, keyValue string, isWebhookKey bool) error {
	table := models.TableWebhookKeys
	if !isWebhookKey {
		table = models.TableClientKeys
	}

	return ks.deactivateKeyInTable(ctx, keyValue, table)
}

// CreateWebhookKey creates a new webhook key
func (ks *KeyService) CreateWebhookKey(ctx context.Context, keyValue string) (*models.WebhookKey, error) {
	wk := &models.WebhookKey{}
	err := ks.pool.QueryRow(ctx,
		"INSERT INTO webhook_keys (key_value, status, created_at) VALUES ($1, 'active', NOW()) RETURNING id, key_value, status, created_at, events_count",
		keyValue,
	).Scan(&wk.ID, &wk.KeyValue, &wk.Status, &wk.CreatedAt, &wk.EventsCount)

	if err != nil {
		return nil, fmt.Errorf("failed to create webhook key: %w", err)
	}

	return wk, nil
}

// CreateClientKey creates a new client key linked to a webhook key
func (ks *KeyService) CreateClientKey(ctx context.Context, keyValue string, webhookKeyID string) (*models.ClientKey, error) {
	ck := &models.ClientKey{}
	err := ks.pool.QueryRow(ctx,
		"INSERT INTO client_keys (key_value, webhook_key_id, status, created_at) VALUES ($1, $2, 'active', NOW()) RETURNING id, key_value, webhook_key_id, status, created_at, events_delivered",
		keyValue, webhookKeyID,
	).Scan(&ck.ID, &ck.KeyValue, &ck.WebhookKeyID, &ck.Status, &ck.CreatedAt, &ck.EventsDelivered)

	if err != nil {
		return nil, fmt.Errorf("failed to create client key: %w", err)
	}

	return ck, nil
}

// GetWebhookKeys returns all webhook keys with their paired client keys
func (ks *KeyService) GetWebhookKeys(ctx context.Context) ([]*models.WebhookKey, error) {
	rows, err := ks.pool.Query(ctx, `
		SELECT
			wh.id,
			wh.key_value,
			CASE WHEN wh.is_active THEN 'active' ELSE 'inactive' END as status,
			wh.created_at,
			wh.last_used,
			(SELECT COUNT(*) FROM events WHERE webhook_key_id = wh.id) as events_count,
			COALESCE(ck.key_value, '') as client_key_value
		FROM api_keys wh
		LEFT JOIN api_keys ck ON ck.pair_id = wh.id AND ck.key_type = 'client'
		WHERE wh.key_type = 'webhook'
		ORDER BY wh.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query webhook keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.WebhookKey
	for rows.Next() {
		wk := &models.WebhookKey{}
		var clientKeyValue string

		if err := rows.Scan(
			&wk.ID,
			&wk.KeyValue,
			&wk.Status,
			&wk.CreatedAt,
			&wk.LastUsed,
			&wk.EventsCount,
			&clientKeyValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan webhook key: %w", err)
		}

		wk.ClientKeyValue = clientKeyValue
		keys = append(keys, wk)
	}

	return keys, nil
}

// GetClientKeys returns all client keys
func (ks *KeyService) GetClientKeys(ctx context.Context) ([]*models.ClientKey, error) {
	rows, err := ks.pool.Query(ctx,
		"SELECT id, key_value, webhook_key_id, status, created_at, last_connected, events_delivered FROM client_keys ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query client keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.ClientKey
	for rows.Next() {
		ck := &models.ClientKey{}
		if err := rows.Scan(&ck.ID, &ck.KeyValue, &ck.WebhookKeyID, &ck.Status, &ck.CreatedAt, &ck.LastConnected, &ck.EventsDelivered); err != nil {
			return nil, fmt.Errorf("failed to scan client key: %w", err)
		}
		keys = append(keys, ck)
	}

	return keys, nil
}

// generateKeyValue generates a random key with the specified prefix
func (ks *KeyService) generateKeyValue(prefix string) (string, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return prefix + hex.EncodeToString(keyBytes), nil
}

// generateKeyPair generates both webhook and client keys
func (ks *KeyService) generateKeyPair() (webhookKey string, clientKey string, err error) {
	webhookKey, err = ks.generateKeyValue("wh_")
	if err != nil {
		return "", "", err
	}

	clientKey, err = ks.generateKeyValue("ck_")
	if err != nil {
		return "", "", err
	}

	return webhookKey, clientKey, nil
}

// mapBoolToStatus converts is_active boolean to status string
func (ks *KeyService) mapBoolToStatus(isActive bool) string {
	if isActive {
		return string(models.KeyStatusActive)
	}
	return string(models.KeyStatusInactive)
}

// CreateKeyPair creates a webhook key and client key together in a single transaction.
// The client key's pair_id references the webhook key's id, establishing the relationship.
func (ks *KeyService) CreateKeyPair(ctx context.Context) (*models.WebhookKey, *models.ClientKey, error) {
	// Generate keys
	webhookKeyValue, clientKeyValue, err := ks.generateKeyPair()
	if err != nil {
		return nil, nil, err
	}

	// Begin transaction
	tx, err := ks.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert webhook key
	webhookID := uuid.New()
	now := time.Now()
	var wk models.WebhookKey
	var isActive bool
	err = tx.QueryRow(ctx, `
		INSERT INTO api_keys (
			id, key_value, key_type, pair_id, is_active, activated_at,
			created_at, usage_count
		)
		VALUES ($1, $2, 'webhook', $1, true, $3, $3, 0)
		RETURNING id, key_value, is_active, created_at, last_used, usage_count
	`, webhookID, webhookKeyValue, now).Scan(&wk.ID, &wk.KeyValue, &isActive, &wk.CreatedAt, &wk.LastUsed, &wk.EventsCount)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create webhook key: %w", err)
	}
	wk.Status = ks.mapBoolToStatus(isActive)

	// Insert client key
	var ck models.ClientKey
	var clientIsActive bool
	err = tx.QueryRow(ctx, `
		INSERT INTO api_keys (
			id, key_value, key_type, pair_id, is_active, activated_at,
			created_at, usage_count
		)
		VALUES ($1, $2, 'client', $3, true, $4, $4, 0)
		RETURNING id, key_value, pair_id, is_active, created_at, last_used, usage_count
	`, uuid.New(), clientKeyValue, webhookID, now).Scan(&ck.ID, &ck.KeyValue, &ck.WebhookKeyID, &clientIsActive, &ck.CreatedAt, &ck.LastConnected, &ck.EventsDelivered)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client key: %w", err)
	}
	ck.Status = ks.mapBoolToStatus(clientIsActive)

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &wk, &ck, nil
}

// GetUsers returns all users with their key information, ordered by created_at DESC
func (ks *KeyService) GetUsers(ctx context.Context) ([]User, error) {
	query := `
		SELECT
			user_email,
			COALESCE(MAX(user_name), '') as user_name,
			BOOL_OR(is_active) as is_active,
			MIN(created_at) as created_at,
			MAX(last_used) as last_used,
			COALESCE(SUM(usage_count), 0) as usage_count,
			COUNT(*) FILTER (WHERE key_type = 'webhook') as webhook_key_count,
			COUNT(*) FILTER (WHERE key_type = 'client') as client_key_count,
			COALESCE(MAX(key_value) FILTER (WHERE key_type = 'webhook'), '') as webhook_key,
			COALESCE(MAX(key_value) FILTER (WHERE key_type = 'client'), '') as client_key
		FROM api_keys
		WHERE user_email IS NOT NULL AND user_email != ''
		GROUP BY user_email
		ORDER BY MIN(created_at) DESC
	`

	rows, err := ks.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.UserEmail,
			&user.UserName,
			&user.IsActive,
			&user.CreatedAt,
			&user.LastUsed,
			&user.UsageCount,
			&user.WebhookKeyCount,
			&user.ClientKeyCount,
			&user.WebhookKey,
			&user.ClientKey,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// GetUserDetails returns detailed information about a specific user by email
func (ks *KeyService) GetUserDetails(ctx context.Context, userEmail string) (*User, error) {
	user := &User{UserEmail: userEmail}

	// Get created_at (earliest key)
	err := ks.pool.QueryRow(ctx, `
		SELECT MIN(created_at) FROM api_keys
		WHERE user_email = $1
	`, userEmail).Scan(&user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user details: %w", err)
	}

	// Get the most recent active webhook key value
	var webhookKey string
	err = ks.pool.QueryRow(ctx, `
		SELECT key_value FROM api_keys
		WHERE user_email = $1 AND key_type = 'webhook' AND is_active = true
		ORDER BY created_at DESC
		LIMIT 1
	`, userEmail).Scan(&webhookKey)
	if err == nil {
		user.WebhookKey = webhookKey
	}

	// Get the most recent active client key value
	var clientKey string
	err = ks.pool.QueryRow(ctx, `
		SELECT key_value FROM api_keys
		WHERE user_email = $1 AND key_type = 'client' AND is_active = true
		ORDER BY created_at DESC
		LIMIT 1
	`, userEmail).Scan(&clientKey)
	if err == nil {
		user.ClientKey = clientKey
	}

	return user, nil
}

// GetUserKeyCount returns the count of active keys of a specific type for a user
func (ks *KeyService) GetUserKeyCount(ctx context.Context, userEmail string, keyType string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM api_keys
		WHERE user_email = $1 AND key_type = $2 AND is_active = true
	`

	var count int
	err := ks.pool.QueryRow(ctx, query, userEmail, keyType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get user key count: %w", err)
	}

	return count, nil
}

// GetUserEventCount returns the count of events for a user
func (ks *KeyService) GetUserEventCount(ctx context.Context, userEmail string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM events
		WHERE webhook_key_id IN (
			SELECT id FROM api_keys WHERE user_email = $1 AND key_type = 'webhook'
		)
	`

	var count int
	err := ks.pool.QueryRow(ctx, query, userEmail).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get user event count: %w", err)
	}

	return count, nil
}

// GetUserWebhookLogCount returns the count of webhook logs for a user
func (ks *KeyService) GetUserWebhookLogCount(ctx context.Context, userEmail string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM webhook_logs
		WHERE webhook_key_id IN (
			SELECT id FROM api_keys WHERE user_email = $1 AND key_type = 'webhook'
		)
	`

	var count int
	err := ks.pool.QueryRow(ctx, query, userEmail).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get user webhook log count: %w", err)
	}

	return count, nil
}

// GetUserWebhookLogs returns webhook delivery logs for a specific user with pagination
func (ks *KeyService) GetUserWebhookLogs(ctx context.Context, userEmail string, limit int, offset int) ([]WebhookLog, int, error) {
	// Get total count
	countQuery := `
		SELECT COUNT(*)
		FROM webhook_logs
		WHERE webhook_key_id IN (
			SELECT id FROM api_keys WHERE user_email = $1 AND key_type = 'webhook'
		)
	`

	var total int
	err := ks.pool.QueryRow(ctx, countQuery, userEmail).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get webhook log count: %w", err)
	}

	// Get paginated logs ordered by attempted_at DESC
	query := `
		SELECT
			wl.id,
			wl.event_id,
			wl.webhook_key_id,
			wl.client_key_id,
			wl.delivery_status,
			wl.status_code,
			wl.error_message,
			wl.attempted_at,
			wl.delivered_at,
			wl.acked_at,
			wl.client_ip,
			wl.created_at
		FROM webhook_logs wl
		WHERE wl.webhook_key_id IN (
			SELECT id FROM api_keys WHERE user_email = $1 AND key_type = 'webhook'
		)
		ORDER BY wl.attempted_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := ks.pool.Query(ctx, query, userEmail, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query webhook logs: %w", err)
	}
	defer rows.Close()

	var logs []WebhookLog
	for rows.Next() {
		wl := WebhookLog{}
		if err := rows.Scan(
			&wl.ID,
			&wl.EventID,
			&wl.WebhookKeyID,
			&wl.ClientKeyID,
			&wl.DeliveryStatus,
			&wl.StatusCode,
			&wl.ErrorMessage,
			&wl.AttemptedAt,
			&wl.DeliveredAt,
			&wl.AckedAt,
			&wl.ClientIP,
			&wl.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan webhook log: %w", err)
		}
		logs = append(logs, wl)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating webhook logs: %w", err)
	}

	return logs, total, nil
}

// WebhookLogEntry is a simplified webhook log for dashboard API (no path — privacy)
type WebhookLogEntry struct {
	ID             string     `json:"id"`
	EventID        string     `json:"event_id"`
	DeliveryStatus string     `json:"delivery_status"`
	StatusCode     *int       `json:"status_code,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	AttemptedAt    time.Time  `json:"attempted_at"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	AckedAt        *time.Time `json:"acked_at,omitempty"`
}

// CreateWebhookLog creates a new webhook log entry with pending status
func (ks *KeyService) CreateWebhookLog(ctx context.Context, eventID uuid.UUID, webhookKeyID uuid.UUID, statusCode int) error {
	_, err := ks.pool.Exec(ctx, `
		INSERT INTO webhook_logs (event_id, webhook_key_id, delivery_status, status_code, attempted_at)
		VALUES ($1, $2, 'pending', $3, NOW())
	`, eventID, webhookKeyID, statusCode)
	if err != nil {
		return fmt.Errorf("failed to create webhook log: %w", err)
	}
	return nil
}

// UpdateWebhookLogDelivered updates a webhook log to delivered status
func (ks *KeyService) UpdateWebhookLogDelivered(ctx context.Context, eventID uuid.UUID, webhookKeyID uuid.UUID, clientKeyID uuid.UUID) error {
	_, err := ks.pool.Exec(ctx, `
		UPDATE webhook_logs
		SET delivery_status = 'delivered', delivered_at = NOW(), client_key_id = $3
		WHERE event_id = $1 AND webhook_key_id = $2 AND delivery_status = 'pending'
	`, eventID, webhookKeyID, clientKeyID)
	if err != nil {
		return fmt.Errorf("failed to update webhook log to delivered: %w", err)
	}
	return nil
}

// UpdateWebhookLogAcked updates a webhook log to acked status
func (ks *KeyService) UpdateWebhookLogAcked(ctx context.Context, eventID uuid.UUID) error {
	_, err := ks.pool.Exec(ctx, `
		UPDATE webhook_logs
		SET delivery_status = 'acked', acked_at = NOW()
		WHERE event_id = $1 AND delivery_status IN ('pending', 'delivered')
	`, eventID)
	if err != nil {
		return fmt.Errorf("failed to update webhook log to acked: %w", err)
	}
	return nil
}

// UpdateKeyUsageStats updates last_used and increments usage_count for a webhook key
func (ks *KeyService) UpdateKeyUsageStats(ctx context.Context, webhookKeyID uuid.UUID) error {
	_, err := ks.pool.Exec(ctx, `
		UPDATE api_keys SET last_used = NOW(), usage_count = usage_count + 1 WHERE id = $1
	`, webhookKeyID)
	if err != nil {
		return fmt.Errorf("failed to update key usage stats: %w", err)
	}
	return nil
}

// GetUserWebhookLogEntries returns webhook log entries for dashboard API (no path — privacy)
func (ks *KeyService) GetUserWebhookLogEntries(ctx context.Context, userEmail string, limit int, offset int) ([]WebhookLogEntry, int, error) {
	var total int
	err := ks.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_logs
		WHERE webhook_key_id IN (
			SELECT id FROM api_keys WHERE user_email = $1 AND key_type = 'webhook'
		)
	`, userEmail).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count webhook logs: %w", err)
	}

	rows, err := ks.pool.Query(ctx, `
		SELECT id, event_id, delivery_status, status_code, error_message,
			   attempted_at, delivered_at, acked_at
		FROM webhook_logs
		WHERE webhook_key_id IN (
			SELECT id FROM api_keys WHERE user_email = $1 AND key_type = 'webhook'
		)
		ORDER BY attempted_at DESC
		LIMIT $2 OFFSET $3
	`, userEmail, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query webhook logs: %w", err)
	}
	defer rows.Close()

	var logs []WebhookLogEntry
	for rows.Next() {
		var l WebhookLogEntry
		if err := rows.Scan(
			&l.ID, &l.EventID, &l.DeliveryStatus, &l.StatusCode,
			&l.ErrorMessage, &l.AttemptedAt, &l.DeliveredAt, &l.AckedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan webhook log: %w", err)
		}
		logs = append(logs, l)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating webhook logs: %w", err)
	}

	return logs, total, nil
}

// KeyPair represents a webhook+client key pair for the dashboard
type KeyPair struct {
	PairID     string     `json:"pair_id"`
	WebhookKey string     `json:"webhook_key"`
	ClientKey  string     `json:"client_key"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsed   *time.Time `json:"last_used,omitempty"`
	UsageCount int        `json:"usage_count"`
}

// GetUserKeyPairs returns all key pairs for a user, ordered newest first
func (ks *KeyService) GetUserKeyPairs(ctx context.Context, userEmail string) ([]KeyPair, error) {
	rows, err := ks.pool.Query(ctx, `
		SELECT
			wk.id,
			wk.key_value,
			COALESCE(ck.key_value, ''),
			wk.is_active,
			wk.created_at,
			wk.last_used,
			wk.usage_count
		FROM api_keys wk
		LEFT JOIN api_keys ck ON ck.pair_id = wk.id AND ck.key_type = 'client'
		WHERE wk.user_email = $1 AND wk.key_type = 'webhook'
		ORDER BY wk.created_at DESC
	`, userEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to query key pairs: %w", err)
	}
	defer rows.Close()

	var pairs []KeyPair
	for rows.Next() {
		var p KeyPair
		if err := rows.Scan(&p.PairID, &p.WebhookKey, &p.ClientKey, &p.IsActive, &p.CreatedAt, &p.LastUsed, &p.UsageCount); err != nil {
			return nil, fmt.Errorf("failed to scan key pair: %w", err)
		}
		pairs = append(pairs, p)
	}
	return pairs, rows.Err()
}

// DeactivateKeyPairByID deactivates a specific key pair, scoped to user email
func (ks *KeyService) DeactivateKeyPairByID(ctx context.Context, pairID uuid.UUID, userEmail string) error {
	result, err := ks.pool.Exec(ctx, `
		UPDATE api_keys SET is_active = false
		WHERE pair_id = $1 AND user_email = $2 AND is_active = true
	`, pairID, userEmail)
	if err != nil {
		return fmt.Errorf("failed to deactivate key pair: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("key pair not found or already revoked")
	}
	return nil
}

// DeactivateKeyByID deactivates a key by its UUID ID
func (ks *KeyService) DeactivateKeyByID(ctx context.Context, keyID string) error {
	_, err := uuid.Parse(keyID)
	if err != nil {
		return fmt.Errorf("invalid key ID format: %w", err)
	}

	result, err := ks.pool.Exec(ctx,
		"UPDATE api_keys SET is_active = false WHERE id = $1",
		keyID,
	)

	if err != nil {
		return fmt.Errorf("failed to deactivate key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrKeyNotFound
	}

	return nil
}

// GetKeyByID retrieves a key by its ID
func (ks *KeyService) GetKeyByID(ctx context.Context, keyID string) (*KeyInfo, error) {
	// Parse keyID as UUID to validate format
	_, err := uuid.Parse(keyID)
	if err != nil {
		return nil, fmt.Errorf("invalid key ID format: %w", err)
	}

	var keyInfo KeyInfo
	err = ks.pool.QueryRow(ctx,
		"SELECT id, key_value, key_type, is_active FROM api_keys WHERE id = $1",
		keyID,
	).Scan(&keyInfo.ID, &keyInfo.KeyValue, &keyInfo.KeyType, &keyInfo.IsActive)

	if err != nil {
		return nil, fmt.Errorf("key not found: %w", err)
	}

	return &keyInfo, nil
}

// DeleteKeyPair deletes both webhook and client keys by pair_id
func (ks *KeyService) DeleteKeyPair(ctx context.Context, webhookKeyValue string) error {
	// First get the pair_id from webhook key
	var pairID *uuid.UUID
	err := ks.pool.QueryRow(ctx,
		"SELECT pair_id FROM api_keys WHERE key_value = $1 AND key_type = 'webhook'",
		webhookKeyValue,
	).Scan(&pairID)

	if err != nil {
		fmt.Printf("ERROR: webhook key not found: %s, error: %v\n", webhookKeyValue, err)
		return fmt.Errorf("webhook key not found: %w", err)
	}

	// If pair_id is NULL, delete by key_value directly
	if pairID == nil {
		fmt.Printf("DEBUG: pair_id is NULL for webhook key %s, deleting by key_value\n", webhookKeyValue)
		result, err := ks.pool.Exec(ctx,
			"DELETE FROM api_keys WHERE key_value = $1",
			webhookKeyValue,
		)
		if err != nil {
			fmt.Printf("ERROR: failed to delete key: %v\n", err)
			return fmt.Errorf("failed to delete key: %w", err)
		}
		rowsAffected := result.RowsAffected()
		fmt.Printf("DEBUG: Deleted %d rows with key_value %s\n", rowsAffected, webhookKeyValue)
		return nil
	}

	fmt.Printf("DEBUG: Found pair_id %s for webhook key %s\n", *pairID, webhookKeyValue)

	// Delete all keys with this pair_id (both webhook and client)
	result, err := ks.pool.Exec(ctx,
		"DELETE FROM api_keys WHERE pair_id = $1",
		*pairID,
	)

	if err != nil {
		fmt.Printf("ERROR: failed to delete key pair: %v\n", err)
		return fmt.Errorf("failed to delete key pair: %w", err)
	}

	rowsAffected := result.RowsAffected()
	fmt.Printf("DEBUG: Deleted %d rows with pair_id %s\n", rowsAffected, *pairID)

	return nil
}
