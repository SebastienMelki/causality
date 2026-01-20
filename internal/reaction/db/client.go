// Package db provides database operations for the reaction engine.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// ErrDatabaseConnection indicates a database connection error.
var ErrDatabaseConnection = errors.New("database connection error")

// Config holds PostgreSQL connection settings.
type Config struct {
	// Host is the PostgreSQL host
	Host string `env:"HOST" envDefault:"localhost"`

	// Port is the PostgreSQL port
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
	MaxOpenConns int `env:"MAX_OPEN_CONNS" envDefault:"25"`

	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int `env:"MAX_IDLE_CONNS" envDefault:"5"`

	// ConnMaxLifetime is the maximum connection lifetime
	ConnMaxLifetime time.Duration `env:"CONN_MAX_LIFETIME" envDefault:"5m"`
}

// Client provides database access for the reaction engine.
type Client struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewClient creates a new database client.
func NewClient(ctx context.Context, cfg Config, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "reaction-db")

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: %w", ErrDatabaseConnection, err)
	}

	logger.Info("connected to database",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.Name,
	)

	return &Client{
		db:     db,
		logger: logger,
	}, nil
}

// Close closes the database connection.
func (c *Client) Close() error {
	return c.db.Close()
}

// DB returns the underlying database connection for use by repository structs.
func (c *Client) DB() *sql.DB {
	return c.db
}

// Ping checks if the database connection is still alive.
func (c *Client) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// BeginTx starts a new transaction.
func (c *Client) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return c.db.BeginTx(ctx, opts)
}
