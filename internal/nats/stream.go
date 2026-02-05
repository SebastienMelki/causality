package nats

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

// StreamManager handles JetStream stream creation and management.
type StreamManager struct {
	js     jetstream.JetStream
	config StreamConfig
	logger *slog.Logger
}

// NewStreamManager creates a new stream manager.
func NewStreamManager(js jetstream.JetStream, cfg StreamConfig, logger *slog.Logger) *StreamManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &StreamManager{
		js:     js,
		config: cfg,
		logger: logger.With("component", "stream-manager"),
	}
}

// EnsureStream creates or updates the stream with the configured settings.
func (m *StreamManager) EnsureStream(ctx context.Context) (jetstream.Stream, error) {
	storage := jetstream.FileStorage
	if strings.ToLower(m.config.Storage) == "memory" {
		storage = jetstream.MemoryStorage
	}

	streamCfg := jetstream.StreamConfig{
		Name:        m.config.Name,
		Subjects:    m.config.Subjects,
		Storage:     storage,
		MaxAge:      m.config.MaxAge,
		MaxBytes:    m.config.MaxBytes,
		Replicas:    m.config.Replicas,
		Retention:   jetstream.LimitsPolicy,
		Discard:     jetstream.DiscardOld,
		AllowDirect: true,
	}

	// Try to get existing stream first
	_, err := m.js.Stream(ctx, m.config.Name)
	if err == nil {
		// Stream exists, update it
		m.logger.Info("updating existing stream", "name", m.config.Name)
		stream, err := m.js.UpdateStream(ctx, streamCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to update stream: %w", err)
		}
		m.logger.Info("stream updated", "name", m.config.Name)
		return stream, nil
	}

	// Stream doesn't exist, create it
	m.logger.Info("creating new stream", "name", m.config.Name, "subjects", m.config.Subjects)
	stream, err := m.js.CreateStream(ctx, streamCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	m.logger.Info("stream created",
		"name", m.config.Name,
		"storage", m.config.Storage,
		"max_age", m.config.MaxAge,
		"max_bytes", m.config.MaxBytes,
	)

	return stream, nil
}

// EnsureConsumers creates the default consumers for the stream.
func (m *StreamManager) EnsureConsumers(ctx context.Context, stream jetstream.Stream, configs []ConsumerConfig) error {
	for _, cfg := range configs {
		if err := m.ensureConsumer(ctx, stream, cfg); err != nil {
			return fmt.Errorf("failed to ensure consumer %s: %w", cfg.Name, err)
		}
	}
	return nil
}

func (m *StreamManager) ensureConsumer(ctx context.Context, stream jetstream.Stream, cfg ConsumerConfig) error {
	consumerCfg := jetstream.ConsumerConfig{
		Durable:       cfg.Name,
		FilterSubject: cfg.FilterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       cfg.AckWait,
		MaxAckPending: cfg.MaxAckPending,
		MaxDeliver:    cfg.MaxDeliver,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	}

	// Try to get existing consumer first
	_, err := stream.Consumer(ctx, cfg.Name)
	if err == nil {
		// Consumer exists, update it
		m.logger.Info("updating existing consumer", "name", cfg.Name)
		_, err = stream.UpdateConsumer(ctx, consumerCfg)
		if err != nil {
			return fmt.Errorf("failed to update consumer: %w", err)
		}
		return nil
	}

	// Consumer doesn't exist, create it
	m.logger.Info("creating new consumer",
		"name", cfg.Name,
		"filter", cfg.FilterSubject,
	)
	_, err = stream.CreateConsumer(ctx, consumerCfg)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	m.logger.Info("consumer created", "name", cfg.Name)
	return nil
}

// EnsureDLQStream creates or updates the dead-letter queue stream.
// The DLQ stream captures messages republished to "dlq.>" subjects after
// exceeding their MaxDeliver retry limit on the main stream.
func (m *StreamManager) EnsureDLQStream(ctx context.Context) (jetstream.Stream, error) {
	dlqCfg := jetstream.StreamConfig{
		Name:        m.config.DLQStreamName,
		Subjects:    []string{"dlq.>"},
		Storage:     jetstream.FileStorage,
		MaxAge:      m.config.DLQMaxAge,
		Retention:   jetstream.LimitsPolicy,
		Discard:     jetstream.DiscardOld,
		AllowDirect: true,
	}

	// Try to get existing stream first
	_, err := m.js.Stream(ctx, m.config.DLQStreamName)
	if err == nil {
		// Stream exists, update it
		m.logger.Info("updating existing DLQ stream", "name", m.config.DLQStreamName)
		stream, updateErr := m.js.UpdateStream(ctx, dlqCfg)
		if updateErr != nil {
			return nil, fmt.Errorf("failed to update DLQ stream: %w", updateErr)
		}
		m.logger.Info("DLQ stream updated", "name", m.config.DLQStreamName)
		return stream, nil
	}

	// Stream doesn't exist, create it
	m.logger.Info("creating new DLQ stream",
		"name", m.config.DLQStreamName,
		"subjects", dlqCfg.Subjects,
		"max_age", m.config.DLQMaxAge,
	)
	stream, err := m.js.CreateStream(ctx, dlqCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create DLQ stream: %w", err)
	}

	m.logger.Info("DLQ stream created",
		"name", m.config.DLQStreamName,
		"max_age", m.config.DLQMaxAge,
	)

	return stream, nil
}

// GetStreamInfo returns information about the stream.
func (m *StreamManager) GetStreamInfo(ctx context.Context) (*jetstream.StreamInfo, error) {
	stream, err := m.js.Stream(ctx, m.config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream: %w", err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	return info, nil
}
