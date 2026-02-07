-- ============================================================================
-- Obsidian Webhooks - Test Database Schema
-- ============================================================================
-- This schema mirrors production but is designed for testing.
-- Run: psql -f scripts/test_schema.sql
-- Or use docker-compose.test.yml which auto-initializes this.
-- ============================================================================

-- Drop existing tables for clean state (safe for parallel execution)
-- Note: CASCADE automatically drops dependent views
DROP TABLE IF EXISTS webhook_logs CASCADE;
DROP TABLE IF EXISTS events CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS admin_users CASCADE;

-- admin_users table
CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- api_keys table (unified webhook + client keys)
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_value VARCHAR(255) UNIQUE NOT NULL,
    key_type VARCHAR(20) NOT NULL,
    pair_id UUID,
    is_active BOOLEAN NOT NULL DEFAULT true,
    activated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- User information (email authentication)
    user_email VARCHAR(255),
    user_name VARCHAR(255),
    email_verified BOOLEAN NOT NULL DEFAULT false,
    magic_link_token VARCHAR(255),
    magic_link_expires_at TIMESTAMP,
    magic_link_used_at TIMESTAMP,

    -- Event retention settings
    event_ttl_days INTEGER NOT NULL DEFAULT 30,

    -- Audit and usage tracking
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used TIMESTAMP,
    usage_count INTEGER NOT NULL DEFAULT 0,

    CONSTRAINT key_type_check CHECK (key_type IN ('webhook', 'client'))
);

-- Backward compatibility views
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

-- events table
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

-- webhook_logs table
CREATE TABLE webhook_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    webhook_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    client_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    delivery_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    status_code INTEGER,
    error_message TEXT,
    attempted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMP,
    acked_at TIMESTAMP,
    client_ip VARCHAR(45),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT delivery_status_check CHECK (delivery_status IN ('pending', 'delivered', 'failed', 'acked'))
);

-- Indexes for performance
CREATE INDEX idx_api_keys_key_value ON api_keys(key_value);
CREATE INDEX idx_api_keys_type ON api_keys(key_type);
CREATE INDEX idx_api_keys_pair_id ON api_keys(pair_id);
CREATE INDEX idx_api_keys_user_email ON api_keys(user_email);
CREATE INDEX idx_api_keys_magic_link_token ON api_keys(magic_link_token);
CREATE INDEX idx_api_keys_email_verified ON api_keys(email_verified);
CREATE INDEX idx_events_webhook_key_id ON events(webhook_key_id);
CREATE INDEX idx_events_processed ON events(processed);
CREATE INDEX idx_events_expires_at ON events(expires_at);
CREATE INDEX idx_webhook_logs_event_id ON webhook_logs(event_id);
CREATE INDEX idx_webhook_logs_webhook_key_id ON webhook_logs(webhook_key_id);

-- ============================================================================
-- Test Helper Functions
-- ============================================================================

-- Truncate all tables (for test cleanup)
CREATE OR REPLACE FUNCTION truncate_all_tables() RETURNS void AS $$
BEGIN
    TRUNCATE webhook_logs CASCADE;
    TRUNCATE events CASCADE;
    TRUNCATE api_keys CASCADE;
    TRUNCATE admin_users CASCADE;
END;
$$ LANGUAGE plpgsql;

-- Create a test key pair (returns webhook_key_id, client_key_id, webhook_key_value, client_key_value)
-- Note: pair_id for webhook key equals its own id (matching production behavior)
-- Note: pair_id for client key equals the webhook key's id (for proper linking)
-- Parameters kept for backward compatibility but ignored (legacy from Telegram auth)
CREATE OR REPLACE FUNCTION create_test_key_pair(
    p_legacy_id BIGINT DEFAULT NULL,
    p_legacy_name VARCHAR DEFAULT NULL
)
RETURNS TABLE(
    webhook_key_id UUID,
    client_key_id UUID,
    webhook_key_value VARCHAR,
    client_key_value VARCHAR,
    pair_id UUID
) AS $$
DECLARE
    v_webhook_key_id UUID := gen_random_uuid();
    v_client_key_id UUID;
    v_webhook_key_value VARCHAR := 'wh_test_' || substr(md5(random()::text), 1, 24);
    v_client_key_value VARCHAR := 'ck_test_' || substr(md5(random()::text), 1, 24);
BEGIN
    -- Create webhook key with pair_id = id (matching production behavior)
    INSERT INTO api_keys (id, key_value, key_type, pair_id)
    VALUES (v_webhook_key_id, v_webhook_key_value, 'webhook', v_webhook_key_id);

    -- Create client key with pair_id = webhook_key_id (for proper linking)
    INSERT INTO api_keys (key_value, key_type, pair_id)
    VALUES (v_client_key_value, 'client', v_webhook_key_id)
    RETURNING id INTO v_client_key_id;

    RETURN QUERY SELECT v_webhook_key_id, v_client_key_id, v_webhook_key_value, v_client_key_value, v_webhook_key_id;
END;
$$ LANGUAGE plpgsql;

-- Create a test admin user (returns id, username)
CREATE OR REPLACE FUNCTION create_test_admin(
    p_username VARCHAR DEFAULT 'testadmin',
    p_password_hash VARCHAR DEFAULT '$2a$10$test.hash.for.testing.only'
) RETURNS TABLE(admin_id UUID, username VARCHAR) AS $$
DECLARE
    v_admin_id UUID;
BEGIN
    INSERT INTO admin_users (username, password_hash)
    VALUES (p_username, p_password_hash)
    RETURNING id INTO v_admin_id;

    RETURN QUERY SELECT v_admin_id, p_username;
END;
$$ LANGUAGE plpgsql;

-- Create a test event
CREATE OR REPLACE FUNCTION create_test_event(
    p_webhook_key_id UUID,
    p_path VARCHAR DEFAULT '/test/path',
    p_data BYTEA DEFAULT '{"test": true}'::bytea,
    p_expires_hours INTEGER DEFAULT 24
) RETURNS UUID AS $$
DECLARE
    v_event_id UUID;
BEGIN
    INSERT INTO events (webhook_key_id, path, data, expires_at)
    VALUES (p_webhook_key_id, p_path, p_data, NOW() + (p_expires_hours || ' hours')::interval)
    RETURNING id INTO v_event_id;

    RETURN v_event_id;
END;
$$ LANGUAGE plpgsql;
