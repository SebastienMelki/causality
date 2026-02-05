// Package dlq provides the dead-letter queue module for capturing and managing
// messages that have exceeded their maximum delivery attempts in NATS JetStream.
//
// When a consumer fails to acknowledge a message after MaxDeliver attempts,
// NATS emits an advisory event. This module listens for those advisories,
// fetches the original failed message, and republishes it to a dedicated DLQ
// stream for later investigation and reprocessing.
package dlq

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/SebastienMelki/causality/internal/dlq/internal/service"
	"github.com/SebastienMelki/causality/internal/observability"
)

// Config holds configuration for the DLQ module.
type Config struct {
	// AlertThreshold is the number of DLQ messages at which an alert should be raised
	AlertThreshold int64 `env:"DLQ_ALERT_THRESHOLD" envDefault:"100"`

	// DLQStreamName is the name of the DLQ stream to query for counts
	DLQStreamName string `env:"NATS_STREAM_DLQ_STREAM_NAME" envDefault:"CAUSALITY_DLQ"`
}

// Module is the dead-letter queue module facade.
// It wraps the DLQ service and provides a clean public API.
type Module struct {
	service       *service.DLQService
	config        Config
	dlqStreamName string
}

// New creates a new DLQ module.
//
// Parameters:
//   - js: JetStream context for publishing to DLQ and fetching original messages
//   - nc: raw NATS connection for subscribing to advisory subjects (core NATS)
//   - streamName: the main event stream name (e.g., "CAUSALITY_EVENTS")
//   - consumerNames: consumer durable names to monitor for MaxDeliver advisories
//   - cfg: module configuration
//   - metrics: observability metrics (may be nil)
//   - logger: structured logger
func New(
	js jetstream.JetStream,
	nc *nats.Conn,
	streamName string,
	consumerNames []string,
	cfg Config,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *Module {
	if logger == nil {
		logger = slog.Default()
	}

	dlqSvc := service.NewDLQService(js, nc, streamName, consumerNames, metrics, logger)

	return &Module{
		service:       dlqSvc,
		config:        cfg,
		dlqStreamName: cfg.DLQStreamName,
	}
}

// Start begins listening for MaxDeliver advisory events.
func (m *Module) Start(ctx context.Context) error {
	return m.service.Start(ctx)
}

// Stop unsubscribes from all advisory subscriptions and shuts down the service.
func (m *Module) Stop() {
	m.service.Stop()
}

// GetDLQCount returns the number of messages currently in the DLQ stream.
func (m *Module) GetDLQCount(ctx context.Context) (int64, error) {
	return m.service.GetDLQCount(ctx, m.dlqStreamName)
}

// AlertThreshold returns the configured alert threshold.
func (m *Module) AlertThreshold() int64 {
	return m.config.AlertThreshold
}
