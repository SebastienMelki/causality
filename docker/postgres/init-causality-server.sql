-- Create causality_server database for auth module (API key management)
CREATE DATABASE causality_server;
GRANT ALL PRIVILEGES ON DATABASE causality_server TO hive;

-- Connect to causality_server database
\connect causality_server

-- API keys table for authentication
CREATE TABLE IF NOT EXISTS api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id     TEXT NOT NULL,
    key_hash   TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL DEFAULT '',
    revoked    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

-- Partial index for fast lookup of active keys by hash
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash) WHERE NOT revoked;

-- Index for listing keys by app
CREATE INDEX idx_api_keys_app_id ON api_keys(app_id);

-- Seed a development API key for local testing.
-- Plaintext key: deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef
-- SHA256 hash:   247d08f3e13938b244f5ecd8966f1778e5e72b175820f46ba86c9c039272affa
-- DO NOT use this key in production.
INSERT INTO api_keys (app_id, key_hash, name) VALUES
    ('dev-app', '247d08f3e13938b244f5ecd8966f1778e5e72b175820f46ba86c9c039272affa', 'Development key (all examples)')
ON CONFLICT (key_hash) DO NOTHING;

-- Grant permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO hive;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO hive;
