package storage

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// newTestQueue creates a Queue with a temporary database for testing.
func newTestQueue(t *testing.T, maxSize int) (*Queue, *DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewQueue(db, maxSize), db
}

func TestEnqueue_Success(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	err := q.Enqueue(`{"type":"screen_view"}`, "key-1")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	events, err := q.DequeueBatch(10)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventJSON != `{"type":"screen_view"}` {
		t.Fatalf("unexpected event JSON: %s", events[0].EventJSON)
	}
	if events[0].IdempotencyKey != "key-1" {
		t.Fatalf("unexpected idempotency key: %s", events[0].IdempotencyKey)
	}
	if events[0].RetryCount != 0 {
		t.Fatalf("expected retry count 0, got %d", events[0].RetryCount)
	}
	if events[0].ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if events[0].CreatedAt == 0 {
		t.Fatal("expected non-zero created_at")
	}
}

func TestEnqueue_Eviction(t *testing.T) {
	maxSize := 5
	q, _ := newTestQueue(t, maxSize)

	// Fill the queue to capacity.
	for i := 0; i < maxSize; i++ {
		key := fmt.Sprintf("key-%d", i)
		err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), key)
		if err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
		// Small delay to ensure distinct created_at timestamps.
		time.Sleep(time.Millisecond)
	}

	count, err := q.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != maxSize {
		t.Fatalf("expected %d events, got %d", maxSize, count)
	}

	// Enqueue one more; should evict the oldest.
	err = q.Enqueue(`{"n":5}`, "key-5")
	if err != nil {
		t.Fatalf("Enqueue overflow: %v", err)
	}

	count, err = q.Count()
	if err != nil {
		t.Fatalf("Count after eviction: %v", err)
	}
	if count != maxSize {
		t.Fatalf("expected %d events after eviction, got %d", maxSize, count)
	}

	// Verify oldest (key-0) was evicted and key-1 is now first.
	events, err := q.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	if events[0].IdempotencyKey != "key-1" {
		t.Fatalf("expected key-1 as oldest, got %s", events[0].IdempotencyKey)
	}
}

func TestEnqueue_DuplicateIdempotencyKey(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	err := q.Enqueue(`{"type":"first"}`, "dup-key")
	if err != nil {
		t.Fatalf("first Enqueue: %v", err)
	}

	// Duplicate key should not error.
	err = q.Enqueue(`{"type":"second"}`, "dup-key")
	if err != nil {
		t.Fatalf("duplicate Enqueue should not error: %v", err)
	}

	// Should still have only one event.
	count, err := q.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 event (duplicate ignored), got %d", count)
	}

	// The original event should be preserved.
	events, err := q.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	if events[0].EventJSON != `{"type":"first"}` {
		t.Fatalf("expected original event preserved, got: %s", events[0].EventJSON)
	}
}

func TestDequeueBatch_FIFO(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	// Enqueue events in order.
	for i := 0; i < 5; i++ {
		err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("fifo-key-%d", i))
		if err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	events, err := q.DequeueBatch(5)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	// Verify FIFO order.
	for i, e := range events {
		expected := fmt.Sprintf(`{"n":%d}`, i)
		if e.EventJSON != expected {
			t.Fatalf("event %d: expected %s, got %s", i, expected, e.EventJSON)
		}
	}
}

func TestDequeueBatch_Empty(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	events, err := q.DequeueBatch(10)
	if err != nil {
		t.Fatalf("DequeueBatch on empty queue: %v", err)
	}
	if events == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestDequeueBatch_ZeroLimit(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	events, err := q.DequeueBatch(0)
	if err != nil {
		t.Fatalf("DequeueBatch(0): %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events for limit 0, got %d", len(events))
	}
}

func TestDequeueBatch_NegativeLimit(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	events, err := q.DequeueBatch(-1)
	if err != nil {
		t.Fatalf("DequeueBatch(-1): %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events for negative limit, got %d", len(events))
	}
}

func TestDequeueBatch_PartialBatch(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	// Enqueue 3 events, request 10.
	for i := 0; i < 3; i++ {
		if err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("partial-%d", i)); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	events, err := q.DequeueBatch(10)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
}

func TestDelete_RemovesEvents(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	for i := 0; i < 3; i++ {
		if err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("del-key-%d", i)); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	events, err := q.DequeueBatch(3)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}

	// Delete the first two events.
	err = q.Delete([]int64{events[0].ID, events[1].ID})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Only the third event should remain.
	remaining, err := q.DequeueBatch(10)
	if err != nil {
		t.Fatalf("DequeueBatch after delete: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining event, got %d", len(remaining))
	}
	if remaining[0].ID != events[2].ID {
		t.Fatalf("expected event %d to remain, got %d", events[2].ID, remaining[0].ID)
	}
}

func TestDelete_EmptyIDs(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	// Should not error with empty slice.
	err := q.Delete(nil)
	if err != nil {
		t.Fatalf("Delete(nil): %v", err)
	}

	err = q.Delete([]int64{})
	if err != nil {
		t.Fatalf("Delete(empty): %v", err)
	}
}

func TestMarkRetry_IncrementsCount(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	if err := q.Enqueue(`{"type":"retry"}`, "retry-key"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	events, err := q.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	if events[0].RetryCount != 0 {
		t.Fatalf("expected initial retry count 0, got %d", events[0].RetryCount)
	}

	// Mark retry twice.
	for i := 0; i < 2; i++ {
		if err := q.MarkRetry(events[0].ID); err != nil {
			t.Fatalf("MarkRetry %d: %v", i+1, err)
		}
	}

	updated, err := q.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch after retry: %v", err)
	}
	if updated[0].RetryCount != 2 {
		t.Fatalf("expected retry count 2, got %d", updated[0].RetryCount)
	}
}

