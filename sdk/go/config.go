package causality

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// Default configuration values.
const (
	DefaultBatchSize     = 50
	DefaultFlushInterval = 30 * time.Second
	DefaultMaxRetries    = 3
	DefaultTimeout       = 10 * time.Second
)

// Config holds the SDK configuration.
type Config struct {
	// APIKey is the API key for authentication (required)
	APIKey string

	// Endpoint is the Causality server URL (required, e.g., "http://localhost:8080")
	Endpoint string

	// AppID is the application identifier (required)
	AppID string

	// BatchSize is the maximum number of events per batch (default: 50)
	BatchSize int

	// FlushInterval is the maximum time between flushes (default: 30s)
	FlushInterval time.Duration

	// MaxRetries is the maximum number of retry attempts on 5xx errors (default: 3)
	MaxRetries int

	// Timeout is the HTTP request timeout (default: 10s)
	Timeout time.Duration
}

// validate checks that required fields are set and values are valid.
func (c *Config) validate() error {
	if c.APIKey == "" {
		return errors.New("causality: APIKey is required")
	}
	if c.Endpoint == "" {
		return errors.New("causality: Endpoint is required")
	}
	if c.AppID == "" {
		return errors.New("causality: AppID is required")
	}

	// Validate endpoint is a valid URL
	if _, err := url.Parse(c.Endpoint); err != nil {
		return errors.New("causality: Endpoint must be a valid URL")
	}

	// Validate batch size
	if c.BatchSize < 0 {
		return errors.New("causality: BatchSize must be non-negative")
	}

	// Validate flush interval
	if c.FlushInterval < 0 {
		return errors.New("causality: FlushInterval must be non-negative")
	}

	// Validate max retries
	if c.MaxRetries < 0 {
		return errors.New("causality: MaxRetries must be non-negative")
	}

	// Validate timeout
	if c.Timeout < 0 {
		return errors.New("causality: Timeout must be non-negative")
	}

	return nil
}

// withDefaults returns a copy of the config with default values applied.
func (c Config) withDefaults() Config {
	cfg := c

	// Trim trailing slash from endpoint
	cfg.Endpoint = strings.TrimSuffix(cfg.Endpoint, "/")

	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = DefaultFlushInterval
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = DefaultMaxRetries
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	return cfg
}
