package nats

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	"github.com/SebastienMelki/causality/internal/events"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// Publisher handles publishing events to NATS JetStream.
type Publisher struct {
	js         jetstream.JetStream
	streamName string
	logger     *slog.Logger
}

// NewPublisher creates a new event publisher.
func NewPublisher(js jetstream.JetStream, streamName string, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{
		js:         js,
		streamName: streamName,
		logger:     logger.With("component", "publisher"),
	}
}

// PublishEvent publishes a single event to the appropriate NATS subject.
func (p *Publisher) PublishEvent(ctx context.Context, event *pb.EventEnvelope) error {
	subject := p.deriveSubject(event)

	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	ack, err := p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("event published",
		"event_id", event.GetId(),
		"subject", subject,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// PublishEventBatch publishes multiple events to NATS.
// Returns the number of successfully published events and any error.
func (p *Publisher) PublishEventBatch(ctx context.Context, events []*pb.EventEnvelope) (int, error) {
	published := 0

	for _, event := range events {
		if err := p.PublishEvent(ctx, event); err != nil {
			p.logger.Error("failed to publish event in batch",
				"event_id", event.GetId(),
				"error", err,
			)
			// Continue with remaining events
			continue
		}
		published++
	}

	if published < len(events) {
		return published, fmt.Errorf("%w: %d of %d failed", ErrPartialPublish, len(events)-published, len(events))
	}

	return published, nil
}

// PublishAsync publishes an event asynchronously and returns a future for the ack.
func (p *Publisher) PublishAsync(_ context.Context, event *pb.EventEnvelope) (jetstream.PubAckFuture, error) {
	subject := p.deriveSubject(event)

	data, err := proto.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	future, err := p.js.PublishAsync(subject, data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish event async: %w", err)
	}

	return future, nil
}

// deriveSubject derives the NATS subject from the event envelope.
// Format: {kind}.{app_id}.{category}.{type}.
func (p *Publisher) deriveSubject(event *pb.EventEnvelope) string {
	category, eventType := events.GetCategoryAndType(event)

	// Sanitize app_id for subject (replace dots with underscores)
	appID := strings.ReplaceAll(event.GetAppId(), ".", "_")

	// Sanitize custom event names for subject.
	if category == events.CategoryCustom {
		eventType = events.SanitizeSubjectName(eventType)
	}

	return fmt.Sprintf("events.%s.%s.%s", appID, category, eventType)
}

// DeriveSubjectForTest exposes subject derivation for testing.
func (p *Publisher) DeriveSubjectForTest(event *pb.EventEnvelope) string {
	return p.deriveSubject(event)
}
