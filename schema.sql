-- ============================================================================
-- Obsidian Webhooks Selfhosted - Complete Database Schema
-- ============================================================================
-- This file contains the complete, final schema for production deployment.
-- To reset: DROP all tables in Supabase, then run this entire file.
-- ============================================================================

-- MIGRATION STEP: Drop old indexes and views on webhook_keys and client_keys if they exist
-- This prevents errors when converting tables to views
DROP INDEX IF EXISTS idx_webhook_keys_key_value;
DROP INDEX IF EXISTS idx_webhook_keys_status;
DROP INDEX IF EXISTS idx_webhook_keys_created_at;
DROP INDEX IF EXISTS idx_client_keys_key_value;
DROP INDEX IF EXISTS idx_client_keys_webhook_key_id;
DROP INDEX IF EXISTS idx_client_keys_status;
DROP VIEW IF EXISTS webhook_keys CASCADE;
DROP VIEW IF EXISTS client_keys CASCADE;

-- admin_users table - Admin authentication and system management
CREATE TABLE IF NOT EXISTS admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- MIGRATION STEP: Drop old api_keys table if it exists without the new columns
-- This ensures a clean slate for the new schema
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'api_keys') THEN
        -- Check if the table has the user_email column (new schema)
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public'
            AND table_name = 'api_keys'
            AND column_name = 'user_email'
        ) THEN
            -- Old schema, drop and recreate
            DROP TABLE api_keys CASCADE;
        END IF;
    END IF;
END
$$;

-- api_keys table - Unified keys for webhooks and SSE clients with user tracking
-- Consolidates webhook_keys and client_keys with user information
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_value VARCHAR(255) UNIQUE NOT NULL,
    key_type VARCHAR(20) NOT NULL, -- 'webhook' or 'client'
    pair_id UUID, -- links webhook key to its paired client key(s)
    is_active BOOLEAN NOT NULL DEFAULT true, -- for revocation only
    activated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- User information (email authentication)
    user_email VARCHAR(255),
    user_name VARCHAR(255),
    email_verified BOOLEAN NOT NULL DEFAULT false,
    magic_link_token VARCHAR(255),
    magic_link_expires_at TIMESTAMP,
    magic_link_used_at TIMESTAMP,

    -- Event retention settings
    event_ttl_days INTEGER DEFAULT 30, -- how many days to keep events

    -- Audit and usage tracking
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used TIMESTAMP,
    usage_count INTEGER NOT NULL DEFAULT 0,

    CONSTRAINT key_type_check CHECK (key_type IN ('webhook', 'client'))
);

-- MIGRATION STEP: Drop old tables if they exist
DO $$
BEGIN
    DROP TABLE IF EXISTS webhook_keys CASCADE;
    DROP TABLE IF EXISTS client_keys CASCADE;
    -- Drop legacy tables from payment/subscription system
    DROP TABLE IF EXISTS broadcast_responses CASCADE;
    DROP TABLE IF EXISTS broadcast_recipients CASCADE;
    DROP TABLE IF EXISTS broadcast_messages CASCADE;
    DROP TABLE IF EXISTS payments CASCADE;
END
$$;

-- Keep webhook_keys as a view for backward compatibility
CREATE OR REPLACE VIEW webhook_keys AS
SELECT
    id,
    key_value,
    CASE WHEN is_active THEN 'active' ELSE 'inactive' END as status,
    created_at,
    last_used,
    usage_count as events_count
FROM api_keys
WHERE key_type = 'webhook';

-- Keep client_keys as a view for backward compatibility
CREATE OR REPLACE VIEW client_keys AS
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

-- events table - Webhook events storage
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    path VARCHAR(512) NOT NULL,
    data BYTEA NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT false,
    processed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    CONSTRAINT processed_implies_timestamp CHECK (
        (processed = false AND processed_at IS NULL) OR
        (processed = true AND processed_at IS NOT NULL)
    )
);

-- webhook_logs table - Comprehensive webhook delivery and processing logs
CREATE TABLE IF NOT EXISTS webhook_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    webhook_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    client_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,

    -- Delivery attempt details
    delivery_status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, delivered, failed, acked
    status_code INTEGER, -- HTTP status code if applicable
    error_message TEXT, -- Error details if delivery failed

    -- Timing and location
    attempted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMP, -- When successfully delivered
    acked_at TIMESTAMP, -- When client ACK'd receipt
    client_ip VARCHAR(45), -- IP address of client

    -- Audit trail
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT delivery_status_check CHECK (delivery_status IN ('pending', 'delivered', 'failed', 'acked'))
);

-- Create indexes for optimization on api_keys table
CREATE INDEX IF NOT EXISTS idx_api_keys_key_value ON api_keys(key_value);
CREATE INDEX IF NOT EXISTS idx_api_keys_type ON api_keys(key_type);
CREATE INDEX IF NOT EXISTS idx_api_keys_is_active ON api_keys(is_active);
CREATE INDEX IF NOT EXISTS idx_api_keys_pair_id ON api_keys(pair_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_email ON api_keys(user_email);
CREATE INDEX IF NOT EXISTS idx_api_keys_magic_link_token ON api_keys(magic_link_token);
CREATE INDEX IF NOT EXISTS idx_api_keys_email_verified ON api_keys(email_verified);
CREATE INDEX IF NOT EXISTS idx_api_keys_type_active ON api_keys(key_type, is_active);

-- Indexes for events table
CREATE INDEX IF NOT EXISTS idx_events_webhook_key_id ON events(webhook_key_id);
CREATE INDEX IF NOT EXISTS idx_events_processed ON events(processed);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_expires_at ON events(expires_at);

-- Indexes for webhook_logs table (comprehensive webhook tracking)
CREATE INDEX IF NOT EXISTS idx_webhook_logs_event_id ON webhook_logs(event_id);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_webhook_key_id ON webhook_logs(webhook_key_id);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_client_key_id ON webhook_logs(client_key_id);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_delivery_status ON webhook_logs(delivery_status);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_attempted_at ON webhook_logs(attempted_at);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_delivered_at ON webhook_logs(delivered_at);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_acked_at ON webhook_logs(acked_at);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_webhook_status ON webhook_logs(webhook_key_id, delivery_status);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_events_webhook_key_processed ON events(webhook_key_id, processed);
CREATE INDEX IF NOT EXISTS idx_events_created_expires ON events(created_at, expires_at);
