package batch

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SebastienMelki/causality/sdk/mobile/internal/storage"
	"github.com/SebastienMelki/causality/sdk/mobile/internal/transport"
)

// --- Mock implementations ---

// mockQueue implements EventQueue for testing.
type mockQueue struct {
	mu          sync.Mutex
	events      []storage.QueuedEvent
	nextID      int64
	enqueueCalls int
	deleteCalls  int
	retryCalls   int

	enqueueErr error
	dequeueErr error
	deleteErr  error
	retryErr   error
}

func newMockQueue() *mockQueue {
	return &mockQueue{
		events: make([]storage.QueuedEvent, 0),
		nextID: 1,
	}
}

func (q *mockQueue) Enqueue(eventJSON string, idempotencyKey string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enqueueCalls++

	if q.enqueueErr != nil {
		return q.enqueueErr
	}

	q.events = append(q.events, storage.QueuedEvent{
		ID:             q.nextID,
		EventJSON:      eventJSON,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now().UnixMilli(),
		RetryCount:     0,
	})
	q.nextID++
	return nil
}

func (q *mockQueue) DequeueBatch(n int) ([]storage.QueuedEvent, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.dequeueErr != nil {
		return nil, q.dequeueErr
	}

	limit := n
	if limit > len(q.events) {
		limit = len(q.events)
	}

	result := make([]storage.QueuedEvent, limit)
	copy(result, q.events[:limit])
	return result, nil
}

func (q *mockQueue) Delete(ids []int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.deleteCalls++

	if q.deleteErr != nil {
		return q.deleteErr
	}

	idSet := make(map[int64]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	remaining := make([]storage.QueuedEvent, 0, len(q.events))
	for _, e := range q.events {
		if !idSet[e.ID] {
			remaining = append(remaining, e)
		}
	}
	q.events = remaining
	return nil
}

func (q *mockQueue) MarkRetry(id int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.retryCalls++

	if q.retryErr != nil {
		return q.retryErr
	}

	for i := range q.events {
		if q.events[i].ID == id {
			q.events[i].RetryCount++
			return nil
		}
	}
	return fmt.Errorf("event %d not found", id)
}

func (q *mockQueue) Count() (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.events), nil
}

func (q *mockQueue) getEvents() []storage.QueuedEvent {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]storage.QueuedEvent, len(q.events))
	copy(result, q.events)
	return result
}

// mockSender implements EventSender for testing.
type mockSender struct {
	mu        sync.Mutex
	calls     int
	lastBatch []string
	err       error
	result    *transport.SendResult
}

func newMockSender() *mockSender {
	return &mockSender{
		result: &transport.SendResult{StatusCode: 200, Accepted: 0},
	}
}

func (s *mockSender) SendBatch(ctx context.Context, events []string) (*transport.SendResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastBatch = make([]string, len(events))
	copy(s.lastBatch, events)
	s.result.Accepted = len(events)

	if s.err != nil {
		return nil, s.err
	}

	return s.result, nil
}

func (s *mockSender) getCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *mockSender) getLastBatch() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastBatch == nil {
		return nil
	}
	result := make([]string, len(s.lastBatch))
	copy(result, s.lastBatch)
	return result
}

// --- Tests ---

func TestNewBatcher_EnforcesMinimums(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()

	b := NewBatcher(q, s, 1, 1*time.Second)
	if b.batchSize != 5 {
		t.Errorf("batchSize: got %d, want 5 (minimum)", b.batchSize)
	}
	if b.flushInterval != 5*time.Second {
		t.Errorf("flushInterval: got %v, want 5s (minimum)", b.flushInterval)
	}
}

func TestNewBatcher_AcceptsValidValues(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()

	b := NewBatcher(q, s, 50, 30*time.Second)
	if b.batchSize != 50 {
		t.Errorf("batchSize: got %d, want 50", b.batchSize)
	}
	if b.flushInterval != 30*time.Second {
		t.Errorf("flushInterval: got %v, want 30s", b.flushInterval)
	}
}

func TestAdd_EnqueuesEvent(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	b := NewBatcher(q, s, 100, 1*time.Minute) // Large batch size so no auto-flush

	err := b.Add(`{"type":"test"}`, "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := q.getEvents()
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].EventJSON != `{"type":"test"}` {
		t.Errorf("EventJSON: got %q", events[0].EventJSON)
	}
	if events[0].IdempotencyKey != "key-1" {
		t.Errorf("IdempotencyKey: got %q", events[0].IdempotencyKey)
	}
}

