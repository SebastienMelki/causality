package reaction

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// Consumer consumes events from NATS JetStream and processes them through the reaction engine.
type Consumer struct {
	js           jetstream.JetStream
	engine       *Engine
	anomaly      *AnomalyDetector
	logger       *slog.Logger
	consumerName string
	streamName   string

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewConsumer creates a new reaction consumer.
func NewConsumer(
	js jetstream.JetStream,
	engine *Engine,
	anomaly *AnomalyDetector,
	consumerName string,
	streamName string,
	logger *slog.Logger,
) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}

	return &Consumer{
		js:           js,
		engine:       engine,
		anomaly:      anomaly,
		logger:       logger.With("component", "reaction-consumer"),
		consumerName: consumerName,
		streamName:   streamName,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// Start starts consuming events from NATS.
func (c *Consumer) Start(ctx context.Context) error {
	// Get stream
	stream, err := c.js.Stream(ctx, c.streamName)
	if err != nil {
		return fmt.Errorf("failed to get stream: %w", err)
	}

	// Get consumer
	consumer, err := stream.Consumer(ctx, c.consumerName)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}

	c.logger.Info("starting reaction consumer",
		"consumer", c.consumerName,
		"stream", c.streamName,
	)

	// Start consuming
	go func() {
		defer close(c.doneCh)

		for {
			select {
			case <-ctx.Done():
				c.logger.Info("context cancelled, stopping consumer")
				return
			case <-c.stopCh:
				c.logger.Info("stop signal received, stopping consumer")
				return
			default:
				// Fetch messages
				msgs, err := consumer.Fetch(100, jetstream.FetchMaxWait(5*time.Second))
				if err != nil {
					if !errors.Is(err, context.DeadlineExceeded) {
						c.logger.Error("failed to fetch messages", "error", err)
					}
					continue
				}

				for msg := range msgs.Messages() {
					if err := c.processMessage(ctx, msg); err != nil {
						c.logger.Error("failed to process message", "error", err)
						// NAK to retry later
						if nakErr := msg.Nak(); nakErr != nil {
							c.logger.Error("failed to NAK message", "error", nakErr)
						}
						continue
					}

					// ACK successful processing
					if err := msg.Ack(); err != nil {
						c.logger.Error("failed to ACK message", "error", err)
					}
				}

				if err := msgs.Error(); err != nil {
					c.logger.Error("messages error", "error", err)
				}
			}
		}
	}()

	return nil
}

// processMessage processes a single NATS message.
func (c *Consumer) processMessage(ctx context.Context, msg jetstream.Msg) error {
	var event pb.EventEnvelope
	if err := proto.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	c.logger.Info("processing event",
		"event_id", event.Id,
		"app_id", event.AppId,
		"subject", msg.Subject(),
	)

	// Process through rule engine
	if c.engine != nil {
		if err := c.engine.ProcessEvent(ctx, &event); err != nil {
			c.logger.Error("rule engine error",
				"event_id", event.Id,
				"error", err,
			)
		}
	}

	// Process through anomaly detector
	if c.anomaly != nil {
		if err := c.anomaly.ProcessEvent(ctx, &event); err != nil {
			c.logger.Error("anomaly detector error",
				"event_id", event.Id,
				"error", err,
			)
		}
	}

	return nil
}

// Stop stops the consumer gracefully.
func (c *Consumer) Stop(ctx context.Context) error {
	c.logger.Info("stopping reaction consumer")
	close(c.stopCh)

	// Wait for consumer to stop or context to cancel
	select {
	case <-c.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
