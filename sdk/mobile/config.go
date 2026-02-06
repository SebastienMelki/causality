package mobile

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Config holds the SDK configuration.
// All fields use gomobile-compatible types (string, int, bool).
// JSON tags enable initialization from serialized config strings.
type Config struct {
	// APIKey is the API key for authentication (required).
	APIKey string `json:"api_key"`

	// Endpoint is the Causality server URL (required, e.g., "https://analytics.example.com").
	Endpoint string `json:"endpoint"`

	// AppID is the application identifier (required).
	AppID string `json:"app_id"`

	// BatchSize is the maximum number of events per batch (default: 50).
	BatchSize int `json:"batch_size,omitempty"`

	// FlushIntervalMs is the maximum time between flushes in milliseconds (default: 30000).
	FlushIntervalMs int `json:"flush_interval_ms,omitempty"`

	// MaxQueueSize is the maximum number of events in the queue (default: 1000).
	MaxQueueSize int `json:"max_queue_size,omitempty"`

	// SessionTimeoutMs is the session inactivity timeout in milliseconds (default: 1800000 = 30min).
	SessionTimeoutMs int `json:"session_timeout_ms,omitempty"`

	// DebugMode enables verbose logging (default: false).
	DebugMode bool `json:"debug_mode,omitempty"`

	// EnableSessionTracking enables automatic session tracking (default: true).
	EnableSessionTracking *bool `json:"enable_session_tracking,omitempty"`

	// PersistentDeviceID enables a persistent device ID that survives app reinstalls (default: false).
	// Privacy-first: disabled by default. Enable for cross-reinstall tracking.
	PersistentDeviceID bool `json:"persistent_device_id,omitempty"`

	// OfflineRetentionMs is how long to keep offline events in milliseconds (default: 86400000 = 24h).
	OfflineRetentionMs int `json:"offline_retention_ms,omitempty"`

	// DataPath is the platform-specific path for SQLite storage (required for persistence).
	DataPath string `json:"data_path,omitempty"`
}

// Default configuration values.
const (
	DefaultBatchSize          = 50
	DefaultFlushIntervalMs    = 30000  // 30 seconds
	DefaultMaxQueueSize       = 1000
	DefaultSessionTimeoutMs   = 1800000 // 30 minutes
	DefaultOfflineRetentionMs = 86400000 // 24 hours

	MinBatchSize       = 1
	MinFlushIntervalMs = 1000 // 1 second minimum
	MinQueueSize       = 10
)

// validate checks that required fields are set and values are valid.
// Returns empty string on success, error message on failure.
func (c *Config) validate() string {
	if strings.TrimSpace(c.APIKey) == "" {
		return "api_key is required"
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return "endpoint is required"
	}
	if strings.TrimSpace(c.AppID) == "" {
		return "app_id is required"
	}

	// Validate endpoint is a valid URL with scheme
	parsed, err := url.Parse(c.Endpoint)
	if err != nil {
		return fmt.Sprintf("endpoint is not a valid URL: %s", err.Error())
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "endpoint must include scheme and host (e.g., https://analytics.example.com)"
	}

	// Validate optional numeric fields if set
	if c.BatchSize < 0 {
		return "batch_size must be non-negative"
	}
	if c.FlushIntervalMs < 0 {
		return "flush_interval_ms must be non-negative"
	}
	if c.MaxQueueSize < 0 {
		return "max_queue_size must be non-negative"
	}
	if c.SessionTimeoutMs < 0 {
		return "session_timeout_ms must be non-negative"
	}
	if c.OfflineRetentionMs < 0 {
		return "offline_retention_ms must be non-negative"
	}

	return ""
}

// applyDefaults fills in default values for unset optional fields.
func (c *Config) applyDefaults() {
	// Trim trailing slash from endpoint
	c.Endpoint = strings.TrimSuffix(c.Endpoint, "/")

	if c.BatchSize == 0 {
		c.BatchSize = DefaultBatchSize
	}
	if c.FlushIntervalMs == 0 {
		c.FlushIntervalMs = DefaultFlushIntervalMs
	}
	if c.MaxQueueSize == 0 {
		c.MaxQueueSize = DefaultMaxQueueSize
	}
	if c.SessionTimeoutMs == 0 {
		c.SessionTimeoutMs = DefaultSessionTimeoutMs
	}
	if c.OfflineRetentionMs == 0 {
		c.OfflineRetentionMs = DefaultOfflineRetentionMs
	}

	// Session tracking defaults to true
	if c.EnableSessionTracking == nil {
		enabled := true
		c.EnableSessionTracking = &enabled
	}
}

// configFromJSON parses a JSON config string and returns a validated Config.
func configFromJSON(jsonStr string) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, fmt.Errorf("invalid config JSON: %w", err)
	}

	if errMsg := cfg.validate(); errMsg != "" {
		return nil, fmt.Errorf("config validation failed: %s", errMsg)
	}

	cfg.applyDefaults()
	return &cfg, nil
}
