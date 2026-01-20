package warehouse

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// Consumer consumes events from NATS JetStream and writes them to S3.
type Consumer struct {
	js           jetstream.JetStream
	config       Config
	s3Client     *S3Client
	parquet      *ParquetWriter
	logger       *slog.Logger
	consumerName string
	streamName   string

	mu       sync.Mutex
	batch    []*pb.EventEnvelope
	lastFlush time.Time
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewConsumer creates a new warehouse consumer.
func NewConsumer(
	js jetstream.JetStream,
	cfg Config,
	s3Client *S3Client,
	consumerName string,
	streamName string,
	logger *slog.Logger,
) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}

	return &Consumer{
		js:           js,
		config:       cfg,
		s3Client:     s3Client,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger.With("component", "warehouse-consumer"),
		consumerName: consumerName,
		streamName:   streamName,
		batch:        make([]*pb.EventEnvelope, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// Start starts consuming events from NATS.
func (c *Consumer) Start(ctx context.Context) error {
	// Get consumer
	stream, err := c.js.Stream(ctx, c.streamName)
	if err != nil {
		return fmt.Errorf("failed to get stream: %w", err)
	}

	consumer, err := stream.Consumer(ctx, c.consumerName)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}

	c.logger.Info("starting warehouse consumer",
		"consumer", c.consumerName,
		"stream", c.streamName,
	)

	// Start flush timer
	go c.flushTimer(ctx)

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

	c.mu.Lock()
	c.batch = append(c.batch, &event)
	shouldFlush := len(c.batch) >= c.config.Batch.MaxEvents
	c.mu.Unlock()

	if shouldFlush {
		if err := c.flush(ctx); err != nil {
			return fmt.Errorf("failed to flush batch: %w", err)
		}
	}

	return nil
}

// flushTimer periodically flushes the batch based on time interval.
func (c *Consumer) flushTimer(ctx context.Context) {
	ticker := time.NewTicker(c.config.Batch.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.Lock()
			batchLen := len(c.batch)
			timeSinceFlush := time.Since(c.lastFlush)
			c.mu.Unlock()

			if batchLen > 0 && timeSinceFlush >= c.config.Batch.FlushInterval {
				c.logger.Debug("time-based flush triggered",
					"batch_size", batchLen,
					"interval", timeSinceFlush,
				)
				if err := c.flush(ctx); err != nil {
					c.logger.Error("failed to flush batch on timer", "error", err)
				}
			}
		}
	}
}

// flush writes the current batch to S3.
// Errors from individual partitions are logged but do not stop the flush.
//
//nolint:unparam // Always returns nil by design; errors are logged per-partition.
func (c *Consumer) flush(ctx context.Context) error {
	c.mu.Lock()
	if len(c.batch) == 0 {
		c.mu.Unlock()
		return nil
	}

	// Swap batch
	events := c.batch
	c.batch = make([]*pb.EventEnvelope, 0, c.config.Batch.MaxEvents)
	c.lastFlush = time.Now()
	c.mu.Unlock()

	c.logger.Info("flushing batch", "count", len(events))

	// Group events by app_id and time partition
	partitions := c.groupByPartition(events)

	// Write each partition
	for key, partitionEvents := range partitions {
		if err := c.writePartition(ctx, key, partitionEvents); err != nil {
			c.logger.Error("failed to write partition",
				"partition", key,
				"error", err,
			)
			// Continue with other partitions
		}
	}

	c.logger.Info("batch flushed",
		"count", len(events),
		"partitions", len(partitions),
	)

	return nil
}

// partitionKey represents a unique partition for events.
type partitionKey struct {
	AppID string
	Year  int
	Month int
	Day   int
	Hour  int
}

// groupByPartition groups events by their partition key.
func (c *Consumer) groupByPartition(events []*pb.EventEnvelope) map[partitionKey][]*pb.EventEnvelope {
	partitions := make(map[partitionKey][]*pb.EventEnvelope)

	for _, event := range events {
		t := time.UnixMilli(event.GetTimestampMs()).UTC()
		key := partitionKey{
			AppID: event.GetAppId(),
			Year:  t.Year(),
			Month: int(t.Month()),
			Day:   t.Day(),
			Hour:  t.Hour(),
		}

		partitions[key] = append(partitions[key], event)
	}

	return partitions
}

// writePartition writes a partition of events to S3.
func (c *Consumer) writePartition(ctx context.Context, key partitionKey, events []*pb.EventEnvelope) error {
	// Convert to Parquet rows
	rows := make([]EventRow, len(events))
	for i, event := range events {
		rows[i] = EventRowFromProto(event, key.Year, key.Month, key.Day, key.Hour)
	}

	// Write to Parquet
	data, err := c.parquet.Write(rows)
	if err != nil {
		return fmt.Errorf("failed to write parquet: %w", err)
	}

	// Upload to S3
	s3Key := c.s3Client.GenerateKey(key.AppID, key.Year, key.Month, key.Day, key.Hour)
	if err := c.s3Client.Upload(ctx, s3Key, data); err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	c.logger.Debug("partition written",
		"key", s3Key,
		"events", len(events),
		"size_bytes", len(data),
	)

	return nil
}

// Stop stops the consumer gracefully.
func (c *Consumer) Stop(ctx context.Context) error {
	c.logger.Info("stopping warehouse consumer")
	close(c.stopCh)

	// Wait for consumer to stop or context to cancel
	select {
	case <-c.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Final flush
	if err := c.flush(ctx); err != nil {
		c.logger.Error("failed final flush", "error", err)
	}

	return nil
}
