-- Create reaction_engine database
CREATE DATABASE reaction_engine;
GRANT ALL PRIVILEGES ON DATABASE reaction_engine TO hive;

-- Connect to reaction_engine database
\connect reaction_engine

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Webhooks table: stores webhook endpoint configurations
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    auth_type VARCHAR(50) NOT NULL DEFAULT 'none', -- none, basic, bearer, hmac
    auth_config JSONB DEFAULT '{}', -- {"username":"x","password":"y"} or {"token":"x"} or {"secret":"x","header":"X-Signature"}
    headers JSONB DEFAULT '{}', -- Additional headers to send
    enabled BOOLEAN NOT NULL DEFAULT true,
    timeout_ms INTEGER NOT NULL DEFAULT 30000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhooks_enabled ON webhooks(enabled);

-- Rules table: stores rule definitions for event matching
CREATE TABLE rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    app_id VARCHAR(255), -- NULL means all apps
    event_category VARCHAR(100), -- NULL means all categories
    event_type VARCHAR(100), -- NULL means all types
    conditions JSONB NOT NULL DEFAULT '[]', -- [{"path":"$.field","operator":"eq","value":"x"}]
    actions JSONB NOT NULL DEFAULT '{}', -- {"webhooks":["uuid"],"publish_subjects":["alerts.{app_id}.x"]}
    priority INTEGER NOT NULL DEFAULT 0, -- Higher priority rules evaluated first
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rules_enabled ON rules(enabled);
CREATE INDEX idx_rules_app_id ON rules(app_id);
CREATE INDEX idx_rules_category_type ON rules(event_category, event_type);
CREATE INDEX idx_rules_priority ON rules(priority DESC);

-- Anomaly configs table: stores anomaly detection configurations
CREATE TABLE anomaly_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    app_id VARCHAR(255), -- NULL means all apps
    event_category VARCHAR(100), -- NULL means all categories
    event_type VARCHAR(100), -- NULL means all types
    detection_type VARCHAR(50) NOT NULL, -- threshold, rate, count
    config JSONB NOT NULL DEFAULT '{}', -- Type-specific config (see below)
    cooldown_seconds INTEGER NOT NULL DEFAULT 300, -- Min time between alerts
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- detection_type configs:
-- threshold: {"path":"$.field","min":0,"max":100}
-- rate: {"max_per_minute":100}
-- count: {"window_seconds":60,"max_count":1000}

CREATE INDEX idx_anomaly_configs_enabled ON anomaly_configs(enabled);
CREATE INDEX idx_anomaly_configs_app_id ON anomaly_configs(app_id);
CREATE INDEX idx_anomaly_configs_category_type ON anomaly_configs(event_category, event_type);

-- Webhook deliveries table: queue for webhook delivery with retry state
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    rule_id UUID REFERENCES rules(id) ON DELETE SET NULL,
    anomaly_config_id UUID REFERENCES anomaly_configs(id) ON DELETE SET NULL,
    payload JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, in_progress, delivered, failed, dead_letter
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_attempt_at TIMESTAMPTZ,
    last_error TEXT,
    last_status_code INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ
);

CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_next_attempt ON webhook_deliveries(next_attempt_at) WHERE status IN ('pending', 'in_progress');
CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);

-- Anomaly events table: log of detected anomalies
CREATE TABLE anomaly_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    anomaly_config_id UUID NOT NULL REFERENCES anomaly_configs(id) ON DELETE CASCADE,
    app_id VARCHAR(255),
    event_category VARCHAR(100),
    event_type VARCHAR(100),
    detection_type VARCHAR(50) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}', -- {"value":150,"threshold_max":100} or {"rate":120,"max_per_minute":100}
    event_data JSONB, -- The event that triggered the anomaly (for threshold type)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_anomaly_events_config_id ON anomaly_events(anomaly_config_id);
CREATE INDEX idx_anomaly_events_app_id ON anomaly_events(app_id);
CREATE INDEX idx_anomaly_events_created_at ON anomaly_events(created_at);

-- Anomaly state table: sliding window state for rate/count detection
CREATE TABLE anomaly_state (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    anomaly_config_id UUID NOT NULL REFERENCES anomaly_configs(id) ON DELETE CASCADE,
    app_id VARCHAR(255) NOT NULL,
    window_key VARCHAR(255) NOT NULL, -- e.g., "2024-01-15T10:30" for minute-based windows
    event_count INTEGER NOT NULL DEFAULT 0,
    last_alert_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(anomaly_config_id, app_id, window_key)
);

CREATE INDEX idx_anomaly_state_config_app ON anomaly_state(anomaly_config_id, app_id);
CREATE INDEX idx_anomaly_state_window ON anomaly_state(window_key);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply update triggers
CREATE TRIGGER update_webhooks_updated_at
    BEFORE UPDATE ON webhooks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_rules_updated_at
    BEFORE UPDATE ON rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_anomaly_configs_updated_at
    BEFORE UPDATE ON anomaly_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_anomaly_state_updated_at
    BEFORE UPDATE ON anomaly_state
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Grant permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO hive;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO hive;
