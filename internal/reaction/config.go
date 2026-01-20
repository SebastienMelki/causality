// Package reaction provides the reaction engine for event-driven rules and anomaly detection.
package reaction

import (
	"time"

	"github.com/SebastienMelki/causality/internal/reaction/db"
)

// Config holds the complete reaction engine configuration.
type Config struct {
	// Database configuration
	Database db.Config `envPrefix:"DATABASE_"`

	// Engine configuration
	Engine EngineConfig `envPrefix:"ENGINE_"`

	// Dispatcher configuration
	Dispatcher DispatcherConfig `envPrefix:"DISPATCHER_"`

	// Anomaly detection configuration
	Anomaly AnomalyConfig `envPrefix:"ANOMALY_"`
}

// EngineConfig holds rule engine settings.
type EngineConfig struct {
	// RuleRefreshInterval is how often to reload rules from the database
	RuleRefreshInterval time.Duration `env:"RULE_REFRESH_INTERVAL" envDefault:"30s"`

	// MaxConcurrentEvaluations is the max number of concurrent rule evaluations
	MaxConcurrentEvaluations int `env:"MAX_CONCURRENT_EVALUATIONS" envDefault:"100"`
}

// DispatcherConfig holds webhook dispatcher settings.
type DispatcherConfig struct {
	// Workers is the number of webhook delivery workers
	Workers int `env:"WORKERS" envDefault:"5"`

	// PollInterval is how often to poll for pending deliveries
	PollInterval time.Duration `env:"POLL_INTERVAL" envDefault:"1s"`

	// BatchSize is the number of deliveries to fetch per poll
	BatchSize int `env:"BATCH_SIZE" envDefault:"100"`

	// InitialBackoff is the initial retry backoff duration
	InitialBackoff time.Duration `env:"INITIAL_BACKOFF" envDefault:"1s"`

	// MaxBackoff is the maximum retry backoff duration
	MaxBackoff time.Duration `env:"MAX_BACKOFF" envDefault:"5m"`

	// BackoffMultiplier is the backoff multiplier for exponential backoff
	BackoffMultiplier float64 `env:"BACKOFF_MULTIPLIER" envDefault:"2.0"`

	// MaxAttempts is the maximum number of delivery attempts
	MaxAttempts int `env:"MAX_ATTEMPTS" envDefault:"5"`

	// RequestTimeout is the HTTP request timeout for webhook calls
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" envDefault:"30s"`
}

// AnomalyConfig holds anomaly detection settings.
type AnomalyConfig struct {
	// ConfigRefreshInterval is how often to reload anomaly configs
	ConfigRefreshInterval time.Duration `env:"CONFIG_REFRESH_INTERVAL" envDefault:"30s"`

	// StateCleanupInterval is how often to clean up old state records
	StateCleanupInterval time.Duration `env:"STATE_CLEANUP_INTERVAL" envDefault:"1h"`

	// StateRetentionDuration is how long to keep state records
	StateRetentionDuration time.Duration `env:"STATE_RETENTION_DURATION" envDefault:"24h"`
}

// BasicAuthConfig holds basic auth configuration.
type BasicAuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// BearerAuthConfig holds bearer token configuration.
type BearerAuthConfig struct {
	Token string `json:"token"`
}

// HMACAuthConfig holds HMAC signature configuration.
type HMACAuthConfig struct {
	Secret string `json:"secret"` // HMAC secret key
	Header string `json:"header"` // Header name for the signature (default: X-Signature)
}
