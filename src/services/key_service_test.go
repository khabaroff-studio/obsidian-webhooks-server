package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Test constants
const (
	errNoRows        = "no rows in result set"
	testStatusActive = "active"
)

// TestValidateWebhookKey_ActiveKey tests validation of active webhook key
func TestValidateWebhookKey_ActiveKey(t *testing.T) {
	// Setup
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	ks := NewKeyService(pool)

	// Create a test webhook key
	keyID := uuid.New()
	err := pool.QueryRow(ctx,
		`INSERT INTO webhook_keys (id, key_value, status)
		 VALUES ($1, $2, $3)`,
		keyID, "test_wh_key_123", "active",
	).Scan()
	if err == nil || err.Error() != errNoRows {
		t.Fatalf("Failed to insert test key: %v", err)
	}

	// Test: ValidateWebhookKey should return true for active key
	isValid, err := ks.ValidateWebhookKey(ctx, "test_wh_key_123")
	if err != nil {
		t.Fatalf("ValidateWebhookKey failed: %v", err)
	}

	if !isValid {
		t.Errorf("Expected isValid to be true, got false")
	}
}

// TestValidateWebhookKey_InactiveKey tests that inactive keys are rejected
func TestValidateWebhookKey_InactiveKey(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	ks := NewKeyService(pool)

	// Create inactive webhook key
	keyID := uuid.New()
	_ = pool.QueryRow(ctx,
		`INSERT INTO webhook_keys (id, key_value, status)
		 VALUES ($1, $2, $3)`,
		keyID, "test_wh_key_inactive", "inactive",
	).Scan()

	// Test: ValidateWebhookKey should return false for inactive key
	isValid, err := ks.ValidateWebhookKey(ctx, "test_wh_key_inactive")
	if err != nil {
		t.Fatalf("ValidateWebhookKey failed: %v", err)
	}

	if isValid {
		t.Errorf("Expected isValid to be false for inactive key, got true")
	}
}

// TestValidateWebhookKey_NonexistentKey tests handling of non-existent keys
func TestValidateWebhookKey_NonexistentKey(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	ks := NewKeyService(pool)

	// Test: ValidateWebhookKey should return false for non-existent key
	isValid, err := ks.ValidateWebhookKey(ctx, "nonexistent_key")
	if err != nil {
		t.Fatalf("ValidateWebhookKey failed: %v", err)
	}

	if isValid {
		t.Errorf("Expected isValid to be false for nonexistent key, got true")
	}
}

// TestCreateKeyPair tests key pair creation with api_keys table
func TestCreateKeyPair(t *testing.T) {
	pool := setupTestDBWithAPIKeys(t)
	defer pool.Close()

	ctx := context.Background()
	ks := NewKeyService(pool)

	// Test: CreateKeyPair should create both webhook and client keys
	wk, ck, err := ks.CreateKeyPair(ctx)
	if err != nil {
		t.Fatalf("CreateKeyPair failed: %v", err)
	}

	// Verify webhook key
	if wk == nil {
		t.Fatal("Expected webhook key, got nil")
	}
	if wk.ID == uuid.Nil {
		t.Errorf("Webhook key ID is not valid")
	}
	if wk.KeyValue == "" || !isValidKeyValue(wk.KeyValue, "wh_") {
		t.Errorf("Invalid webhook key value: %s", wk.KeyValue)
	}
	if wk.Status != testStatusActive {
		t.Errorf("Expected webhook key status '%s', got %s", testStatusActive, wk.Status)
	}

	// Verify client key
	if ck == nil {
		t.Fatal("Expected client key, got nil")
	}
	if ck.ID == uuid.Nil {
		t.Errorf("Client key ID is not valid")
	}
	if ck.KeyValue == "" || !isValidKeyValue(ck.KeyValue, "ck_") {
		t.Errorf("Invalid client key value: %s", ck.KeyValue)
	}
	if ck.Status != testStatusActive {
		t.Errorf("Expected client key status '%s', got %s", testStatusActive, ck.Status)
	}
	if ck.WebhookKeyID != wk.ID {
		t.Errorf("Client key webhook_key_id does not match webhook key ID")
	}

	// Verify keys can be retrieved
	retrievedWK, err := ks.GetWebhookKeyByValue(ctx, wk.KeyValue)
	if err != nil {
		t.Fatalf("Failed to retrieve webhook key: %v", err)
	}
	if retrievedWK.ID != wk.ID {
		t.Errorf("Retrieved webhook key ID mismatch")
	}

	retrievedCK, err := ks.GetClientKeyByValue(ctx, ck.KeyValue)
	if err != nil {
		t.Fatalf("Failed to retrieve client key: %v", err)
	}
	if retrievedCK.ID != ck.ID {
		t.Errorf("Retrieved client key ID mismatch")
	}
}

