// Package gateway provides the HTTP gateway for event ingestion.
package gateway

import (
	"time"
)

// Config holds HTTP gateway configuration.
type Config struct {
	// Addr is the address to listen on (e.g., ":8080")
	Addr string `env:"HTTP_ADDR" envDefault:":8080"`

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"10s"`

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`

	// IdleTimeout is the maximum amount of time to wait for the next request
	IdleTimeout time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`

	// MaxHeaderBytes is the maximum size of request headers
	MaxHeaderBytes int `env:"HTTP_MAX_HEADER_BYTES" envDefault:"1048576"` // 1MB

	// CORS configuration
	CORS CORSConfig `envPrefix:"CORS_"`

	// Rate limiting configuration
	RateLimit RateLimitConfig `envPrefix:"RATE_LIMIT_"`

	// MaxBodySize is the maximum request body size in bytes (default: 5 MB)
	MaxBodySize int64 `env:"MAX_BODY_SIZE" envDefault:"5242880"`

	// MaxBatchEvents is the maximum number of events in a single batch request
	MaxBatchEvents int `env:"MAX_BATCH_EVENTS" envDefault:"1000"`

	// Shutdown timeout for graceful shutdown
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"30s"`
}

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	// AllowedOrigins is a list of allowed origins
	AllowedOrigins []string `env:"ALLOWED_ORIGINS" envDefault:"*"`

	// AllowedMethods is a list of allowed HTTP methods
	AllowedMethods []string `env:"ALLOWED_METHODS" envDefault:"GET,POST,PUT,DELETE,OPTIONS"`

	// AllowedHeaders is a list of allowed headers
	AllowedHeaders []string `env:"ALLOWED_HEADERS" envDefault:"Accept,Authorization,Content-Type,X-Request-ID,X-Correlation-ID,X-API-Key"`

	// ExposedHeaders is a list of headers exposed to the client
	ExposedHeaders []string `env:"EXPOSED_HEADERS" envDefault:"X-Request-ID"`

	// AllowCredentials indicates whether cookies are allowed
	AllowCredentials bool `env:"ALLOW_CREDENTIALS" envDefault:"false"`

	// MaxAge is the max age (in seconds) for preflight cache
	MaxAge int `env:"MAX_AGE" envDefault:"86400"` // 24 hours
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// Enabled indicates whether rate limiting is enabled
	Enabled bool `env:"ENABLED" envDefault:"true"`

	// RequestsPerSecond is the number of requests allowed per second (global)
	RequestsPerSecond float64 `env:"REQUESTS_PER_SECOND" envDefault:"1000"`

	// BurstSize is the maximum burst size (global)
	BurstSize int `env:"BURST_SIZE" envDefault:"2000"`

	// PerKeyRPS is the per-API-key rate limit in requests per second
	PerKeyRPS float64 `env:"PER_KEY_RPS" envDefault:"1000"`

	// PerKeyBurst is the per-API-key burst size
	PerKeyBurst int `env:"PER_KEY_BURST" envDefault:"2000"`
}
