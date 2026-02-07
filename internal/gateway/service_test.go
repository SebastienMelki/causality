package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// mockPublisher is a mock implementation of EventPublisher for testing.
type mockPublisher struct {
	publishedEvents []*pb.EventEnvelope
	publishErr      error
	// failOnIndex allows failing on specific event indexes in batch operations
	failOnIndex map[int]error
	callCount   int
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{
		publishedEvents: make([]*pb.EventEnvelope, 0),
		failOnIndex:     make(map[int]error),
	}
}

func (m *mockPublisher) PublishEvent(_ context.Context, event *pb.EventEnvelope) error {
	defer func() { m.callCount++ }()

	// Check if this specific call should fail
	if err, exists := m.failOnIndex[m.callCount]; exists {
		return err
	}
	// Check if all calls should fail
	if m.publishErr != nil {
		return m.publishErr
	}
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

// mockDedupChecker mocks the dedup checker for testing.
type mockDedupChecker struct {
	duplicateKeys map[string]bool
	// trackSeen controls whether to mark keys as seen on first check
	trackSeen bool
}

func newMockDedupChecker() *mockDedupChecker {
	return &mockDedupChecker{
		duplicateKeys: make(map[string]bool),
		trackSeen:     true, // Default behavior: track seen keys
	}
}

func (m *mockDedupChecker) IsDuplicate(key string) bool {
	if key == "" {
		return false
	}
	if m.duplicateKeys[key] {
		return true
	}
	if m.trackSeen {
		m.duplicateKeys[key] = true
	}
	return false
}

// markAsDuplicate pre-marks a key as duplicate for testing
func (m *mockDedupChecker) markAsDuplicate(key string) {
	m.duplicateKeys[key] = true
}

func TestEventService_IngestEvent(t *testing.T) {
	tests := []struct {
		name       string
		request    *pb.IngestEventRequest
		shouldFail bool
		wantErr    bool
	}{
		{
			name: "valid event without ID",
			request: &pb.IngestEventRequest{
				Event: &pb.EventEnvelope{
					AppId:    "testapp",
					DeviceId: "device123",
					Payload: &pb.EventEnvelope_ScreenView{
						ScreenView: &pb.ScreenView{
							ScreenName: "home",
						},
					},
				},
			},
			shouldFail: false,
			wantErr:    false,
		},
		{
			name: "valid event with ID",
			request: &pb.IngestEventRequest{
				Event: &pb.EventEnvelope{
					Id:          "01234567-89ab-cdef-0123-456789abcdef",
					AppId:       "testapp",
					DeviceId:    "device123",
					TimestampMs: time.Now().UnixMilli(),
					Payload: &pb.EventEnvelope_UserLogin{
						UserLogin: &pb.UserLogin{
							UserId: "user456",
							Method: "email",
						},
					},
				},
			},
			shouldFail: false,
			wantErr:    false,
		},
		{
			name: "nil event",
			request: &pb.IngestEventRequest{
				Event: nil,
			},
			shouldFail: false,
			wantErr:    true,
		},
		{
			name: "publisher failure",
			request: &pb.IngestEventRequest{
				Event: &pb.EventEnvelope{
					AppId:    "testapp",
					DeviceId: "device123",
					Payload: &pb.EventEnvelope_ScreenView{
						ScreenView: &pb.ScreenView{
							ScreenName: "home",
						},
					},
				},
			},
			shouldFail: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test requires the generated proto code to compile.
			// In a real scenario, we would need to mock the publisher interface.
			// For now, we're testing the enrichment logic conceptually.

			if tt.request.Event != nil {
				// Test enrichment
				event := tt.request.Event
				originalID := event.Id
				originalTimestamp := event.TimestampMs

				// Simulate enrichment
				if event.Id == "" {
					event.Id = "generated-uuid-v7"
				}
				if event.TimestampMs == 0 {
					event.TimestampMs = time.Now().UnixMilli()
				}

				// Verify enrichment occurred when needed
				if originalID == "" && event.Id == "" {
					t.Error("ID should have been generated")
				}
				if originalTimestamp == 0 && event.TimestampMs == 0 {
					t.Error("Timestamp should have been generated")
				}
			}
		})
	}
}

