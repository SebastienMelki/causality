// Command server runs the HTTP gateway for event ingestion.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v10"
	_ "github.com/lib/pq"

	"github.com/SebastienMelki/causality/internal/auth"
	"github.com/SebastienMelki/causality/internal/dedup"
	"github.com/SebastienMelki/causality/internal/gateway"
	"github.com/SebastienMelki/causality/internal/nats"
	"github.com/SebastienMelki/causality/internal/observability"
)

// Config holds all server configuration.
type Config struct {
	// LogLevel is the log level (debug, info, warn, error).
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// LogFormat is the log format (json, text).
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// HTTP gateway configuration.
	Gateway gateway.Config `envPrefix:""`

	// NATS configuration.
	NATS nats.Config `envPrefix:""`

	// Database configuration for auth module.
	Database DatabaseConfig `envPrefix:"DATABASE_"`

	// Dedup configuration.
	Dedup dedup.Config `envPrefix:""`
}

// DatabaseConfig holds PostgreSQL connection configuration.
type DatabaseConfig struct {
	Host     string `env:"HOST"     envDefault:"localhost"`
	Port     int    `env:"PORT"     envDefault:"5432"`
	User     string `env:"USER"     envDefault:"hive"`
	Password string `env:"PASSWORD" envDefault:"hive"`
	Name     string `env:"NAME"     envDefault:"causality_server"`
	SSLMode  string `env:"SSL_MODE" envDefault:"disable"`
}

// DSN returns the PostgreSQL connection string.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration from environment
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return err
	}

	// Setup logger
	logger := setupLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	logger.Info("starting causality server",
		"log_level", cfg.LogLevel,
		"http_addr", cfg.Gateway.Addr,
		"nats_url", cfg.NATS.URL,
		"db_host", cfg.Database.Host,
		"db_name", cfg.Database.Name,
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// --- Observability module ---
	obs, err := observability.New("causality-server")
	if err != nil {
		return fmt.Errorf("failed to create observability module: %w", err)
	}

	metrics, err := observability.NewMetrics(obs.Meter())
	if err != nil {
		return fmt.Errorf("failed to create metrics: %w", err)
	}

	// --- Database connection ---
	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	logger.Info("connected to database", "host", cfg.Database.Host, "name", cfg.Database.Name)

	// --- Auth module ---
	authModule := auth.New(db, logger)

	// --- Dedup module ---
	dedupModule := dedup.New(cfg.Dedup, metrics, logger)
	dedupModule.Start(ctx)

	// --- NATS ---
	natsClient, err := nats.NewClient(ctx, cfg.NATS, logger)
	if err != nil {
		return err
	}
	defer natsClient.Close()

	// Setup stream and consumers
	streamMgr := nats.NewStreamManager(natsClient.JetStream(), cfg.NATS.Stream, logger)
	if _, err := streamMgr.EnsureStream(ctx); err != nil {
		return err
	}

	// Create publisher
	publisher := nats.NewPublisher(natsClient.JetStream(), cfg.NATS.Stream.Name, logger)

	// --- HTTP Server ---
	serverOpts := &gateway.ServerOpts{
		AuthMiddleware:      authModule.AuthMiddleware(),
		MetricsHandler:      obs.MetricsHandler(),
		Metrics:             metrics,
		Dedup:               dedupModule,
		AdminRouteRegistrar: authModule.RegisterAdminRoutes,
	}

	server, err := gateway.NewServer(cfg.Gateway, natsClient, publisher, logger, serverOpts)
	if err != nil {
		return err
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	logger.Info("causality server started",
		"addr", cfg.Gateway.Addr,
		"auth", "enabled",
		"dedup_window", cfg.Dedup.Window.String(),
		"max_body_size", cfg.Gateway.MaxBodySize,
		"rate_limit_per_key_rps", cfg.Gateway.RateLimit.PerKeyRPS,
	)

	// Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		logger.Info("received shutdown signal", "signal", sig)
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "error", err)
		}
	}

	// Graceful shutdown
	logger.Info("initiating graceful shutdown")
	cancel()

	if err := server.Shutdown(context.Background()); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	dedupModule.Stop()
	logger.Info("dedup module stopped")

	if err := obs.Shutdown(context.Background()); err != nil {
		logger.Error("observability shutdown error", "error", err)
	}
	logger.Info("observability module stopped")

	if err := natsClient.Drain(); err != nil {
		logger.Error("NATS drain error", "error", err)
	}

	logger.Info("server stopped")
	return nil
}

// setupLogger creates a logger based on configuration.
func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