func TestAdd_ReturnsEnqueueError(t *testing.T) {
	q := newMockQueue()
	q.enqueueErr = fmt.Errorf("disk full")
	s := newMockSender()
	b := NewBatcher(q, s, 100, 1*time.Minute)

	err := b.Add(`{"type":"test"}`, "key-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAdd_TriggersFlushAtBatchSize(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	batchSize := 5
	b := NewBatcher(q, s, batchSize, 1*time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.StartFlushLoop(ctx)

	// Add batchSize events
	for i := 0; i < batchSize; i++ {
		err := b.Add(fmt.Sprintf(`{"type":"test","n":%d}`, i), fmt.Sprintf("key-%d", i))
		if err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}

	// Wait for the flush to be triggered and processed
	time.Sleep(100 * time.Millisecond)

	if calls := s.getCalls(); calls < 1 {
		t.Errorf("expected at least 1 SendBatch call after reaching batch size, got %d", calls)
	}

	// Events should be deleted from queue after successful send
	remaining := q.getEvents()
	if len(remaining) != 0 {
		t.Errorf("remaining events: got %d, want 0 (should be deleted after send)", len(remaining))
	}

	cancel()
	// Wait for loop to finish
	<-b.doneCh
}

func TestFlush_SendsAndDeletes(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	b := NewBatcher(q, s, 100, 1*time.Minute)

	// Enqueue some events directly
	q.Enqueue(`{"type":"e1"}`, "k1")
	q.Enqueue(`{"type":"e2"}`, "k2")
	q.Enqueue(`{"type":"e3"}`, "k3")

	err := b.Flush(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sender was called with correct events
	if calls := s.getCalls(); calls != 1 {
		t.Errorf("SendBatch calls: got %d, want 1", calls)
	}

	batch := s.getLastBatch()
	if len(batch) != 3 {
		t.Fatalf("batch size: got %d, want 3", len(batch))
	}
	if batch[0] != `{"type":"e1"}` {
		t.Errorf("batch[0]: got %q", batch[0])
	}

	// Verify events deleted from queue
	remaining := q.getEvents()
	if len(remaining) != 0 {
		t.Errorf("remaining events: got %d, want 0", len(remaining))
	}

	q.mu.Lock()
	if q.deleteCalls != 1 {
		t.Errorf("delete calls: got %d, want 1", q.deleteCalls)
	}
	q.mu.Unlock()
}

func TestFlush_EmptyQueue(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	b := NewBatcher(q, s, 100, 1*time.Minute)

	err := b.Flush(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := s.getCalls(); calls != 0 {
		t.Errorf("SendBatch should not be called on empty queue, got %d calls", calls)
	}
}

func TestFlush_KeepsFailedEvents(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	s.err = fmt.Errorf("network error")
	b := NewBatcher(q, s, 100, 1*time.Minute)

	// Enqueue events
	q.Enqueue(`{"type":"e1"}`, "k1")
	q.Enqueue(`{"type":"e2"}`, "k2")

	err := b.Flush(context.Background())
	if err == nil {
		t.Fatal("expected error from failed send")
	}

	// Events should still be in queue
	remaining := q.getEvents()
	if len(remaining) != 2 {
		t.Errorf("remaining events: got %d, want 2 (failed events should stay)", len(remaining))
	}

	// Delete should NOT have been called
	q.mu.Lock()
	if q.deleteCalls != 0 {
		t.Errorf("delete calls: got %d, want 0 (no delete on failure)", q.deleteCalls)
	}

	// MarkRetry should have been called for each event
	if q.retryCalls != 2 {
		t.Errorf("retry calls: got %d, want 2", q.retryCalls)
	}
	q.mu.Unlock()

	// Verify retry count incremented
	for _, e := range remaining {
		if e.RetryCount != 1 {
			t.Errorf("event %d retry count: got %d, want 1", e.ID, e.RetryCount)
		}
	}
}

func TestFlush_DequeueError(t *testing.T) {
	q := newMockQueue()
	q.dequeueErr = fmt.Errorf("db error")
	s := newMockSender()
	b := NewBatcher(q, s, 100, 1*time.Minute)

	err := b.Flush(context.Background())
	if err == nil {
		t.Fatal("expected error from dequeue failure")
	}
}

func TestFlushLoop_PeriodicFlush(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	// Use minimum flush interval (5s is too slow for tests)
	// We'll use a short interval by creating the batcher manually
	b := &Batcher{
		queue:         q,
		sender:        s,
		batchSize:     100,
		flushInterval: 50 * time.Millisecond, // Short for testing
		lastFlush:     time.Now(),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	// Enqueue an event
	q.Enqueue(`{"type":"periodic"}`, "k-periodic")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.StartFlushLoop(ctx)

	// Wait for at least one periodic flush
	time.Sleep(200 * time.Millisecond)

	if calls := s.getCalls(); calls < 1 {
		t.Errorf("expected at least 1 periodic flush, got %d", calls)
	}

	cancel()
	<-b.doneCh
}

func TestStop_WaitsForCompletion(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	b := &Batcher{
		queue:         q,
		sender:        s,
		batchSize:     100,
		flushInterval: 1 * time.Hour, // Long interval: won't trigger
		lastFlush:     time.Now(),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	// Enqueue events that should be flushed on stop
	q.Enqueue(`{"type":"final"}`, "k-final")

	ctx := context.Background()
	b.StartFlushLoop(ctx)

	// Stop should trigger final flush and wait
	done := make(chan struct{})
	go func() {
		b.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good: Stop returned
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within timeout")
	}

	// Final flush should have sent the event
	if calls := s.getCalls(); calls != 1 {
		t.Errorf("expected 1 final flush call, got %d", calls)
	}

	remaining := q.getEvents()
	if len(remaining) != 0 {
		t.Errorf("remaining events: got %d, want 0 (final flush should send)", len(remaining))
	}
}

func TestStop_FinalFlushError(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	s.err = fmt.Errorf("server down")

	var errorCaptured error
	var errorMu sync.Mutex

	b := &Batcher{
		queue:         q,
		sender:        s,
		batchSize:     100,
		flushInterval: 1 * time.Hour,
		lastFlush:     time.Now(),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		onError: func(err error) {
			errorMu.Lock()
			errorCaptured = err
			errorMu.Unlock()
		},
	}

	q.Enqueue(`{"type":"fail"}`, "k-fail")

	ctx := context.Background()
	b.StartFlushLoop(ctx)

	b.Stop()

	errorMu.Lock()
	if errorCaptured == nil {
		t.Error("expected onError to be called on final flush failure")
	}
	errorMu.Unlock()

	// Events should remain in queue (failed to send)
	remaining := q.getEvents()
	if len(remaining) != 1 {
		t.Errorf("remaining events: got %d, want 1 (send failed)", len(remaining))
	}
}

func TestSetOnError(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	b := NewBatcher(q, s, 100, 1*time.Minute)

	called := false
	b.SetOnError(func(err error) {
		called = true
	})

	if b.onError == nil {
		t.Fatal("onError should be set")
	}

	b.onError(fmt.Errorf("test"))
	if !called {
		t.Error("onError callback was not called")
	}
}

func TestFlush_BatchSizeLimitsDequeue(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	batchSize := 5
	b := NewBatcher(q, s, batchSize, 1*time.Minute)

	// Enqueue more events than batch size
	for i := 0; i < 10; i++ {
		q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("k-%d", i))
	}

	err := b.Flush(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have sent exactly batchSize events
	batch := s.getLastBatch()
	if len(batch) != batchSize {
		t.Errorf("batch size: got %d, want %d", len(batch), batchSize)
	}

	// Remaining events should be 10 - 5 = 5
	remaining := q.getEvents()
	if len(remaining) != 5 {
		t.Errorf("remaining events: got %d, want 5", len(remaining))
	}
}

func TestContextCancellation_StopsFlushLoop(t *testing.T) {
	q := newMockQueue()
	s := newMockSender()
	b := &Batcher{
		queue:         q,
		sender:        s,
		batchSize:     100,
		flushInterval: 1 * time.Hour,
		lastFlush:     time.Now(),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	b.StartFlushLoop(ctx)

	// Cancel context
	cancel()

	// doneCh should close
	select {
	case <-b.doneCh:
		// Good: loop exited
	case <-time.After(2 * time.Second):
		t.Fatal("flush loop did not exit after context cancellation")
	}
}
