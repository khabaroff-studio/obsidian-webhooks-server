package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Database holds the PostgreSQL connection pool
type Database struct {
	pool *pgxpool.Pool
}

// New creates a new database connection
func New(ctx context.Context, databaseURL string) (*Database, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	db := &Database{pool: pool}

	// Initialize schema
	if err := db.initializeSchema(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection pool
func (db *Database) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// GetPool returns the connection pool
func (db *Database) GetPool() *pgxpool.Pool {
	return db.pool
}

// initializeSchema reads and executes schema.sql
func (db *Database) initializeSchema(ctx context.Context) error {
	// Try to read schema.sql from multiple locations
	schemaPath := "schema.sql"

	content, err := os.ReadFile(schemaPath)
	if err != nil {
		// Try from current working directory
		content, err = os.ReadFile(filepath.Join(".", schemaPath))
		if err != nil {
			// Try from root directory
			content, err = os.ReadFile(filepath.Join("/", schemaPath))
			if err != nil {
				return fmt.Errorf("failed to read schema.sql: %w", err)
			}
		}
	}

	// Execute schema
	_, err = db.pool.Exec(ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	// Run migrations
	if err := db.runMigrations(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database schema initialized successfully")
	return nil
}

// runMigrations runs database migrations
func (db *Database) runMigrations(ctx context.Context) error {
	// Migration 1: Add event_ttl_days column if it doesn't exist
	_, err := db.pool.Exec(ctx, `
		ALTER TABLE api_keys
		ADD COLUMN IF NOT EXISTS event_ttl_days INTEGER DEFAULT 30;
	`)
	if err != nil {
		return fmt.Errorf("failed to add event_ttl_days column: %w", err)
	}

	// Migration 2: Update ALL records to ensure key_type and is_active are set correctly
	result, err := db.pool.Exec(ctx, `
		UPDATE api_keys
		SET
			key_type = CASE
				WHEN key_value LIKE 'wh_%' THEN 'webhook'
				WHEN key_value LIKE 'ck_%' THEN 'client'
				ELSE 'webhook'
			END,
			is_active = COALESCE(is_active, true)
	`)
	if err != nil {
		log.Printf("Migration WARNING: failed to update api_keys: %v", err)
	} else if result.RowsAffected() > 0 {
		log.Printf("Migration: Updated %d api_keys records", result.RowsAffected())
	}

	log.Println("Migrations completed successfully")
	return nil
}

// Health checks if the database is healthy
func (db *Database) Health(ctx context.Context) error {
	if db == nil || db.pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return db.pool.Ping(ctx)
}

// QueryRow executes a query and returns a single row
func (db *Database) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

// Query executes a query and returns rows
func (db *Database) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

// Exec executes a query without returning rows
func (db *Database) Exec(ctx context.Context, sql string, args ...interface{}) error {
	_, err := db.pool.Exec(ctx, sql, args...)
	return err
}