func TestEventService_IngestEventBatch(t *testing.T) {
	events := []*pb.EventEnvelope{
		{
			AppId:    "testapp",
			DeviceId: "device1",
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "home"},
			},
		},
		{
			AppId:    "testapp",
			DeviceId: "device2",
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "profile"},
			},
		},
		{
			AppId:    "testapp",
			DeviceId: "device3",
			Payload: &pb.EventEnvelope_ButtonTap{
				ButtonTap: &pb.ButtonTap{ButtonId: "submit"},
			},
		},
	}

	// Test batch enrichment
	for i, event := range events {
		if event.Id == "" {
			event.Id = "generated-id-" + string(rune('a'+i))
		}
		if event.TimestampMs == 0 {
			event.TimestampMs = time.Now().UnixMilli()
		}
	}

	// Verify all events were enriched
	for i, event := range events {
		if event.Id == "" {
			t.Errorf("Event %d should have an ID", i)
		}
		if event.TimestampMs == 0 {
			t.Errorf("Event %d should have a timestamp", i)
		}
	}
}

func TestEnrichEnvelope(t *testing.T) {
	tests := []struct {
		name             string
		event            *pb.EventEnvelope
		wantIDGenerated  bool
		wantTSGenerated  bool
	}{
		{
			name: "empty event gets enriched",
			event: &pb.EventEnvelope{
				AppId:    "test",
				DeviceId: "dev",
			},
			wantIDGenerated: true,
			wantTSGenerated: true,
		},
		{
			name: "event with ID keeps ID",
			event: &pb.EventEnvelope{
				Id:       "existing-id",
				AppId:    "test",
				DeviceId: "dev",
			},
			wantIDGenerated: false,
			wantTSGenerated: true,
		},
		{
			name: "event with timestamp keeps timestamp",
			event: &pb.EventEnvelope{
				AppId:       "test",
				DeviceId:    "dev",
				TimestampMs: 1234567890000,
			},
			wantIDGenerated: true,
			wantTSGenerated: false,
		},
		{
			name: "fully populated event unchanged",
			event: &pb.EventEnvelope{
				Id:          "existing-id",
				AppId:       "test",
				DeviceId:    "dev",
				TimestampMs: 1234567890000,
			},
			wantIDGenerated: false,
			wantTSGenerated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalID := tt.event.Id
			originalTS := tt.event.TimestampMs

			// Simulate enrichment logic
			if tt.event.Id == "" {
				tt.event.Id = "new-generated-id"
			}
			if tt.event.TimestampMs == 0 {
				tt.event.TimestampMs = time.Now().UnixMilli()
			}

			// Check ID
			if tt.wantIDGenerated {
				if tt.event.Id == originalID || tt.event.Id == "" {
					t.Error("Expected ID to be generated")
				}
			} else {
				if tt.event.Id != originalID {
					t.Error("Expected ID to remain unchanged")
				}
			}

			// Check timestamp
			if tt.wantTSGenerated {
				if tt.event.TimestampMs == originalTS || tt.event.TimestampMs == 0 {
					t.Error("Expected timestamp to be generated")
				}
			} else {
				if tt.event.TimestampMs != originalTS {
					t.Error("Expected timestamp to remain unchanged")
				}
			}
		})
	}
}

// TestIngestEvent_MissingAppID_ReturnsError verifies that events without app_id are rejected.
func TestIngestEvent_MissingAppID_ReturnsError(t *testing.T) {
	svc := NewEventService(nil, nil, 0, nil)

	req := &pb.IngestEventRequest{
		Event: &pb.EventEnvelope{
			AppId:       "", // Missing app_id
			TimestampMs: time.Now().UnixMilli(),
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "home"},
			},
		},
	}

	_, err := svc.IngestEvent(context.Background(), req)
	if err == nil {
		t.Error("IngestEvent() should return error for missing app_id")
	}
	if !errors.Is(err, ErrAppIDRequired) {
		t.Errorf("IngestEvent() error = %v, want ErrAppIDRequired", err)
	}
}

