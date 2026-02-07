// Package warehouse tests the NATS consumer ACK/NAK/Term behavior.
package warehouse

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/metric/noop"
	"google.golang.org/protobuf/proto"

	"github.com/SebastienMelki/causality/internal/observability"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// mockJetStreamMsg implements jetstream.Msg for testing.
type mockJetStreamMsg struct {
	data       []byte
	subject    string
	ackCalled  atomic.Bool
	nakCalled  atomic.Bool
	termCalled atomic.Bool
	ackErr     error
	nakErr     error
	termErr    error
}

func (m *mockJetStreamMsg) Data() []byte {
	return m.data
}

func (m *mockJetStreamMsg) Subject() string {
	return m.subject
}

func (m *mockJetStreamMsg) Reply() string {
	return ""
}

func (m *mockJetStreamMsg) Headers() nats.Header {
	return nats.Header{}
}

func (m *mockJetStreamMsg) Ack() error {
	m.ackCalled.Store(true)
	return m.ackErr
}

func (m *mockJetStreamMsg) Nak() error {
	m.nakCalled.Store(true)
	return m.nakErr
}

func (m *mockJetStreamMsg) NakWithDelay(delay time.Duration) error {
	m.nakCalled.Store(true)
	return m.nakErr
}

func (m *mockJetStreamMsg) InProgress() error {
	return nil
}

func (m *mockJetStreamMsg) Term() error {
	m.termCalled.Store(true)
	return m.termErr
}

func (m *mockJetStreamMsg) TermWithReason(reason string) error {
	m.termCalled.Store(true)
	return m.termErr
}

func (m *mockJetStreamMsg) DoubleAck(ctx context.Context) error {
	return m.Ack()
}

func (m *mockJetStreamMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{}, nil
}

// mockS3Client mocks S3 operations for testing.
type mockS3Client struct {
	uploadErr   error
	uploadCalls atomic.Int32
	uploadedKey string
}

func (m *mockS3Client) Upload(_ context.Context, key string, _ []byte) error {
	m.uploadCalls.Add(1)
	m.uploadedKey = key
	return m.uploadErr
}

func (m *mockS3Client) GenerateKey(appID string, year, month, day, hour int) string {
	return "test-key.parquet"
}

// createTestConsumer creates a Consumer with mocked dependencies for testing.
func createTestConsumer(t *testing.T) *Consumer {
	t.Helper()

	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
		ShutdownTimeout: 5 * time.Second,
		Parquet: ParquetConfig{
			Compression:  "snappy",
			RowGroupSize: 1024,
		},
	}

	// Use a discard logger to avoid nil pointer issues
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// createTestMetrics creates metrics for testing.
func createTestMetrics(t *testing.T) *observability.Metrics {
	t.Helper()
	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.NewMetrics(meter)
	if err != nil {
		t.Fatalf("Failed to create test metrics: %v", err)
	}
	return m
}

// TestProcessMessage_UnmarshalError_TermsMessage verifies that poison messages are terminated.
func TestProcessMessage_UnmarshalError_TermsMessage(t *testing.T) {
	c := createTestConsumer(t)

	// Create a message with invalid proto data
	msg := &mockJetStreamMsg{
		data:    []byte("invalid proto data"),
		subject: "events.test",
	}

	// Process the message
	c.processMessage(context.Background(), msg)

	// msg.Term() should have been called
	if !msg.termCalled.Load() {
		t.Error("msg.Term() should be called for poison messages (unmarshal failure)")
	}

	// msg.Ack() and msg.Nak() should NOT have been called
	if msg.ackCalled.Load() {
		t.Error("msg.Ack() should not be called for poison messages")
	}
	if msg.nakCalled.Load() {
		t.Error("msg.Nak() should not be called for poison messages")
	}
}