func TestMarkRetry_NotFound(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	err := q.MarkRetry(99999)
	if err == nil {
		t.Fatal("expected error for non-existent event")
	}
}

func TestCount_Accurate(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	// Empty queue.
	count, err := q.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}

	// Add events.
	for i := 0; i < 7; i++ {
		if err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("count-key-%d", i)); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	count, err = q.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 7 {
		t.Fatalf("expected count 7, got %d", count)
	}

	// Delete some.
	events, err := q.DequeueBatch(3)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	ids := make([]int64, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}
	if err := q.Delete(ids); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	count, err = q.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected count 4, got %d", count)
	}
}

func TestClear_RemovesAllEvents(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	for i := 0; i < 5; i++ {
		if err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("clear-key-%d", i)); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	if err := q.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	count, err := q.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 after clear, got %d", count)
	}
}

func TestNewQueue_DefaultMaxSize(t *testing.T) {
	q, _ := newTestQueue(t, 0)
	if q.maxSize != 1000 {
		t.Fatalf("expected default maxSize 1000, got %d", q.maxSize)
	}
}

func TestNewQueue_NegativeMaxSize(t *testing.T) {
	q, _ := newTestQueue(t, -1)
	if q.maxSize != 1000 {
		t.Fatalf("expected default maxSize 1000 for negative input, got %d", q.maxSize)
	}
}

func TestEnqueue_MultipleEvictions(t *testing.T) {
	maxSize := 3
	q, _ := newTestQueue(t, maxSize)

	// Fill to capacity.
	for i := 0; i < maxSize; i++ {
		if err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("multi-key-%d", i)); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	// Add 3 more events, causing 3 evictions.
	for i := maxSize; i < maxSize+3; i++ {
		if err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("multi-key-%d", i)); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	count, err := q.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != maxSize {
		t.Fatalf("expected %d events, got %d", maxSize, count)
	}

	// Oldest should be the event with n=3 (0, 1, 2 evicted).
	events, err := q.DequeueBatch(maxSize)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	if events[0].EventJSON != `{"n":3}` {
		t.Fatalf("expected oldest event {\"n\":3}, got %s", events[0].EventJSON)
	}
}

func TestMarkRetry_UpdatesLastRetryAt(t *testing.T) {
	q, _ := newTestQueue(t, 100)

	if err := q.Enqueue(`{"type":"retry-ts"}`, "retry-ts-key"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	events, err := q.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}

	if err := q.MarkRetry(events[0].ID); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}

	// Verify last_retry_at was updated.
	var lastRetryAt int64
	err = q.db.QueryRow("SELECT last_retry_at FROM events WHERE id = ?", events[0].ID).Scan(&lastRetryAt)
	if err != nil {
		t.Fatalf("query last_retry_at: %v", err)
	}
	if lastRetryAt == 0 {
		t.Fatal("expected non-zero last_retry_at after MarkRetry")
	}
}

func TestEnqueue_NoEvictionWhenBelowCapacity(t *testing.T) {
	q, _ := newTestQueue(t, 10)

	// Enqueue less than maxSize; no eviction should happen.
	for i := 0; i < 5; i++ {
		if err := q.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("below-key-%d", i)); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	count, err := q.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5, got %d", count)
	}

	// Verify all events present in order.
	events, err := q.DequeueBatch(5)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	for i, e := range events {
		expected := fmt.Sprintf("below-key-%d", i)
		if e.IdempotencyKey != expected {
			t.Fatalf("event %d: expected key %s, got %s", i, expected, e.IdempotencyKey)
		}
	}
}

func TestQueue_ErrorAfterClose(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	q := NewQueue(db, 100)

	// Close the database, then operations should fail.
	db.Close()

	_, err = q.Count()
	if err == nil {
		t.Fatal("expected error on Count after close")
	}

	err = q.Enqueue(`{"type":"fail"}`, "fail-key")
	if err == nil {
		t.Fatal("expected error on Enqueue after close")
	}

	_, err = q.DequeueBatch(1)
	if err == nil {
		t.Fatal("expected error on DequeueBatch after close")
	}

	err = q.Clear()
	if err == nil {
		t.Fatal("expected error on Clear after close")
	}

	err = q.Delete([]int64{1})
	if err == nil {
		t.Fatal("expected error on Delete after close")
	}

	err = q.MarkRetry(1)
	if err == nil {
		t.Fatal("expected error on MarkRetry after close")
	}
}

func TestQueue_PersistsSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	// Open, enqueue, close.
	db1, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("first NewDB: %v", err)
	}
	q1 := NewQueue(db1, 100)

	for i := 0; i < 3; i++ {
		if err := q1.Enqueue(fmt.Sprintf(`{"n":%d}`, i), fmt.Sprintf("persist-key-%d", i)); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}
	db1.Close()

	// Reopen and verify events survived.
	db2, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("second NewDB: %v", err)
	}
	defer db2.Close()
	q2 := NewQueue(db2, 100)

	count, err := q2.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 events after reopen, got %d", count)
	}

	events, err := q2.DequeueBatch(3)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	for i, e := range events {
		expected := fmt.Sprintf(`{"n":%d}`, i)
		if e.EventJSON != expected {
			t.Fatalf("event %d: expected %s, got %s", i, expected, e.EventJSON)
		}
	}
}
