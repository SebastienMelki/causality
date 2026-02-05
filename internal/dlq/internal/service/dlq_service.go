// Package service provides the dead-letter queue service that listens for NATS
// JetStream MaxDeliver advisory events and republishes failed messages to the
// DLQ stream for later investigation.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/SebastienMelki/causality/internal/observability"
)

// advisorySubject builds the NATS advisory subject for MaxDeliver exceeded events.
// Format: $JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.<stream>.<consumer>
func advisorySubject(streamName, consumerName string) string {
	return fmt.Sprintf("$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.%s.%s", streamName, consumerName)
}

// maxDeliverAdvisory represents the JSON payload from a NATS JetStream
// MaxDeliver advisory event. This is emitted by the server when a message
// has been delivered more than MaxDeliver times without acknowledgment.
type maxDeliverAdvisory struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Stream   string `json:"stream"`
	Consumer string `json:"consumer"`
	StreamSeq uint64 `json:"stream_seq"`
	Deliveries uint64 `json:"deliveries"`
}

// DLQService listens for NATS JetStream MaxDeliver advisory events and
// republishes the original failed messages to a dead-letter queue stream.
type DLQService struct {
	js            jetstream.JetStream
	nc            *nats.Conn
	metrics       *observability.Metrics
	logger        *slog.Logger
	streamName    string
	consumerNames []string
	subs          []*nats.Subscription
}

// NewDLQService creates a new DLQ service.
//
// Parameters:
//   - js: JetStream context for publishing to DLQ and fetching original messages
//   - nc: raw NATS connection for subscribing to advisory subjects (core NATS)
//   - streamName: the main stream name (e.g., "CAUSALITY_EVENTS")
//   - consumerNames: consumer names to monitor for MaxDeliver advisories
//   - metrics: observability metrics (may be nil)
//   - logger: structured logger
func NewDLQService(
	js jetstream.JetStream,
	nc *nats.Conn,
	streamName string,
	consumerNames []string,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *DLQService {
	if logger == nil {
		logger = slog.Default()
	}
	return &DLQService{
		js:            js,
		nc:            nc,
		metrics:       metrics,
		logger:        logger.With("component", "dlq-service"),
		streamName:    streamName,
		consumerNames: consumerNames,
	}
}

// Start subscribes to MaxDeliver advisory subjects for each monitored consumer.
// It blocks until the context is cancelled or Stop is called.
func (s *DLQService) Start(ctx context.Context) error {
	for _, consumerName := range s.consumerNames {
		subject := advisorySubject(s.streamName, consumerName)
		s.logger.Info("subscribing to MaxDeliver advisory",
			"subject", subject,
			"consumer", consumerName,
		)

		sub, err := s.nc.Subscribe(subject, s.handleAdvisory(ctx))
		if err != nil {
			// Clean up any subscriptions we already created
			s.Stop()
			return fmt.Errorf("failed to subscribe to advisory %s: %w", subject, err)
		}
		s.subs = append(s.subs, sub)
	}

	s.logger.Info("DLQ service started",
		"stream", s.streamName,
		"consumers", s.consumerNames,
	)

	return nil
}

// handleAdvisory returns a NATS message handler that processes MaxDeliver advisories.
func (s *DLQService) handleAdvisory(ctx context.Context) nats.MsgHandler {
	return func(msg *nats.Msg) {
		var advisory maxDeliverAdvisory
		if err := json.Unmarshal(msg.Data, &advisory); err != nil {
			s.logger.Error("failed to parse MaxDeliver advisory",
				"error", err,
				"data", string(msg.Data),
			)
			return
		}

		s.logger.Warn("MaxDeliver exceeded",
			"stream", advisory.Stream,
			"consumer", advisory.Consumer,
			"stream_seq", advisory.StreamSeq,
			"deliveries", advisory.Deliveries,
		)

		// Fetch the original message from the stream by sequence number
		stream, err := s.js.Stream(ctx, s.streamName)
		if err != nil {
			s.logger.Error("failed to get stream for DLQ message fetch",
				"stream", s.streamName,
				"error", err,
			)
			return
		}

		rawMsg, err := stream.GetMsg(ctx, advisory.StreamSeq)
		if err != nil {
			s.logger.Error("failed to fetch original message for DLQ",
				"stream", s.streamName,
				"stream_seq", advisory.StreamSeq,
				"error", err,
			)
			return
		}

		// Republish to DLQ stream with original subject preserved
		dlqSubject := "dlq." + rawMsg.Subject

		// Build headers preserving original metadata
		headers := nats.Header{}
		if rawMsg.Header != nil {
			for k, v := range rawMsg.Header {
				headers[k] = v
			}
		}
		headers.Set("X-DLQ-Original-Subject", rawMsg.Subject)
		headers.Set("X-DLQ-Original-Stream", advisory.Stream)
		headers.Set("X-DLQ-Original-Consumer", advisory.Consumer)
		headers.Set("X-DLQ-Original-Sequence", fmt.Sprintf("%d", advisory.StreamSeq))
		headers.Set("X-DLQ-Deliveries", fmt.Sprintf("%d", advisory.Deliveries))

		pubMsg := &nats.Msg{
			Subject: dlqSubject,
			Data:    rawMsg.Data,
			Header:  headers,
		}

		if _, err := s.js.PublishMsg(ctx, pubMsg); err != nil {
			s.logger.Error("failed to publish message to DLQ",
				"dlq_subject", dlqSubject,
				"stream_seq", advisory.StreamSeq,
				"error", err,
			)
			return
		}

		// Increment DLQ depth metric
		if s.metrics != nil {
			s.metrics.DLQDepth.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("consumer", advisory.Consumer),
					attribute.String("original_subject", rawMsg.Subject),
				),
			)
		}

		s.logger.Warn("message moved to DLQ",
			"dlq_subject", dlqSubject,
			"stream_seq", advisory.StreamSeq,
			"consumer", advisory.Consumer,
			"deliveries", advisory.Deliveries,
		)
	}
}

// Stop unsubscribes from all advisory subscriptions.
func (s *DLQService) Stop() {
	for _, sub := range s.subs {
		if sub.IsValid() {
			if err := sub.Unsubscribe(); err != nil {
				s.logger.Error("failed to unsubscribe from advisory",
					"subject", sub.Subject,
					"error", err,
				)
			}
		}
	}
	s.subs = nil
	s.logger.Info("DLQ service stopped")
}

// GetDLQCount returns the number of messages currently in the DLQ stream.
func (s *DLQService) GetDLQCount(ctx context.Context, dlqStreamName string) (int64, error) {
	stream, err := s.js.Stream(ctx, dlqStreamName)
	if err != nil {
		return 0, fmt.Errorf("failed to get DLQ stream: %w", err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get DLQ stream info: %w", err)
	}

	return int64(info.State.Msgs), nil
}