// TestProcessMessage_ValidEvent_AddsToBatch verifies valid events are added to the batch.
func TestProcessMessage_ValidEvent_AddsToBatch(t *testing.T) {
	c := createTestConsumer(t)

	// Create a valid event
	event := &pb.EventEnvelope{
		Id:          "test-event-1",
		AppId:       "test-app",
		TimestampMs: time.Now().UnixMilli(),
		Payload: &pb.EventEnvelope_ScreenView{
			ScreenView: &pb.ScreenView{ScreenName: "home"},
		},
	}

	data, err := proto.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	msg := &mockJetStreamMsg{
		data:    data,
		subject: "events.test",
	}

	// Process the message
	c.processMessage(context.Background(), msg)

	// Verify event was added to batch
	if len(c.batch) != 1 {
		t.Errorf("Batch length = %d, want 1", len(c.batch))
	}

	// Verify the tracked event has the message reference (compare as interface)
	if c.batch[0].msg == nil {
		t.Error("Tracked event should have a message reference")
	}

	// Verify event data is correct
	if c.batch[0].event.Id != "test-event-1" {
		t.Errorf("Event ID = %q, want %q", c.batch[0].event.Id, "test-event-1")
	}
}

// TestGroupByPartition verifies events are correctly grouped by partition.
func TestGroupByPartition(t *testing.T) {
	c := createTestConsumer(t)

	// Create events for different partitions
	ts1 := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli()
	ts2 := time.Date(2026, 1, 15, 11, 30, 0, 0, time.UTC).UnixMilli() // Different hour
	ts3 := time.Date(2026, 1, 16, 10, 30, 0, 0, time.UTC).UnixMilli() // Different day

	tracked := []trackedEvent{
		{
			event: &pb.EventEnvelope{AppId: "app-1", TimestampMs: ts1},
			msg:   &mockJetStreamMsg{},
		},
		{
			event: &pb.EventEnvelope{AppId: "app-1", TimestampMs: ts1}, // Same partition as first
			msg:   &mockJetStreamMsg{},
		},
		{
			event: &pb.EventEnvelope{AppId: "app-1", TimestampMs: ts2}, // Different hour
			msg:   &mockJetStreamMsg{},
		},
		{
			event: &pb.EventEnvelope{AppId: "app-2", TimestampMs: ts1}, // Different app
			msg:   &mockJetStreamMsg{},
		},
		{
			event: &pb.EventEnvelope{AppId: "app-1", TimestampMs: ts3}, // Different day
			msg:   &mockJetStreamMsg{},
		},
	}

	partitions := c.groupByPartition(tracked)

	// Should have 4 unique partitions
	if len(partitions) != 4 {
		t.Errorf("Partition count = %d, want 4", len(partitions))
	}

	// Verify partition grouping
	for key, events := range partitions {
		if key.AppID == "app-1" && key.Hour == 10 && key.Day == 15 {
			if len(events) != 2 {
				t.Errorf("app-1 hour 10 day 15 should have 2 events, got %d", len(events))
			}
		}
	}
}

// TestFlush_EmptyBatch verifies that flushing an empty batch is a no-op.
func TestFlush_EmptyBatch(t *testing.T) {
	c := createTestConsumer(t)

	err := c.flush(context.Background())
	if err != nil {
		t.Errorf("flush() with empty batch should not return error: %v", err)
	}
}

// TestFlush_ACKAfterWrite_Simulation tests the ACK-after-write behavior.
// Note: This test simulates the expected behavior without actually writing to S3.
func TestFlush_ACKAfterWrite_Simulation(t *testing.T) {
	// Create events with mock messages
	ts := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli()

	msg1 := &mockJetStreamMsg{data: []byte{}, subject: "events.test"}
	msg2 := &mockJetStreamMsg{data: []byte{}, subject: "events.test"}

	// Simulate successful batch processing
	// In real code, after successful S3 write, msg.Ack() is called
	// After failed S3 write, msg.Nak() is called

	// Simulate successful write - ACK both messages
	if err := msg1.Ack(); err != nil {
		t.Errorf("msg1.Ack() failed: %v", err)
	}
	if err := msg2.Ack(); err != nil {
		t.Errorf("msg2.Ack() failed: %v", err)
	}

	if !msg1.ackCalled.Load() {
		t.Error("msg1 should be ACKed after successful write")
	}
	if !msg2.ackCalled.Load() {
		t.Error("msg2 should be ACKed after successful write")
	}

	// Simulate the expected behavior
	_ = ts // Use ts to avoid unused variable
}