// TestCreateKeyPair_BackwardCompatibilityViews verifies that keys created in api_keys
// are accessible via the backward compatibility views webhook_keys and client_keys
func TestCreateKeyPair_BackwardCompatibilityViews(t *testing.T) {
	pool := setupTestDBWithAPIKeys(t)
	defer pool.Close()

	ctx := context.Background()
	ks := NewKeyService(pool)

	// Create key pair (inserts into api_keys table)
	wk, ck, err := ks.CreateKeyPair(ctx)
	if err != nil {
		t.Fatalf("CreateKeyPair failed: %v", err)
	}

	// Test webhook key is accessible via webhook_keys view
	isValid, err := ks.ValidateWebhookKey(ctx, wk.KeyValue)
	if err != nil {
		t.Fatalf("ValidateWebhookKey failed: %v", err)
	}
	if !isValid {
		t.Errorf("Expected webhook key to be valid via webhook_keys view")
	}

	// Test client key is accessible via client_keys view
	isValid, err = ks.ValidateClientKey(ctx, ck.KeyValue)
	if err != nil {
		t.Fatalf("ValidateClientKey failed: %v", err)
	}
	if !isValid {
		t.Errorf("Expected client key to be valid via client_keys view")
	}

	// Test field mapping: is_active (true) → status ('active')
	var status string
	err = pool.QueryRow(ctx, "SELECT status FROM webhook_keys WHERE key_value = $1", wk.KeyValue).Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query webhook_keys view: %v", err)
	}
	if status != testStatusActive {
		t.Errorf("Expected status '%s', got '%s'", testStatusActive, status)
	}

	// Test pair_id mapping: pair_id → webhook_key_id
	var webhookKeyID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT webhook_key_id FROM client_keys WHERE key_value = $1", ck.KeyValue).Scan(&webhookKeyID)
	if err != nil {
		t.Fatalf("Failed to query client_keys view: %v", err)
	}
	if webhookKeyID != wk.ID {
		t.Errorf("Expected webhook_key_id to match webhook key ID")
	}
}

// isValidKeyValue checks if a key value has the correct prefix and format
func isValidKeyValue(keyValue string, prefix string) bool {
	return len(keyValue) > len(prefix) && keyValue[:len(prefix)] == prefix
}

// setupTestDB creates a test database connection with old schema (for backward compatibility tests)
func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()

	// Use test database URL (you may need to set TEST_DATABASE_URL env var)
	dbURL := "postgres://test:test@localhost/obsidian_webhooks_test"

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Skipf("Could not connect to test database: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Skipf("Could not create connection pool: %v", err)
	}

	// Initialize schema
	schema := `
		DROP TABLE IF EXISTS event_acks CASCADE;
		DROP TABLE IF EXISTS events CASCADE;
		DROP TABLE IF EXISTS client_keys CASCADE;
		DROP TABLE IF EXISTS webhook_keys CASCADE;

		CREATE TABLE webhook_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			key_value VARCHAR(255) UNIQUE NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			last_used TIMESTAMP,
			events_count INTEGER NOT NULL DEFAULT 0,
			CONSTRAINT status_check CHECK (status IN ('active', 'inactive'))
		);

		CREATE TABLE client_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			key_value VARCHAR(255) UNIQUE NOT NULL,
			webhook_key_id UUID NOT NULL REFERENCES webhook_keys(id) ON DELETE CASCADE,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			last_connected TIMESTAMP,
			events_delivered INTEGER NOT NULL DEFAULT 0,
			CONSTRAINT status_check CHECK (status IN ('active', 'inactive'))
		);

		CREATE TABLE events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			webhook_key_id UUID NOT NULL REFERENCES webhook_keys(id) ON DELETE CASCADE,
			path VARCHAR(512) NOT NULL,
			data BYTEA NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT false,
			processed_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL
		);

		CREATE TABLE event_acks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
			client_key_id UUID NOT NULL REFERENCES client_keys(id) ON DELETE CASCADE,
			acked_at TIMESTAMP NOT NULL DEFAULT NOW(),
			client_ip VARCHAR(45)
		);
	`

	_, err = pool.Exec(ctx, schema)
	if err != nil {
		t.Skipf("Could not initialize test schema: %v", err)
	}

	return pool
}

