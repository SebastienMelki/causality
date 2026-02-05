// Command warehouse-sink consumes events from NATS and writes them to S3/MinIO.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v10"

	"github.com/SebastienMelki/causality/internal/nats"
	"github.com/SebastienMelki/causality/internal/observability"
	"github.com/SebastienMelki/causality/internal/warehouse"
)

// Config holds all warehouse sink configuration.
type Config struct {
	// LogLevel is the log level (debug, info, warn, error).
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// LogFormat is the log format (json, text).
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// MetricsAddr is the address for the Prometheus metrics endpoint.
	MetricsAddr string `env:"METRICS_ADDR" envDefault:":9090"`

	// NATS configuration.
	NATS nats.Config `envPrefix:""`

	// Warehouse configuration.
	Warehouse warehouse.Config `envPrefix:""`

	// ConsumerName is the NATS consumer name.
	ConsumerName string `env:"CONSUMER_NAME" envDefault:"warehouse-sink"`
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

	logger.Info("starting warehouse sink",
		"log_level", cfg.LogLevel,
		"nats_url", cfg.NATS.URL,
		"s3_endpoint", cfg.Warehouse.S3.Endpoint,
		"s3_bucket", cfg.Warehouse.S3.Bucket,
		"consumer", cfg.ConsumerName,
		"metrics_addr", cfg.MetricsAddr,
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize observability (OTel + Prometheus)
	obs, err := observability.New("warehouse-sink")
	if err != nil {
		return err
	}
	defer func() {
		if shutErr := obs.Shutdown(context.Background()); shutErr != nil {
			logger.Error("observability shutdown error", "error", shutErr)
		}
	}()

	// Create metrics instruments
	metrics, err := observability.NewMetrics(obs.Meter())
	if err != nil {
		return err
	}

	// Start metrics and health HTTP server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", obs.MetricsHandler())
	metricsMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	metricsServer := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metricsMux,
	}
	go func() {
		logger.Info("starting metrics server", "addr", cfg.MetricsAddr)
		if srvErr := metricsServer.ListenAndServe(); srvErr != nil && srvErr != http.ErrServerClosed {
			logger.Error("metrics server error", "error", srvErr)
		}
	}()

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
	stream, err := streamMgr.EnsureStream(ctx)
	if err != nil {
		return err
	}

	// Create consumers
	consumerConfigs := nats.DefaultConsumerConfigs()
	if err := streamMgr.EnsureConsumers(ctx, stream, consumerConfigs); err != nil {
		return err
	}

	// Create S3 client
	s3Client, err := warehouse.NewS3Client(ctx, cfg.Warehouse.S3, logger)
	if err != nil {
		return err
	}

	// Ensure bucket exists
	if err := s3Client.EnsureBucket(ctx); err != nil {
		return err
	}

	// Create and start consumer
	consumer := warehouse.NewConsumer(
		natsClient.JetStream(),
		cfg.Warehouse,
		s3Client,
		cfg.ConsumerName,
		cfg.NATS.Stream.Name,
		logger,
		metrics,
	)

	if err := consumer.Start(ctx); err != nil {
		return err
	}

	logger.Info("warehouse sink started")

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("received shutdown signal", "signal", sig)

	// Graceful shutdown
	logger.Info("initiating graceful shutdown")
	cancel()

	// Stop consumer with shutdown timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Warehouse.ShutdownTimeout)
	defer shutdownCancel()

	if err := consumer.Stop(shutdownCtx); err != nil {
		logger.Error("consumer stop error", "error", err)
	}

	// Stop metrics server
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server shutdown error", "error", err)
	}

	if err := natsClient.Drain(); err != nil {
		logger.Error("NATS drain error", "error", err)
	}

	logger.Info("warehouse sink stopped")
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