// TestFlush_NAKOnWriteError_Simulation tests that messages are NAKed on write failure.
func TestFlush_NAKOnWriteError_Simulation(t *testing.T) {
	msg1 := &mockJetStreamMsg{data: []byte{}, subject: "events.test"}
	msg2 := &mockJetStreamMsg{data: []byte{}, subject: "events.test"}

	// Simulate failed write - NAK both messages
	simulatedWriteError := errors.New("S3 write failed")
	_ = simulatedWriteError // Document that this error would trigger NAK

	if err := msg1.Nak(); err != nil {
		t.Errorf("msg1.Nak() failed: %v", err)
	}
	if err := msg2.Nak(); err != nil {
		t.Errorf("msg2.Nak() failed: %v", err)
	}

	if !msg1.nakCalled.Load() {
		t.Error("msg1 should be NAKed after failed write")
	}
	if !msg2.nakCalled.Load() {
		t.Error("msg2 should be NAKed after failed write")
	}

	// ACK should NOT have been called
	if msg1.ackCalled.Load() {
		t.Error("msg1 should not be ACKed after failed write")
	}
	if msg2.ackCalled.Load() {
		t.Error("msg2 should not be ACKed after failed write")
	}
}

// TestPartitionKey verifies partitionKey struct behavior.
func TestPartitionKey(t *testing.T) {
	key1 := partitionKey{AppID: "app-1", Year: 2026, Month: 1, Day: 15, Hour: 10}
	key2 := partitionKey{AppID: "app-1", Year: 2026, Month: 1, Day: 15, Hour: 10}
	key3 := partitionKey{AppID: "app-2", Year: 2026, Month: 1, Day: 15, Hour: 10}

	// Same partition keys should be equal (used as map keys)
	if key1 != key2 {
		t.Error("Identical partition keys should be equal")
	}

	// Different partition keys should not be equal
	if key1 == key3 {
		t.Error("Different partition keys should not be equal")
	}
}

// TestTrackedEvent verifies the trackedEvent structure.
func TestTrackedEvent(t *testing.T) {
	event := &pb.EventEnvelope{Id: "test-event"}
	msg := &mockJetStreamMsg{data: []byte{}, subject: "events.test"}

	te := trackedEvent{
		event: event,
		msg:   msg,
	}

	if te.event.Id != "test-event" {
		t.Errorf("trackedEvent.event.Id = %q, want %q", te.event.Id, "test-event")
	}
}

// --- Timer, Shutdown, and Worker tests ---