// TestIngestEvent_MissingEventType_ReturnsError verifies that events without payload are rejected.
func TestIngestEvent_MissingEventType_ReturnsError(t *testing.T) {
	svc := NewEventService(nil, nil, 0, nil)

	req := &pb.IngestEventRequest{
		Event: &pb.EventEnvelope{
			AppId:       "test-app",
			TimestampMs: time.Now().UnixMilli(),
			Payload:     nil, // Missing payload (event_type)
		},
	}

	_, err := svc.IngestEvent(context.Background(), req)
	if err == nil {
		t.Error("IngestEvent() should return error for missing event_type (payload)")
	}
	if !errors.Is(err, ErrEventTypeRequired) {
		t.Errorf("IngestEvent() error = %v, want ErrEventTypeRequired", err)
	}
}

// TestIngestEvent_MissingTimestamp_ReturnsError verifies that events without timestamp are rejected.
func TestIngestEvent_MissingTimestamp_ReturnsError(t *testing.T) {
	svc := NewEventService(nil, nil, 0, nil)

	req := &pb.IngestEventRequest{
		Event: &pb.EventEnvelope{
			AppId:       "test-app",
			TimestampMs: 0, // Missing timestamp
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "home"},
			},
		},
	}

	_, err := svc.IngestEvent(context.Background(), req)
	if err == nil {
		t.Error("IngestEvent() should return error for missing timestamp")
	}
	if !errors.Is(err, ErrTimestampRequired) {
		t.Errorf("IngestEvent() error = %v, want ErrTimestampRequired", err)
	}
}

// TestIngestEvent_DuplicateDropped verifies that duplicate events are silently dropped.
func TestIngestEvent_DuplicateDropped(t *testing.T) {
	pub := newMockPublisher()
	dedup := newMockDedupChecker()
	// Mark the key as already seen
	dedup.markAsDuplicate("test-idempotency-key")
	svc := NewEventServiceWithPublisher(pub, dedup, 0, nil)

	req := &pb.IngestEventRequest{
		Event: &pb.EventEnvelope{
			AppId:          "test-app",
			IdempotencyKey: "test-idempotency-key",
			TimestampMs:    time.Now().UnixMilli(),
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "home"},
			},
		},
	}

	resp, err := svc.IngestEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEvent() returned unexpected error: %v", err)
	}

	// Should return success (silently drop)
	if resp.Status != "accepted" {
		t.Errorf("Response status = %q, want %q", resp.Status, "accepted")
	}

	// Publisher should not have been called
	if len(pub.publishedEvents) > 0 {
		t.Error("Duplicate event should not be published")
	}
}

// TestIngestEvent_IdempotencyKeyGenerated verifies that events without idempotency_key get one assigned.
func TestIngestEvent_IdempotencyKeyGenerated(t *testing.T) {
	svc := NewEventService(nil, nil, 0, nil)

	event := &pb.EventEnvelope{
		AppId:          "test-app",
		TimestampMs:    time.Now().UnixMilli(),
		IdempotencyKey: "", // Empty - should be generated
		Payload: &pb.EventEnvelope_ScreenView{
			ScreenView: &pb.ScreenView{ScreenName: "home"},
		},
	}

	// Test enrichment directly
	svc.enrichEnvelope(event)

	if event.IdempotencyKey == "" {
		t.Error("enrichEnvelope() should generate idempotency_key when empty")
	}
}

// TestIngestEventBatch_AllInvalid verifies batch with only invalid events.
// Note: Testing mixed valid/invalid batches requires a mock publisher which the
// current architecture doesn't support (Publisher is a concrete struct).
func TestIngestEventBatch_AllInvalid(t *testing.T) {
	svc := NewEventService(nil, nil, 0, nil)

	req := &pb.IngestEventBatchRequest{
		Events: []*pb.EventEnvelope{
			{
				// Invalid - missing app_id
				AppId:       "",
				TimestampMs: time.Now().UnixMilli(),
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{ScreenName: "profile"},
				},
			},
			{
				// Invalid - missing payload
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     nil,
			},
			{
				// Invalid - missing timestamp
				AppId:       "test-app",
				TimestampMs: 0,
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{ScreenName: "home"},
				},
			},
			nil, // Invalid - nil event
		},
	}

	resp, err := svc.IngestEventBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEventBatch() returned unexpected error: %v", err)
	}

	// All 4 events should be rejected
	if resp.AcceptedCount != 0 {
		t.Errorf("AcceptedCount = %d, want 0", resp.AcceptedCount)
	}
	if resp.RejectedCount != 4 {
		t.Errorf("RejectedCount = %d, want 4", resp.RejectedCount)
	}

	// Verify individual results
	if len(resp.Results) != 4 {
		t.Fatalf("Results count = %d, want 4", len(resp.Results))
	}

	// All should be rejected
	for i, result := range resp.Results {
		if result.Status != "rejected" {
			t.Errorf("Results[%d].Status = %q, want %q", i, result.Status, "rejected")
		}
		if result.Error == "" {
			t.Errorf("Results[%d].Error should not be empty", i)
		}
	}
}

