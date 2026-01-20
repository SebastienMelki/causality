// Command warehouse-sink consumes events from NATS and writes them to S3/MinIO.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v10"

	"github.com/SebastienMelki/causality/internal/nats"
	"github.com/SebastienMelki/causality/internal/warehouse"
)

// Config holds all warehouse sink configuration.
type Config struct {
	// LogLevel is the log level (debug, info, warn, error)
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// LogFormat is the log format (json, text)
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// NATS configuration
	NATS nats.Config `envPrefix:""`

	// Warehouse configuration
	Warehouse warehouse.Config `envPrefix:""`

	// ConsumerName is the NATS consumer name
	ConsumerName string `env:"CONSUMER_NAME" envDefault:"warehouse-sink"`
}

func main() {
	// Load configuration from environment
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	logger.Info("starting warehouse sink",
		"log_level", cfg.LogLevel,
		"nats_url", cfg.NATS.URL,
		"s3_endpoint", cfg.Warehouse.S3.Endpoint,
		"s3_bucket", cfg.Warehouse.S3.Bucket,
		"consumer", cfg.ConsumerName,
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
		logger.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	// Setup stream and consumers
	streamMgr := nats.NewStreamManager(natsClient.JetStream(), cfg.NATS.Stream, logger)
	stream, err := streamMgr.EnsureStream(ctx)
	if err != nil {
		logger.Error("failed to ensure stream", "error", err)
		os.Exit(1)
	}

	// Create consumers
	consumerConfigs := nats.DefaultConsumerConfigs()
	if err := streamMgr.EnsureConsumers(ctx, stream, consumerConfigs); err != nil {
		logger.Error("failed to ensure consumers", "error", err)
		os.Exit(1)
	}

	// Create S3 client
	s3Client, err := warehouse.NewS3Client(ctx, cfg.Warehouse.S3, logger)
	if err != nil {
		logger.Error("failed to create S3 client", "error", err)
		os.Exit(1)
	}

	// Ensure bucket exists
	if err := s3Client.EnsureBucket(ctx); err != nil {
		logger.Error("failed to ensure bucket", "error", err)
		os.Exit(1)
	}

	// Create and start consumer
	consumer := warehouse.NewConsumer(
		natsClient.JetStream(),
		cfg.Warehouse,
		s3Client,
		cfg.ConsumerName,
		cfg.NATS.Stream.Name,
		logger,
	)

	if err := consumer.Start(ctx); err != nil {
		logger.Error("failed to start consumer", "error", err)
		os.Exit(1)
	}

	logger.Info("warehouse sink started")

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("received shutdown signal", "signal", sig)

	// Graceful shutdown
	logger.Info("initiating graceful shutdown")
	cancel()

	if err := consumer.Stop(context.Background()); err != nil {
		logger.Error("consumer stop error", "error", err)
	}

	if err := natsClient.Drain(); err != nil {
		logger.Error("NATS drain error", "error", err)
	}

	logger.Info("warehouse sink stopped")
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
