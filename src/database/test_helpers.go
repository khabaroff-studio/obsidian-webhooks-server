package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	schemaInitOnce sync.Once
	schemaInitErr  error
	cleanupMutex   sync.Mutex // Serializes cleanup to prevent concurrent TRUNCATE conflicts
)

// TestDB wraps a connection pool configured for testing
type TestDB struct {
	Pool *pgxpool.Pool
	t    *testing.T
}

// DefaultTestDatabaseURL is the default connection string for local testing
// Uses port 5433 to avoid conflict with any local PostgreSQL on 5432
const DefaultTestDatabaseURL = "postgres://test:test@localhost:5433/obsidian_webhooks_test?sslmode=disable"

// GetTestDatabaseURL returns the test database URL from environment or default
func GetTestDatabaseURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return DefaultTestDatabaseURL
}

// NewTestDB creates a connection to the test database
// It will skip the test if the database is not available
func NewTestDB(t *testing.T) *TestDB {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbURL := GetTestDatabaseURL()
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Skipf("Could not parse test database URL: %v", err)
		return nil
	}

	// Smaller pool for tests
	config.MaxConns = 5
	config.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Skipf("Could not connect to test database: %v (hint: run 'docker-compose -f docker-compose.test.yml up -d')", err)
		return nil
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Could not ping test database: %v", err)
		return nil
	}

	tdb := &TestDB{Pool: pool, t: t}

	// Clean up when test completes
	t.Cleanup(func() {
		tdb.Cleanup()
		tdb.Close()
	})

	return tdb
}

// SetupSchema initializes the test schema
// This reads and executes scripts/test_schema.sql
func (tdb *TestDB) SetupSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaSQL, err := readTestSchemaSQL()
	if err != nil {
		return fmt.Errorf("could not read test schema: %w", err)
	}

	_, err = tdb.Pool.Exec(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("could not execute test schema: %w", err)
	}

	return nil
}

// Cleanup truncates all tables (thread-safe for parallel tests)
func (tdb *TestDB) Cleanup() {
	// Serialize cleanup to prevent concurrent TRUNCATE conflicts
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to use the helper function first
	_, err := tdb.Pool.Exec(ctx, "SELECT truncate_all_tables()")
	if err != nil {
		// Fallback to manual truncate (ignore error, best effort cleanup)
		_, _ = tdb.Pool.Exec(ctx, `
			TRUNCATE webhook_logs CASCADE;
			TRUNCATE events CASCADE;
			TRUNCATE api_keys CASCADE;
			TRUNCATE admin_users CASCADE;
		`)
	}
}

// Close closes the connection pool
func (tdb *TestDB) Close() {
	if tdb.Pool != nil {
		tdb.Pool.Close()
	}
}

// CreateTestKeyPair creates a test webhook+client key pair
// Returns webhookKeyID, clientKeyID, webhookKeyValue, clientKeyValue
// Parameters are kept for backward compatibility but ignored by the stored function
func (tdb *TestDB) CreateTestKeyPair(legacyID int64, legacyName string) (webhookKeyID, clientKeyID, webhookKeyValue, clientKeyValue string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := tdb.Pool.QueryRow(ctx,
		"SELECT webhook_key_id, client_key_id, webhook_key_value, client_key_value FROM create_test_key_pair($1, $2)",
		legacyID, legacyName,
	)

	err = row.Scan(&webhookKeyID, &clientKeyID, &webhookKeyValue, &clientKeyValue)
	return
}

// CreateTestAdmin creates a test admin user
// Returns adminID, username
func (tdb *TestDB) CreateTestAdmin(username, passwordHash string) (adminID, returnedUsername string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := tdb.Pool.QueryRow(ctx,
		"SELECT admin_id, username FROM create_test_admin($1, $2)",
		username, passwordHash,
	)

	err = row.Scan(&adminID, &returnedUsername)
	return
}

// CreateTestEvent creates a test event for a webhook key
// Returns eventID
func (tdb *TestDB) CreateTestEvent(webhookKeyID, path string, data []byte) (eventID string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := tdb.Pool.QueryRow(ctx,
		"SELECT create_test_event($1, $2, $3)",
		webhookKeyID, path, data,
	)

	err = row.Scan(&eventID)
	return
}

// readTestSchemaSQL reads the test schema SQL file
func readTestSchemaSQL() (string, error) {
	// Try multiple locations for the schema file
	locations := []string{
		"scripts/test_schema.sql",
		"../../scripts/test_schema.sql",
		"../../../scripts/test_schema.sql",
	}

	// Also try relative to this file
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
		locations = append(locations, filepath.Join(projectRoot, "scripts", "test_schema.sql"))
	}

	for _, loc := range locations {
		content, err := os.ReadFile(loc) // #nosec G304 -- test helper, paths are hardcoded
		if err == nil {
			return string(content), nil
		}
	}

	return "", fmt.Errorf("could not find scripts/test_schema.sql in any known location")
}

// WithTestDB is a helper for tests that need database access
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    database.WithTestDB(t, func(tdb *database.TestDB) {
//	        // Use tdb.Pool for database operations
//	    })
//	}
func WithTestDB(t *testing.T, fn func(tdb *TestDB)) {
	t.Helper()

	tdb := NewTestDB(t)
	if tdb == nil {
		return // Test was skipped
	}

	// Setup schema once (thread-safe for parallel tests)
	schemaInitOnce.Do(func() {
		schemaInitErr = tdb.SetupSchema()
	})

	if schemaInitErr != nil {
		t.Skipf("Could not initialize test schema: %v", schemaInitErr)
		return
	}

	fn(tdb)
}

// MustNewTestDB creates a test database connection and fails the test if it can't connect
// Use this when the test absolutely requires a database
func MustNewTestDB(t *testing.T) *TestDB {
	t.Helper()

	tdb := NewTestDB(t)
	if tdb == nil {
		t.Fatal("Test requires database but could not connect")
	}

	if err := tdb.SetupSchema(); err != nil {
		t.Fatalf("Could not initialize test schema: %v", err)
	}

	return tdb
}

// NewDatabaseFromPool creates a Database instance from an existing pool
// This is useful for testing handlers that depend on database.Database
func NewDatabaseFromPool(pool *pgxpool.Pool) *Database {
	return &Database{pool: pool}
}