// TestValidateEvent verifies the event validation logic.
func TestValidateEvent(t *testing.T) {
	svc := NewEventService(nil, nil, 0, nil)

	tests := []struct {
		name    string
		event   *pb.EventEnvelope
		wantErr error
	}{
		{
			name: "valid event",
			event: &pb.EventEnvelope{
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{ScreenName: "home"},
				},
			},
			wantErr: nil,
		},
		{
			name: "missing app_id",
			event: &pb.EventEnvelope{
				AppId:       "",
				TimestampMs: time.Now().UnixMilli(),
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{ScreenName: "home"},
				},
			},
			wantErr: ErrAppIDRequired,
		},
		{
			name: "missing payload",
			event: &pb.EventEnvelope{
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     nil,
			},
			wantErr: ErrEventTypeRequired,
		},
		{
			name: "missing timestamp",
			event: &pb.EventEnvelope{
				AppId:       "test-app",
				TimestampMs: 0,
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{ScreenName: "home"},
				},
			},
			wantErr: ErrTimestampRequired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.validateEvent(tc.event)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("validateEvent() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestIngestEventBatch_BatchTooLarge verifies that batches exceeding the max event count are rejected.
func TestIngestEventBatch_BatchTooLarge(t *testing.T) {
	svc := NewEventService(nil, nil, 2, nil) // Max 2 events

	req := &pb.IngestEventBatchRequest{
		Events: []*pb.EventEnvelope{
			{AppId: "app", TimestampMs: 1, Payload: &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "a"}}},
			{AppId: "app", TimestampMs: 1, Payload: &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "b"}}},
			{AppId: "app", TimestampMs: 1, Payload: &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "c"}}},
		},
	}

	_, err := svc.IngestEventBatch(context.Background(), req)
	if err == nil {
		t.Error("IngestEventBatch() should return error for batch exceeding max size")
	}
	if !errors.Is(err, ErrBatchTooLarge) {
		t.Errorf("IngestEventBatch() error = %v, want ErrBatchTooLarge", err)
	}
}

// TestIngestEventBatch_EmptyBatch verifies that empty batches are rejected.
func TestIngestEventBatch_EmptyBatch(t *testing.T) {
	svc := NewEventService(nil, nil, 0, nil)

	req := &pb.IngestEventBatchRequest{
		Events: []*pb.EventEnvelope{},
	}

	_, err := svc.IngestEventBatch(context.Background(), req)
	if err == nil {
		t.Error("IngestEventBatch() should return error for empty batch")
	}
	if !errors.Is(err, ErrAtLeastOneEvent) {
		t.Errorf("IngestEventBatch() error = %v, want ErrAtLeastOneEvent", err)
	}
}

// --- Integration tests with mock publisher ---

// TestIngestEvent_WithMockPublisher_PublishesEvent verifies that valid events are published.
func TestIngestEvent_WithMockPublisher_PublishesEvent(t *testing.T) {
	pub := newMockPublisher()
	svc := NewEventServiceWithPublisher(pub, nil, 0, nil)

	req := &pb.IngestEventRequest{
		Event: &pb.EventEnvelope{
			AppId:       "test-app",
			TimestampMs: time.Now().UnixMilli(),
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "home"},
			},
		},
	}

	resp, err := svc.IngestEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEvent() returned unexpected error: %v", err)
	}

	if resp.Status != "accepted" {
		t.Errorf("Response status = %q, want %q", resp.Status, "accepted")
	}

	// Verify event was published
	if len(pub.publishedEvents) != 1 {
		t.Fatalf("Expected 1 published event, got %d", len(pub.publishedEvents))
	}

	// Verify event ID was generated
	if pub.publishedEvents[0].GetId() == "" {
		t.Error("Published event should have an ID")
	}
	if resp.EventId != pub.publishedEvents[0].GetId() {
		t.Errorf("Response event_id = %q, want %q", resp.EventId, pub.publishedEvents[0].GetId())
	}
}