// setupTestDBWithAPIKeys creates a test database connection with new api_keys schema
func setupTestDBWithAPIKeys(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()

	dbURL := "postgres://test:test@localhost/obsidian_webhooks_test"

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Skipf("Could not connect to test database: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Skipf("Could not create connection pool: %v", err)
	}

	// Initialize new schema with api_keys table
	schema := `
		DROP TABLE IF EXISTS event_acks CASCADE;
		DROP TABLE IF EXISTS events CASCADE;
		DROP VIEW IF EXISTS client_keys CASCADE;
		DROP VIEW IF EXISTS webhook_keys CASCADE;
		DROP TABLE IF EXISTS api_keys CASCADE;

		CREATE TABLE api_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			key_value VARCHAR(255) UNIQUE NOT NULL,
			key_type VARCHAR(20) NOT NULL,
			pair_id UUID,
			is_active BOOLEAN NOT NULL DEFAULT true,
			activated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			user_email VARCHAR(255),
			user_name VARCHAR(255),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			last_used TIMESTAMP,
			usage_count INTEGER NOT NULL DEFAULT 0,
			CONSTRAINT key_type_check CHECK (key_type IN ('webhook', 'client'))
		);

		CREATE VIEW webhook_keys AS
		SELECT
			id,
			key_value,
			CASE WHEN is_active THEN 'active' ELSE 'inactive' END as status,
			created_at,
			last_used,
			usage_count as events_count
		FROM api_keys
		WHERE key_type = 'webhook';

		CREATE VIEW client_keys AS
		SELECT
			id,
			key_value,
			pair_id as webhook_key_id,
			CASE WHEN is_active THEN 'active' ELSE 'inactive' END as status,
			created_at,
			last_used as last_connected,
			usage_count as events_delivered
		FROM api_keys
		WHERE key_type = 'client';

		CREATE TABLE events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			webhook_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
			path VARCHAR(512) NOT NULL,
			data BYTEA NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT false,
			processed_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL
		);

		CREATE TABLE event_acks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
			client_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
			acked_at TIMESTAMP NOT NULL DEFAULT NOW(),
			client_ip VARCHAR(45)
		);
	`

	_, err = pool.Exec(ctx, schema)
	if err != nil {
		t.Skipf("Could not initialize test schema: %v", err)
	}

	return pool
}

// TestDeactivateKeyByID tests deactivating a key by UUID
func TestDeactivateKeyByID(t *testing.T) {
	pool := setupTestDBWithAPIKeys(t)
	defer pool.Close()

	ctx := context.Background()
	ks := NewKeyService(pool)

	// Create a test key
	keyID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO api_keys (id, key_value, key_type, is_active, created_at)
		 VALUES ($1, $2, $3, $4, NOW())`,
		keyID, "test_deactivate_key", "webhook", true,
	)
	if err != nil {
		t.Fatalf("Failed to insert test key: %v", err)
	}

	// Test: DeactivateKeyByID should deactivate the key
	err = ks.DeactivateKeyByID(ctx, keyID.String())
	if err != nil {
		t.Fatalf("DeactivateKeyByID failed: %v", err)
	}

	// Verify key is deactivated
	keyInfo, err := ks.GetKeyByID(ctx, keyID.String())
	if err != nil {
		t.Fatalf("GetKeyByID failed: %v", err)
	}

	if keyInfo.IsActive {
		t.Errorf("Expected key to be inactive, but IsActive is true")
	}
}

// TestDeactivateKeyByID_NonexistentKey tests handling of non-existent key IDs
func TestDeactivateKeyByID_NonexistentKey(t *testing.T) {
	pool := setupTestDBWithAPIKeys(t)
	defer pool.Close()

	ctx := context.Background()
	ks := NewKeyService(pool)

	// Test: DeactivateKeyByID should fail for non-existent key
	err := ks.DeactivateKeyByID(ctx, uuid.New().String())
	if err == nil {
		t.Fatal("Expected DeactivateKeyByID to fail for non-existent key, but it succeeded")
	}
	if err.Error() != "key not found" {
		t.Errorf("Expected 'key not found' error, got: %v", err)
	}
}

// TestDeactivateKeyByID_InvalidUUID tests handling of invalid UUID format
func TestDeactivateKeyByID_InvalidUUID(t *testing.T) {
	pool := setupTestDBWithAPIKeys(t)
	defer pool.Close()

	ctx := context.Background()
	ks := NewKeyService(pool)

	// Test: DeactivateKeyByID should fail for invalid UUID
	err := ks.DeactivateKeyByID(ctx, "not-a-uuid")
	if err == nil {
		t.Fatal("Expected DeactivateKeyByID to fail for invalid UUID, but it succeeded")
	}
	if err.Error() != "invalid key ID format: invalid UUID length: 10" {
		t.Errorf("Expected invalid UUID error, got: %v", err)
	}
}
