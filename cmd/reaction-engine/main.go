// Command reaction-engine evaluates events against rules and detects anomalies.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v10"

	"github.com/SebastienMelki/causality/internal/dlq"
	"github.com/SebastienMelki/causality/internal/nats"
	"github.com/SebastienMelki/causality/internal/observability"
	"github.com/SebastienMelki/causality/internal/reaction"
	"github.com/SebastienMelki/causality/internal/reaction/db"
)

// Config holds all reaction engine configuration.
type Config struct {
	// LogLevel is the log level (debug, info, warn, error).
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// LogFormat is the log format (json, text).
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// MetricsAddr is the address for the Prometheus metrics endpoint.
	MetricsAddr string `env:"METRICS_ADDR" envDefault:":9091"`

	// NATS configuration.
	NATS nats.Config `envPrefix:""`

	// DLQ configuration.
	DLQ dlq.Config `envPrefix:""`

	// Reaction engine configuration.
	Reaction reaction.Config `envPrefix:""`

	// ConsumerName is the NATS consumer name.
	ConsumerName string `env:"CONSUMER_NAME" envDefault:"analysis-engine"`
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

	logger.Info("starting reaction engine",
		"log_level", cfg.LogLevel,
		"nats_url", cfg.NATS.URL,
		"consumer", cfg.ConsumerName,
		"metrics_addr", cfg.MetricsAddr,
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize observability (OTel + Prometheus)
	obs, err := observability.New("reaction-engine")
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

	// Ensure DLQ stream exists
	if _, err := streamMgr.EnsureDLQStream(ctx); err != nil {
		return err
	}

	// Create and start DLQ module
	dlqModule := dlq.New(
		natsClient.JetStream(),
		natsClient.Conn(),
		cfg.NATS.Stream.Name,
		[]string{cfg.ConsumerName},
		cfg.DLQ,
		metrics,
		logger,
	)
	if err := dlqModule.Start(ctx); err != nil {
		return err
	}

	// Connect to PostgreSQL
	dbClient, err := db.NewClient(ctx, cfg.Reaction.Database, logger)
	if err != nil {
		return err
	}
	defer func() { _ = dbClient.Close() }()

	// Create repositories
	ruleRepo := db.NewRuleRepository(dbClient)
	webhookRepo := db.NewWebhookRepository(dbClient)
	deliveryRepo := db.NewDeliveryRepository(dbClient)
	anomalyConfigRepo := db.NewAnomalyConfigRepository(dbClient)

	// Create rule engine
	engine := reaction.NewEngine(
		ruleRepo,
		webhookRepo,
		deliveryRepo,
		natsClient.JetStream(),
		cfg.Reaction.Engine,
		cfg.Reaction.Dispatcher,
		logger,
	)
	if err := engine.Start(ctx); err != nil {
		return err
	}

	// Create webhook dispatcher
	dispatcher := reaction.NewDispatcher(
		deliveryRepo,
		webhookRepo,
		cfg.Reaction.Dispatcher,
		logger,
	)
	dispatcher.Start(ctx)

	// Create anomaly detector
	anomalyDetector := reaction.NewAnomalyDetector(
		anomalyConfigRepo,
		natsClient.JetStream(),
		cfg.Reaction.Anomaly,
		logger,
	)
	if err := anomalyDetector.Start(ctx); err != nil {
		return err
	}

	// Create and start consumer
	consumer := reaction.NewConsumer(
		natsClient.JetStream(),
		engine,
		anomalyDetector,
		cfg.ConsumerName,
		cfg.NATS.Stream.Name,
		cfg.Reaction.Consumer,
		cfg.Reaction.ShutdownTimeout,
		logger,
		metrics,
	)

	if err := consumer.Start(ctx); err != nil {
		return err
	}

	logger.Info("reaction engine started")

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("received shutdown signal", "signal", sig)

	// Graceful shutdown
	logger.Info("initiating graceful shutdown")
	cancel()

	if err := consumer.Stop(context.Background()); err != nil {
		logger.Error("consumer stop error", "error", err)
	}

	anomalyDetector.Stop()
	dispatcher.Stop()
	engine.Stop()
	dlqModule.Stop()

	// Stop metrics server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Reaction.ShutdownTimeout)
	defer shutdownCancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server shutdown error", "error", err)
	}

	if err := natsClient.Drain(); err != nil {
		logger.Error("NATS drain error", "error", err)
	}

	logger.Info("reaction engine stopped")
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
