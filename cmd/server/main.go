// Command server runs the HTTP gateway for event ingestion.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v10"

	"github.com/SebastienMelki/causality/internal/gateway"
	"github.com/SebastienMelki/causality/internal/nats"
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
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Connect to NATS
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

	// Create and start HTTP server
	server, err := gateway.NewServer(cfg.Gateway, natsClient, publisher, logger)
	if err != nil {
		return err
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

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