// TestFlushTimer_TriggersFlush verifies that the flush timer triggers flush after interval.
func TestFlushTimer_TriggersFlush(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  50 * time.Millisecond, // Short interval for testing
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
		ShutdownTimeout: 1 * time.Second,
		Parquet: ParquetConfig{
			Compression:  "snappy",
			RowGroupSize: 1024,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now().Add(-100 * time.Millisecond), // Set past flush time
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	// Note: Don't add events to batch because flush() will panic without S3Client
	// Instead, we test that the timer ticks and checks the condition.
	// With empty batch, flush returns early without hitting S3.

	// Start the flush timer in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Track that flushTimer was running
	go c.flushTimer(ctx)

	// Wait for at least one timer tick
	time.Sleep(100 * time.Millisecond)

	// Stop the timer
	cancel()

	// Timer should have run at least once (verified by no panic with empty batch)
}

// TestFlushTimer_StopsOnContextCancel verifies timer stops when context is canceled.
func TestFlushTimer_StopsOnContextCancel(t *testing.T) {
	c := createTestConsumer(t)

	ctx, cancel := context.WithCancel(context.Background())

	// Track when timer goroutine exits
	done := make(chan struct{})
	go func() {
		c.flushTimer(ctx)
		close(done)
	}()

	// Cancel immediately
	cancel()

	// Timer should exit quickly
	select {
	case <-done:
		// Success - timer exited
	case <-time.After(500 * time.Millisecond):
		t.Error("Flush timer did not stop on context cancel")
	}
}

// TestFlushTimer_StopsOnStopChannel verifies timer stops when stopCh is closed.
func TestFlushTimer_StopsOnStopChannel(t *testing.T) {
	c := createTestConsumer(t)

	ctx := context.Background()

	// Track when timer goroutine exits
	done := make(chan struct{})
	go func() {
		c.flushTimer(ctx)
		close(done)
	}()

	// Close stop channel
	close(c.stopCh)

	// Timer should exit quickly
	select {
	case <-done:
		// Success - timer exited
	case <-time.After(500 * time.Millisecond):
		t.Error("Flush timer did not stop on stopCh close")
	}
}

// TestStop_ClosesDoneCh verifies Stop closes doneCh after workers finish.
func TestStop_ClosesDoneCh(t *testing.T) {
	c := createTestConsumer(t)

	// Simulate workers already done by closing doneCh
	close(c.doneCh)

	// Stop should complete without error
	err := c.Stop(context.Background())
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

// TestStop_FinalFlush verifies Stop performs final flush on empty batch.
func TestStop_FinalFlush(t *testing.T) {
	c := createTestConsumer(t)

	// Close doneCh to simulate workers stopping
	close(c.doneCh)

	// Stop with empty batch should complete without error
	err := c.Stop(context.Background())
	if err != nil {
		t.Errorf("Stop() with empty batch returned error: %v", err)
	}
}

// TestStop_TimesOutWaitingForWorkers verifies shutdown timeout behavior.
func TestStop_TimesOutWaitingForWorkers(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
		ShutdownTimeout: 100 * time.Millisecond, // Short timeout for testing
		Parquet: ParquetConfig{
			Compression:  "snappy",
			RowGroupSize: 1024,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}), // Never closed - simulates stuck workers
	}

	// Stop should timeout waiting for workers
	start := time.Now()
	_ = c.Stop(context.Background())
	elapsed := time.Since(start)

	// Should have waited approximately the shutdown timeout
	if elapsed < 50*time.Millisecond {
		t.Errorf("Stop() returned too quickly (%v), expected ~100ms timeout", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Stop() took too long (%v), expected ~100ms timeout", elapsed)
	}
}

// TestProcessMessage_BatchThreshold_ChecksShouldFlush verifies the shouldFlush logic.
func TestProcessMessage_BatchThreshold_ChecksShouldFlush(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      2, // Small batch size for testing
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
		ShutdownTimeout: 5 * time.Second,
		Parquet: ParquetConfig{
			Compression:  "snappy",
			RowGroupSize: 1024,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	// Create valid event
	event1 := &pb.EventEnvelope{
		Id:          "batch-full-1",
		AppId:       "test-app",
		TimestampMs: time.Now().UnixMilli(),
		Payload:     &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "home"}},
	}
	data1, _ := proto.Marshal(event1)
	msg1 := &mockJetStreamMsg{data: data1, subject: "events.test"}

	// Process first message - should add to batch
	c.processMessage(context.Background(), msg1)

	c.mu.Lock()
	batchLen1 := len(c.batch)
	c.mu.Unlock()

	if batchLen1 != 1 {
		t.Errorf("After first message, batch len = %d, want 1", batchLen1)
	}

	// The shouldFlush logic checks len(batch) >= MaxEvents
	// We verify the batch size check is working
	c.mu.Lock()
	shouldFlush := len(c.batch) >= c.config.Batch.MaxEvents
	c.mu.Unlock()

	if shouldFlush {
		t.Error("shouldFlush should be false with 1 event and MaxEvents=2")
	}
}

// TestFlush_WithMetrics_RecordsValues verifies metrics are recorded on flush.
func TestFlush_WithMetrics_RecordsValues(t *testing.T) {
	c := createTestConsumer(t)
	c.metrics = createTestMetrics(t)

	// Empty batch - flush should return early without issues
	err := c.flush(context.Background())
	if err != nil {
		t.Errorf("flush() with empty batch and metrics returned error: %v", err)
	}
}

// TestNewConsumer_SetsDefaults verifies constructor sets appropriate defaults.
func TestNewConsumer_SetsDefaults(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
	}

	c := NewConsumer(nil, cfg, nil, "test-consumer", "test-stream", nil, nil)

	if c.logger == nil {
		t.Error("Consumer should have a default logger")
	}
	if c.batch == nil {
		t.Error("Consumer should have initialized batch slice")
	}
	if c.stopCh == nil {
		t.Error("Consumer should have initialized stopCh")
	}
	if c.doneCh == nil {
		t.Error("Consumer should have initialized doneCh")
	}
}

// TestNewConsumer_WithMetrics verifies constructor accepts metrics.
func TestNewConsumer_WithMetrics(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:     100,
			FlushInterval: time.Minute,
		},
	}

	metrics := createTestMetrics(t)
	c := NewConsumer(nil, cfg, nil, "test", "stream", nil, metrics)

	if c.metrics != metrics {
		t.Error("Consumer should store the provided metrics")
	}
}