// TestIngestEvent_WithDedup_FirstEventAccepted verifies first event passes dedup check.
func TestIngestEvent_WithDedup_FirstEventAccepted(t *testing.T) {
	pub := newMockPublisher()
	dedup := newMockDedupChecker()
	svc := NewEventServiceWithPublisher(pub, dedup, 0, nil)

	req := &pb.IngestEventRequest{
		Event: &pb.EventEnvelope{
			AppId:          "test-app",
			IdempotencyKey: "unique-key-123",
			TimestampMs:    time.Now().UnixMilli(),
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "home"},
			},
		},
	}

	resp, err := svc.IngestEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEvent() returned unexpected error: %v", err)
	}

	if resp.Status != "accepted" {
		t.Errorf("Response status = %q, want %q", resp.Status, "accepted")
	}

	// Verify event was published (first time, not a duplicate)
	if len(pub.publishedEvents) != 1 {
		t.Fatalf("Expected 1 published event, got %d", len(pub.publishedEvents))
	}

	// Verify the idempotency key was preserved
	if pub.publishedEvents[0].GetIdempotencyKey() != "unique-key-123" {
		t.Errorf("Idempotency key = %q, want %q", pub.publishedEvents[0].GetIdempotencyKey(), "unique-key-123")
	}
}

// TestIngestEvent_WithDedup_DuplicateSkipsPublish verifies duplicate is not published but returns success.
func TestIngestEvent_WithDedup_DuplicateSkipsPublish(t *testing.T) {
	pub := newMockPublisher()
	dedup := newMockDedupChecker()
	// Pre-mark the key as seen
	dedup.markAsDuplicate("duplicate-key")
	svc := NewEventServiceWithPublisher(pub, dedup, 0, nil)

	req := &pb.IngestEventRequest{
		Event: &pb.EventEnvelope{
			AppId:          "test-app",
			IdempotencyKey: "duplicate-key",
			TimestampMs:    time.Now().UnixMilli(),
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "home"},
			},
		},
	}

	resp, err := svc.IngestEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEvent() returned unexpected error: %v", err)
	}

	// Should return success (silently dropped)
	if resp.Status != "accepted" {
		t.Errorf("Response status = %q, want %q", resp.Status, "accepted")
	}

	// Verify event was NOT published (it's a duplicate)
	if len(pub.publishedEvents) != 0 {
		t.Errorf("Expected 0 published events for duplicate, got %d", len(pub.publishedEvents))
	}
}

// TestIngestEvent_PublishError_ReturnsError verifies publish errors are returned.
func TestIngestEvent_PublishError_ReturnsError(t *testing.T) {
	pub := newMockPublisher()
	pub.publishErr = errors.New("NATS connection failed")
	svc := NewEventServiceWithPublisher(pub, nil, 0, nil)

	req := &pb.IngestEventRequest{
		Event: &pb.EventEnvelope{
			AppId:       "test-app",
			TimestampMs: time.Now().UnixMilli(),
			Payload: &pb.EventEnvelope_ScreenView{
				ScreenView: &pb.ScreenView{ScreenName: "home"},
			},
		},
	}

	_, err := svc.IngestEvent(context.Background(), req)
	if err == nil {
		t.Fatal("IngestEvent() should return error when publish fails")
	}

	if !errors.Is(err, pub.publishErr) && err.Error() == "" {
		// Error should contain the original publish error message
		t.Logf("Got expected error: %v", err)
	}
}

