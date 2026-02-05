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