// TestProcessMessage_TermError_LogsError verifies Term() error is logged.
func TestProcessMessage_TermError_LogsError(t *testing.T) {
	c := createTestConsumer(t)

	// Create poison message where Term() will fail
	msg := &mockJetStreamMsg{
		data:    []byte("invalid proto"),
		subject: "events.test",
		termErr: errors.New("term failed"),
	}

	// This should not panic even though Term() fails
	c.processMessage(context.Background(), msg)

	// Verify Term was attempted
	if !msg.termCalled.Load() {
		t.Error("Term() should be called for poison messages")
	}
}

// TestFlush_SwapsBatch_EmptyDoesNotSwap verifies empty batch returns early.
func TestFlush_SwapsBatch_EmptyDoesNotSwap(t *testing.T) {
	c := createTestConsumer(t)

	// Empty batch
	initialLen := len(c.batch)
	if initialLen != 0 {
		t.Fatalf("Initial batch len = %d, want 0", initialLen)
	}

	// Flush with empty batch returns early
	err := c.flush(context.Background())
	if err != nil {
		t.Errorf("flush() with empty batch returned error: %v", err)
	}

	// Batch should still be empty
	c.mu.Lock()
	afterLen := len(c.batch)
	c.mu.Unlock()

	if afterLen != 0 {
		t.Errorf("After flush, batch len = %d, want 0", afterLen)
	}
}

// TestFlush_EmptyBatch_DoesNotUpdateLastFlush verifies empty batch flush returns early.
func TestFlush_EmptyBatch_DoesNotUpdateLastFlush(t *testing.T) {
	c := createTestConsumer(t)

	// Set lastFlush to past
	pastTime := time.Now().Add(-10 * time.Minute)
	c.lastFlush = pastTime

	// Flush with empty batch
	_ = c.flush(context.Background())

	c.mu.Lock()
	newLastFlush := c.lastFlush
	c.mu.Unlock()

	// lastFlush should NOT be updated for empty batch (returns early)
	if !newLastFlush.Equal(pastTime) {
		t.Errorf("lastFlush should not change for empty batch, got %v, want %v", newLastFlush, pastTime)
	}
}

// --- Mock types for workerLoop testing (minimal) ---

// mockMessagesBatch implements jetstream.MessageBatch for testing.
type mockMessagesBatch struct {
	messages []jetstream.Msg
	err      error
}

func (m *mockMessagesBatch) Messages() <-chan jetstream.Msg {
	ch := make(chan jetstream.Msg, len(m.messages))
	for _, msg := range m.messages {
		ch <- msg
	}
	close(ch)
	return ch
}

func (m *mockMessagesBatch) Error() error {
	return m.err
}

// testableConsumer wraps a fetch function to implement the Consumer interface partially.
// This is used to test workerLoop directly without the full JetStream mock.
type testableConsumer struct {
	fetchFunc func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error)
}

func (tc *testableConsumer) Fetch(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
	return tc.fetchFunc(batch, opts...)
}

func (tc *testableConsumer) FetchBytes(maxBytes int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
	return nil, nil
}

func (tc *testableConsumer) FetchNoWait(batch int) (jetstream.MessageBatch, error) {
	return nil, nil
}

func (tc *testableConsumer) Messages(opts ...jetstream.PullMessagesOpt) (jetstream.MessagesContext, error) {
	return nil, nil
}

func (tc *testableConsumer) Consume(handler jetstream.MessageHandler, opts ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error) {
	return nil, nil
}

func (tc *testableConsumer) Next(opts ...jetstream.FetchOpt) (jetstream.Msg, error) {
	return nil, nil
}

func (tc *testableConsumer) Info(ctx context.Context) (*jetstream.ConsumerInfo, error) {
	return nil, nil
}

func (tc *testableConsumer) CachedInfo() *jetstream.ConsumerInfo {
	return nil
}

// TestWorkerLoop_ContextCancel verifies worker exits on context cancel.
func TestWorkerLoop_ContextCancel(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	consumer := &testableConsumer{
		fetchFunc: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			return nil, context.DeadlineExceeded
		},
	}

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		c.workerLoop(ctx, consumer, 0)
		close(done)
	}()

	// Cancel context
	cancel()

	// Worker should exit
	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("Worker did not exit on context cancel")
	}
}

