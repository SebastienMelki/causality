package reaction

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	"github.com/SebastienMelki/causality/internal/observability"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// Consumer consumes events from NATS JetStream and processes them through the reaction engine.
type Consumer struct {
	js           jetstream.JetStream
	engine       *Engine
	anomaly      *AnomalyDetector
	logger       *slog.Logger
	metrics      *observability.Metrics
	config       ConsumerConfig
	consumerName string
	streamName   string

	shutdownTimeout time.Duration
	stopCh          chan struct{}
	doneCh          chan struct{}
}

// NewConsumer creates a new reaction consumer.
func NewConsumer(
	js jetstream.JetStream,
	engine *Engine,
	anomaly *AnomalyDetector,
	consumerName string,
	streamName string,
	cfg ConsumerConfig,
	shutdownTimeout time.Duration,
	logger *slog.Logger,
	metrics *observability.Metrics,
) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}

	if shutdownTimeout <= 0 {
		shutdownTimeout = 30 * time.Second
	}

	return &Consumer{
		js:              js,
		engine:          engine,
		anomaly:         anomaly,
		logger:          logger.With("component", "reaction-consumer"),
		metrics:         metrics,
		config:          cfg,
		consumerName:    consumerName,
		streamName:      streamName,
		shutdownTimeout: shutdownTimeout,
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}
}

// Start starts consuming events from NATS with a configurable worker pool.
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

	workerCount := c.config.WorkerCount
	if workerCount < 1 {
		workerCount = 1
	}

	c.logger.Info("starting reaction consumer",
		"consumer", c.consumerName,
		"stream", c.streamName,
		"workers", workerCount,
		"fetch_batch_size", c.config.FetchBatchSize,
	)

	// Start worker pool
	var wg sync.WaitGroup
	for i := range workerCount {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c.workerLoop(ctx, consumer, id)
		}(i)
	}

	// Close doneCh when all workers finish
	go func() {
		wg.Wait()
		close(c.doneCh)
	}()

	return nil
}

// workerLoop is the main loop for a single fetch worker. It pulls messages
// from the NATS consumer and processes them.
func (c *Consumer) workerLoop(ctx context.Context, consumer jetstream.Consumer, id int) {
	logger := c.logger.With("worker_id", id)
	logger.Debug("worker started")
	defer logger.Debug("worker stopped")

	fetchSize := c.config.FetchBatchSize
	if fetchSize < 1 {
		fetchSize = 100
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
			msgs, err := consumer.Fetch(fetchSize, jetstream.FetchMaxWait(5*time.Second))
			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					logger.Error("failed to fetch messages", "error", err)
					// Brief backoff before retrying on unexpected errors
					select {
					case <-time.After(time.Second):
					case <-ctx.Done():
						return
					case <-c.stopCh:
						return
					}
				}
				continue
			}

			for msg := range msgs.Messages() {
				c.processMessage(ctx, msg)
			}

			if err := msgs.Error(); err != nil {
				logger.Error("messages iteration error", "error", err)
			}
		}
	}
}

// processMessage deserializes a single NATS message and processes it through
// the rule engine and anomaly detector. Poison messages (unmarshal failures)
// are terminated immediately so they are not redelivered.
func (c *Consumer) processMessage(ctx context.Context, msg jetstream.Msg) {
	var event pb.EventEnvelope
	if err := proto.Unmarshal(msg.Data(), &event); err != nil {
		// Poison message: terminate to prevent infinite redelivery
		c.logger.Error("poison message: unmarshal failure, terminating",
			"error", err,
			"subject", msg.Subject(),
		)
		if termErr := msg.Term(); termErr != nil {
			c.logger.Error("failed to terminate poison message", "error", termErr)
		}
		return
	}

	c.logger.Debug("processing event",
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
		// Record rules evaluated metric
		if c.metrics != nil {
			c.metrics.RulesEvaluated.Add(ctx, 1)
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

	// Record message processed metric
	if c.metrics != nil {
		c.metrics.NATSMessagesProcessed.Add(ctx, 1)
	}

	// ACK successful processing
	if err := msg.Ack(); err != nil {
		c.logger.Error("failed to ACK message", "error", err)
	}
}

// Stop stops the consumer gracefully. It signals workers to stop and waits
// for them to finish up to the configured shutdown timeout.
func (c *Consumer) Stop(ctx context.Context) error {
	c.logger.Info("stopping reaction consumer")
	close(c.stopCh)

	// Create a shutdown context with the configured timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, c.shutdownTimeout)
	defer shutdownCancel()

	// Wait for workers to stop or timeout
	select {
	case <-c.doneCh:
		c.logger.Info("all workers stopped")
	case <-shutdownCtx.Done():
		c.logger.Warn("shutdown timeout waiting for workers",
			"timeout", c.shutdownTimeout,
		)
		return shutdownCtx.Err()
	}

	c.logger.Info("reaction consumer stopped")
	return nil
}