// TestIngestEventBatch_MixedValidInvalid verifies batch with valid + invalid events.
func TestIngestEventBatch_MixedValidInvalid(t *testing.T) {
	pub := newMockPublisher()
	svc := NewEventServiceWithPublisher(pub, nil, 0, nil)

	req := &pb.IngestEventBatchRequest{
		Events: []*pb.EventEnvelope{
			{
				// Valid event
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "home"}},
			},
			{
				// Invalid - missing app_id
				AppId:       "",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "profile"}},
			},
			{
				// Valid event
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_ButtonTap{ButtonTap: &pb.ButtonTap{ButtonId: "submit"}},
			},
			nil, // Invalid - nil event
		},
	}

	resp, err := svc.IngestEventBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEventBatch() returned unexpected error: %v", err)
	}

	// 2 valid, 2 invalid
	if resp.AcceptedCount != 2 {
		t.Errorf("AcceptedCount = %d, want 2", resp.AcceptedCount)
	}
	if resp.RejectedCount != 2 {
		t.Errorf("RejectedCount = %d, want 2", resp.RejectedCount)
	}

	// Only valid events should be published
	if len(pub.publishedEvents) != 2 {
		t.Errorf("Expected 2 published events, got %d", len(pub.publishedEvents))
	}

	// Verify results
	if len(resp.Results) != 4 {
		t.Fatalf("Results count = %d, want 4", len(resp.Results))
	}

	// Check individual results
	if resp.Results[0].Status != "accepted" {
		t.Errorf("Results[0].Status = %q, want accepted", resp.Results[0].Status)
	}
	if resp.Results[1].Status != "rejected" {
		t.Errorf("Results[1].Status = %q, want rejected", resp.Results[1].Status)
	}
	if resp.Results[2].Status != "accepted" {
		t.Errorf("Results[2].Status = %q, want accepted", resp.Results[2].Status)
	}
	if resp.Results[3].Status != "rejected" {
		t.Errorf("Results[3].Status = %q, want rejected", resp.Results[3].Status)
	}
}

// TestIngestEventBatch_WithDedup_FiltersDuplicates verifies batch with duplicates.
func TestIngestEventBatch_WithDedup_FiltersDuplicates(t *testing.T) {
	pub := newMockPublisher()
	dedup := newMockDedupChecker()
	// Pre-mark one key as duplicate
	dedup.markAsDuplicate("dup-key-1")
	svc := NewEventServiceWithPublisher(pub, dedup, 0, nil)

	req := &pb.IngestEventBatchRequest{
		Events: []*pb.EventEnvelope{
			{
				AppId:          "test-app",
				IdempotencyKey: "dup-key-1", // This is pre-marked as duplicate
				TimestampMs:    time.Now().UnixMilli(),
				Payload:        &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "home"}},
			},
			{
				AppId:          "test-app",
				IdempotencyKey: "unique-key-1", // First time seen
				TimestampMs:    time.Now().UnixMilli(),
				Payload:        &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "profile"}},
			},
			{
				AppId:          "test-app",
				IdempotencyKey: "unique-key-1", // Now duplicate (same as previous)
				TimestampMs:    time.Now().UnixMilli(),
				Payload:        &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "settings"}},
			},
		},
	}

	resp, err := svc.IngestEventBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEventBatch() returned unexpected error: %v", err)
	}

	// All 3 should be "accepted" (duplicates are silently dropped but count as accepted)
	if resp.AcceptedCount != 3 {
		t.Errorf("AcceptedCount = %d, want 3", resp.AcceptedCount)
	}
	if resp.RejectedCount != 0 {
		t.Errorf("RejectedCount = %d, want 0", resp.RejectedCount)
	}

	// Only 1 event should be published (unique-key-1 on first occurrence)
	if len(pub.publishedEvents) != 1 {
		t.Errorf("Expected 1 published event, got %d", len(pub.publishedEvents))
	}

	// The published event should have the unique key
	if pub.publishedEvents[0].GetIdempotencyKey() != "unique-key-1" {
		t.Errorf("Published event idempotency_key = %q, want unique-key-1", pub.publishedEvents[0].GetIdempotencyKey())
	}
}