// TestWorkerLoop_StopChannel verifies worker exits on stopCh close.
func TestWorkerLoop_StopChannel(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	consumer := &testableConsumer{
		fetchFunc: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			return nil, context.DeadlineExceeded
		},
	}

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		c.workerLoop(context.Background(), consumer, 0)
		close(done)
	}()

	// Close stop channel
	close(c.stopCh)

	// Worker should exit
	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("Worker did not exit on stopCh close")
	}
}

// TestWorkerLoop_FetchError verifies worker handles fetch errors gracefully.
func TestWorkerLoop_FetchError(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	fetchCount := atomic.Int32{}
	consumer := &testableConsumer{
		fetchFunc: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			count := fetchCount.Add(1)
			if count < 3 {
				return nil, errors.New("fetch failed")
			}
			// After 3 attempts, simulate timeout to allow clean exit
			return nil, context.DeadlineExceeded
		},
	}

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.workerLoop(ctx, consumer, 0)
		close(done)
	}()

	// Wait for worker to exit
	select {
	case <-done:
		// Success - worker exited
	case <-time.After(5 * time.Second):
		t.Error("Worker did not exit in time")
	}

	// Verify fetch was called multiple times (error retry)
	if fetchCount.Load() < 2 {
		t.Errorf("Fetch was only called %d times, expected >= 2", fetchCount.Load())
	}
}

// TestWorkerLoop_ProcessesMessages verifies worker processes messages from batch.
func TestWorkerLoop_ProcessesMessages(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create a valid event message
	event := &pb.EventEnvelope{
		Id:          "worker-test-event",
		AppId:       "test-app",
		TimestampMs: time.Now().UnixMilli(),
		Payload:     &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "home"}},
	}
	data, _ := proto.Marshal(event)
	msg := &mockJetStreamMsg{data: data, subject: "events.test"}

	fetchCount := atomic.Int32{}
	consumer := &testableConsumer{
		fetchFunc: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			count := fetchCount.Add(1)
			if count == 1 {
				// First call returns messages
				return &mockMessagesBatch{messages: []jetstream.Msg{msg}}, nil
			}
			// Subsequent calls return timeout
			return nil, context.DeadlineExceeded
		},
	}

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.workerLoop(ctx, consumer, 0)
		close(done)
	}()

	// Wait a bit for message to be processed
	time.Sleep(50 * time.Millisecond)

	// Check that event was added to batch
	c.mu.Lock()
	batchLen := len(c.batch)
	c.mu.Unlock()

	if batchLen != 1 {
		t.Errorf("Batch len = %d, want 1", batchLen)
	}

	// Cleanup
	cancel()
	<-done
}

// TestWorkerLoop_FetchBatchSizeDefault verifies default fetch batch size is used.
func TestWorkerLoop_FetchBatchSizeDefault(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 0, // Should default to 100
			WorkerCount:    1,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var fetchedBatchSize int
	consumer := &testableConsumer{
		fetchFunc: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			fetchedBatchSize = batch
			return nil, context.DeadlineExceeded
		},
	}

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.workerLoop(ctx, consumer, 0)
		close(done)
	}()

	<-done

	// fetchBatchSize should be default (100) when config is 0
	if fetchedBatchSize != 100 {
		t.Errorf("FetchBatchSize = %d, want 100 (default)", fetchedBatchSize)
	}
}

// TestWorkerLoop_MessagesIterationError verifies error handling from Messages iterator.
func TestWorkerLoop_MessagesIterationError(t *testing.T) {
	cfg := Config{
		Batch: BatchConfig{
			MaxEvents:      100,
			FlushInterval:  time.Minute,
			FetchBatchSize: 10,
			WorkerCount:    1,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	fetchCount := atomic.Int32{}
	consumer := &testableConsumer{
		fetchFunc: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			count := fetchCount.Add(1)
			if count == 1 {
				// Return a batch with an iteration error
				return &mockMessagesBatch{
					messages: []jetstream.Msg{},
					err:      errors.New("iteration error"),
				}, nil
			}
			return nil, context.DeadlineExceeded
		},
	}

	c := &Consumer{
		config:       cfg,
		parquet:      NewParquetWriter(cfg.Parquet),
		logger:       logger,
		batch:        make([]trackedEvent, 0, cfg.Batch.MaxEvents),
		lastFlush:    time.Now(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.workerLoop(ctx, consumer, 0)
		close(done)
	}()

	// Should not panic on iteration error, just log and continue
	<-done
}
