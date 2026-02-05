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

	"github.com/SebastienMelki/causality/internal/observability"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// trackedEvent pairs a deserialized event with its original NATS message so
// that ACK/NAK can be deferred until after the S3 write succeeds or fails.
type trackedEvent struct {
	event *pb.EventEnvelope
	msg   jetstream.Msg
}

// Consumer consumes events from NATS JetStream and writes them to S3.
type Consumer struct {
	js           jetstream.JetStream
	config       Config
	s3Client     *S3Client
	parquet      *ParquetWriter
	logger       *slog.Logger
	metrics      *observability.Metrics
	consumerName string
	streamName   string

	mu        sync.Mutex
	batch     []trackedEvent
	lastFlush time.Time
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewConsumer creates a new warehouse consumer.
func NewConsumer(
	js jetstream.JetStream,
	cfg Config,
	s3Client *S3Client,
	consumerName string,
	streamName string,
	logger *slog.Logger,
	metrics *observability.Metrics,
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
		metrics:      metrics,
		consumerName: consumerName,
		streamName:   streamName,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// Start starts consuming events from NATS with a configurable worker pool.
func (c *Consumer) Start(ctx context.Context) error {
	// Get stream and consumer
	stream, err := c.js.Stream(ctx, c.streamName)
	if err != nil {
		return fmt.Errorf("failed to get stream: %w", err)
	}

	consumer, err := stream.Consumer(ctx, c.consumerName)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}

	workerCount := c.config.Batch.WorkerCount
	if workerCount < 1 {
		workerCount = 1
	}

	c.logger.Info("starting warehouse consumer",
		"consumer", c.consumerName,
		"stream", c.streamName,
		"workers", workerCount,
		"fetch_batch_size", c.config.Batch.FetchBatchSize,
	)

	// Start flush timer
	go c.flushTimer(ctx)

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
// from the NATS consumer and processes them. ACK/NAK is deferred to flush.
func (c *Consumer) workerLoop(ctx context.Context, consumer jetstream.Consumer, id int) {
	logger := c.logger.With("worker_id", id)
	logger.Debug("worker started")
	defer logger.Debug("worker stopped")

	fetchSize := c.config.Batch.FetchBatchSize
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

// processMessage deserializes a single NATS message and adds it to the batch.
// Poison messages (unmarshal failures) are terminated immediately so they are
// not redelivered. Valid messages are tracked and ACKed/NAKed later in flush.
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

	c.mu.Lock()
	c.batch = append(c.batch, trackedEvent{event: &event, msg: msg})
	shouldFlush := len(c.batch) >= c.config.Batch.MaxEvents
	c.mu.Unlock()

	if shouldFlush {
		if err := c.flush(ctx); err != nil {
			c.logger.Error("failed to flush batch", "error", err)
		}
	}
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
// For each partition, messages are ACKed only after a successful S3 write.
// On write failure, messages are NAKed so NATS redelivers them.
func (c *Consumer) flush(ctx context.Context) error {
	flushStart := time.Now()

	c.mu.Lock()
	if len(c.batch) == 0 {
		c.mu.Unlock()
		return nil
	}

	// Swap batch
	tracked := c.batch
	c.batch = make([]trackedEvent, 0, c.config.Batch.MaxEvents)
	c.lastFlush = time.Now()
	c.mu.Unlock()

	batchSize := len(tracked)
	c.logger.Info("flushing batch", "count", batchSize)

	// Record batch size metric
	if c.metrics != nil {
		c.metrics.NATSBatchSize.Record(ctx, int64(batchSize))
	}

	// Group events by partition
	partitions := c.groupByPartition(tracked)

	// Write each partition
	for key, partitionTracked := range partitions {
		if err := c.writePartition(ctx, key, partitionTracked); err != nil {
			c.logger.Error("failed to write partition, NAKing messages for redelivery",
				"partition", key,
				"events", len(partitionTracked),
				"error", err,
			)
			// NAK all messages in the failed partition so NATS redelivers them
			for _, t := range partitionTracked {
				if nakErr := t.msg.Nak(); nakErr != nil {
					c.logger.Error("failed to NAK message", "error", nakErr)
				}
			}
			continue
		}

		// Partition written successfully: ACK all messages
		for _, t := range partitionTracked {
			if ackErr := t.msg.Ack(); ackErr != nil {
				c.logger.Error("failed to ACK message after successful write", "error", ackErr)
			}
		}

		// Record S3 write metric
		if c.metrics != nil {
			c.metrics.S3FilesWritten.Add(ctx, 1)
			c.metrics.NATSMessagesProcessed.Add(ctx, int64(len(partitionTracked)))
		}
	}

	// Record flush latency
	if c.metrics != nil {
		flushDuration := float64(time.Since(flushStart).Milliseconds())
		c.metrics.NATSFlushLatency.Record(ctx, flushDuration)
	}

	c.logger.Info("batch flushed",
		"count", batchSize,
		"partitions", len(partitions),
		"duration_ms", time.Since(flushStart).Milliseconds(),
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

// groupByPartition groups tracked events by their partition key.
func (c *Consumer) groupByPartition(tracked []trackedEvent) map[partitionKey][]trackedEvent {
	partitions := make(map[partitionKey][]trackedEvent)

	for _, t := range tracked {
		ts := time.UnixMilli(t.event.GetTimestampMs()).UTC()
		key := partitionKey{
			AppID: t.event.GetAppId(),
			Year:  ts.Year(),
			Month: int(ts.Month()),
			Day:   ts.Day(),
			Hour:  ts.Hour(),
		}

		partitions[key] = append(partitions[key], t)
	}

	return partitions
}

// writePartition writes a partition of tracked events to S3.
func (c *Consumer) writePartition(ctx context.Context, key partitionKey, tracked []trackedEvent) error {
	// Extract events from tracked for Parquet conversion
	rows := make([]EventRow, len(tracked))
	for i, t := range tracked {
		rows[i] = EventRowFromProto(t.event, key.Year, key.Month, key.Day, key.Hour)
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

	// Record file size metric
	if c.metrics != nil {
		c.metrics.S3FileSize.Record(ctx, int64(len(data)))
	}

	c.logger.Debug("partition written",
		"key", s3Key,
		"events", len(tracked),
		"size_bytes", len(data),
	)

	return nil
}

// Stop stops the consumer gracefully. It signals workers to stop, waits for
// them to finish (up to ShutdownTimeout), and performs a final flush of any
// remaining messages in the batch.
func (c *Consumer) Stop(ctx context.Context) error {
	c.logger.Info("stopping warehouse consumer")
	close(c.stopCh)

	// Create a shutdown context with the configured timeout
	shutdownTimeout := c.config.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 60 * time.Second
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, shutdownTimeout)
	defer shutdownCancel()

	// Wait for workers to stop or timeout
	select {
	case <-c.doneCh:
		c.logger.Info("all workers stopped")
	case <-shutdownCtx.Done():
		c.logger.Warn("shutdown timeout waiting for workers, proceeding with final flush",
			"timeout", shutdownTimeout,
		)
	}

	// Final flush of any remaining messages
	c.logger.Info("performing final flush")
	if err := c.flush(shutdownCtx); err != nil {
		c.logger.Error("failed final flush, messages may be redelivered by NATS", "error", err)
		return fmt.Errorf("final flush failed: %w", err)
	}

	c.logger.Info("warehouse consumer stopped")
	return nil
}