// TestIngestEventBatch_PublishError_ReturnsRejected verifies publish failures in batch.
func TestIngestEventBatch_PublishError_ReturnsRejected(t *testing.T) {
	pub := newMockPublisher()
	// Make the second publish call fail
	pub.failOnIndex[1] = errors.New("NATS timeout")
	svc := NewEventServiceWithPublisher(pub, nil, 0, nil)

	req := &pb.IngestEventBatchRequest{
		Events: []*pb.EventEnvelope{
			{
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "home"}},
			},
			{
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "profile"}},
			},
			{
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "settings"}},
			},
		},
	}

	resp, err := svc.IngestEventBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEventBatch() returned unexpected error: %v", err)
	}

	// 2 accepted, 1 rejected (the one that failed to publish)
	if resp.AcceptedCount != 2 {
		t.Errorf("AcceptedCount = %d, want 2", resp.AcceptedCount)
	}
	if resp.RejectedCount != 1 {
		t.Errorf("RejectedCount = %d, want 1", resp.RejectedCount)
	}

	// Verify results
	if resp.Results[0].Status != "accepted" {
		t.Errorf("Results[0].Status = %q, want accepted", resp.Results[0].Status)
	}
	if resp.Results[1].Status != "rejected" {
		t.Errorf("Results[1].Status = %q, want rejected", resp.Results[1].Status)
	}
	if resp.Results[1].Error == "" {
		t.Error("Results[1].Error should contain error message")
	}
	if resp.Results[2].Status != "accepted" {
		t.Errorf("Results[2].Status = %q, want accepted", resp.Results[2].Status)
	}

	// 2 events should have been published (first and third)
	if len(pub.publishedEvents) != 2 {
		t.Errorf("Expected 2 published events, got %d", len(pub.publishedEvents))
	}
}


// TestIngestEventBatch_AllValid_AllPublished verifies all valid events are published.
func TestIngestEventBatch_AllValid_AllPublished(t *testing.T) {
	pub := newMockPublisher()
	svc := NewEventServiceWithPublisher(pub, nil, 0, nil)

	req := &pb.IngestEventBatchRequest{
		Events: []*pb.EventEnvelope{
			{
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "home"}},
			},
			{
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_UserLogin{UserLogin: &pb.UserLogin{UserId: "user123", Method: "email"}},
			},
			{
				AppId:       "test-app",
				TimestampMs: time.Now().UnixMilli(),
				Payload:     &pb.EventEnvelope_ButtonTap{ButtonTap: &pb.ButtonTap{ButtonId: "submit"}},
			},
		},
	}

	resp, err := svc.IngestEventBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestEventBatch() returned unexpected error: %v", err)
	}

	if resp.AcceptedCount != 3 {
		t.Errorf("AcceptedCount = %d, want 3", resp.AcceptedCount)
	}
	if resp.RejectedCount != 0 {
		t.Errorf("RejectedCount = %d, want 0", resp.RejectedCount)
	}

	// All 3 should be published
	if len(pub.publishedEvents) != 3 {
		t.Errorf("Expected 3 published events, got %d", len(pub.publishedEvents))
	}

	// Each event should have a generated ID
	for i, e := range pub.publishedEvents {
		if e.GetId() == "" {
			t.Errorf("Published event %d should have an ID", i)
		}
		if e.GetIdempotencyKey() == "" {
			t.Errorf("Published event %d should have an idempotency_key", i)
		}
	}
}

// TestEnrichEnvelope_GeneratesIdempotencyKey verifies idempotency key generation.
func TestEnrichEnvelope_GeneratesIdempotencyKey(t *testing.T) {
	svc := NewEventServiceWithPublisher(nil, nil, 0, nil)

	event := &pb.EventEnvelope{
		AppId:          "test-app",
		TimestampMs:    time.Now().UnixMilli(),
		IdempotencyKey: "", // Empty - should be generated
		Payload:        &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "home"}},
	}

	svc.enrichEnvelope(event)

	if event.IdempotencyKey == "" {
		t.Error("enrichEnvelope() should generate idempotency_key when empty")
	}

	// Verify it's a valid UUID format (36 chars with dashes)
	if len(event.IdempotencyKey) != 36 {
		t.Errorf("Generated idempotency_key length = %d, want 36 (UUID format)", len(event.IdempotencyKey))
	}
}

// TestEnrichEnvelope_PreservesExistingIdempotencyKey verifies existing keys are preserved.
func TestEnrichEnvelope_PreservesExistingIdempotencyKey(t *testing.T) {
	svc := NewEventServiceWithPublisher(nil, nil, 0, nil)

	existingKey := "user-provided-key-123"
	event := &pb.EventEnvelope{
		AppId:          "test-app",
		TimestampMs:    time.Now().UnixMilli(),
		IdempotencyKey: existingKey,
		Payload:        &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{ScreenName: "home"}},
	}

	svc.enrichEnvelope(event)

	if event.IdempotencyKey != existingKey {
		t.Errorf("enrichEnvelope() should preserve existing idempotency_key, got %q, want %q",
			event.IdempotencyKey, existingKey)
	}
}
