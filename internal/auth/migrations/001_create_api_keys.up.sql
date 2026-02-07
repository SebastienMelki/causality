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
