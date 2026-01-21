// Package gateway provides the HTTP gateway for event ingestion.
package gateway

import (
	"fmt"
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

	// Shutdown timeout for graceful shutdown
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"30s"`

	// Admin UI configuration
	Admin AdminConfig `envPrefix:"ADMIN_"`

	// Database configuration for admin UI
	Database DatabaseConfig `envPrefix:"DATABASE_"`

	// Trino configuration for event browser
	Trino TrinoConfig `envPrefix:"TRINO_"`
}

// AdminConfig holds admin UI configuration.
type AdminConfig struct {
	// Enabled indicates whether the admin UI is enabled
	Enabled bool `env:"ENABLED" envDefault:"true"`

	// BasePath is the base path for admin routes
	BasePath string `env:"BASE_PATH" envDefault:"/admin"`
}

// DatabaseConfig holds PostgreSQL database configuration.
type DatabaseConfig struct {
	// Host is the database host
	Host string `env:"HOST" envDefault:"localhost"`

	// Port is the database port
	Port int `env:"PORT" envDefault:"5432"`

	// User is the database user
	User string `env:"USER" envDefault:"hive"`

	// Password is the database password
	Password string `env:"PASSWORD" envDefault:"hive"`

	// Name is the database name
	Name string `env:"NAME" envDefault:"reaction_engine"`

	// SSLMode is the SSL mode (disable, require, verify-ca, verify-full)
	SSLMode string `env:"SSL_MODE" envDefault:"disable"`

	// MaxOpenConns is the maximum number of open connections
	MaxOpenConns int `env:"MAX_OPEN_CONNS" envDefault:"10"`

	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int `env:"MAX_IDLE_CONNS" envDefault:"5"`

	// ConnMaxLifetime is the maximum connection lifetime
	ConnMaxLifetime time.Duration `env:"CONN_MAX_LIFETIME" envDefault:"1h"`
}

// DSN returns the PostgreSQL connection string.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

// TrinoConfig holds Trino configuration.
type TrinoConfig struct {
	// Host is the Trino host
	Host string `env:"HOST" envDefault:"localhost"`

	// Port is the Trino port
	Port int `env:"PORT" envDefault:"8080"`

	// Catalog is the Trino catalog to use
	Catalog string `env:"CATALOG" envDefault:"hive"`

	// Schema is the Trino schema to use
	Schema string `env:"SCHEMA" envDefault:"causality"`

	// User is the Trino user
	User string `env:"USER" envDefault:"trino"`
}

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	// AllowedOrigins is a list of allowed origins
	AllowedOrigins []string `env:"ALLOWED_ORIGINS" envDefault:"*"`

	// AllowedMethods is a list of allowed HTTP methods
	AllowedMethods []string `env:"ALLOWED_METHODS" envDefault:"GET,POST,PUT,DELETE,OPTIONS"`

	// AllowedHeaders is a list of allowed headers
	AllowedHeaders []string `env:"ALLOWED_HEADERS" envDefault:"Accept,Authorization,Content-Type,X-Request-ID,X-Correlation-ID"`

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

	// RequestsPerSecond is the number of requests allowed per second
	RequestsPerSecond float64 `env:"REQUESTS_PER_SECOND" envDefault:"1000"`

	// BurstSize is the maximum burst size
	BurstSize int `env:"BURST_SIZE" envDefault:"2000"`
}
