package storage

import (
	"fmt"
	"strings"
	"time"
)

// QueuedEvent represents an event stored in the persistent queue.
type QueuedEvent struct {
	// ID is the auto-incremented row identifier used for deletion after send.
	ID int64

	// EventJSON is the serialized event payload.
	EventJSON string

	// IdempotencyKey is the unique deduplication key for this event.
	IdempotencyKey string

	// CreatedAt is the Unix millisecond timestamp when the event was enqueued.
	CreatedAt int64

	// RetryCount tracks how many times delivery has been attempted.
	RetryCount int
}

// Queue provides a FIFO persistent event queue backed by SQLite.
// When the queue reaches maxSize, the oldest events are evicted to make room.
type Queue struct {
	db      *DB
	maxSize int
}

// NewQueue creates a new Queue with the given DB and maximum size.
// maxSize must be > 0; if not, it defaults to 1000.
func NewQueue(db *DB, maxSize int) *Queue {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &Queue{
		db:      db,
		maxSize: maxSize,
	}
}

// Enqueue adds an event to the queue. If the queue is at capacity, the oldest
// event(s) are evicted to make room. Duplicate idempotency keys are silently
// ignored (no error returned).
func (q *Queue) Enqueue(eventJSON string, idempotencyKey string) error {
	// Evict oldest events if at or above capacity.
	count, err := q.Count()
	if err != nil {
		return fmt.Errorf("count events: %w", err)
	}

	if count >= q.maxSize {
		evictCount := count - q.maxSize + 1
		if err := q.evictOldest(evictCount); err != nil {
			return fmt.Errorf("evict oldest: %w", err)
		}
	}

	now := time.Now().UnixMilli()

	// INSERT OR IGNORE handles duplicate idempotency keys gracefully.
	_, err = q.db.Exec(
		`INSERT OR IGNORE INTO events (event_json, idempotency_key, created_at) VALUES (?, ?, ?)`,
		eventJSON, idempotencyKey, now,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

// DequeueBatch returns up to n events in FIFO order (oldest first).
// Events are NOT removed; call Delete after successful delivery.
// Returns an empty slice (not nil) if no events are available.
func (q *Queue) DequeueBatch(n int) ([]QueuedEvent, error) {
	if n <= 0 {
		return []QueuedEvent{}, nil
	}

	rows, err := q.db.Query(
		`SELECT id, event_json, idempotency_key, created_at, retry_count
		 FROM events
		 ORDER BY created_at ASC, id ASC
		 LIMIT ?`,
		n,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []QueuedEvent
	for rows.Next() {
		var e QueuedEvent
		if err := rows.Scan(&e.ID, &e.EventJSON, &e.IdempotencyKey, &e.CreatedAt, &e.RetryCount); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	// Guarantee non-nil empty slice.
	if events == nil {
		events = []QueuedEvent{}
	}

	return events, nil
}

// Delete removes events by their IDs. Call this after successful delivery.
func (q *Queue) Delete(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// Build placeholder list for IN clause.
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("DELETE FROM events WHERE id IN (%s)", strings.Join(placeholders, ","))
	_, err := q.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("delete events: %w", err)
	}

	return nil
}

// MarkRetry increments the retry count and updates last_retry_at for an event.
func (q *Queue) MarkRetry(id int64) error {
	now := time.Now().UnixMilli()
	result, err := q.db.Exec(
		`UPDATE events SET retry_count = retry_count + 1, last_retry_at = ? WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("mark retry: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("event %d not found", id)
	}

	return nil
}

// Count returns the number of events currently in the queue.
func (q *Queue) Count() (int, error) {
	var count int
	err := q.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count events: %w", err)
	}
	return count, nil
}

// Clear removes all events from the queue. Used for ResetAll.
func (q *Queue) Clear() error {
	_, err := q.db.Exec("DELETE FROM events")
	if err != nil {
		return fmt.Errorf("clear events: %w", err)
	}
	return nil
}

// evictOldest removes the n oldest events from the queue.
func (q *Queue) evictOldest(n int) error {
	_, err := q.db.Exec(
		`DELETE FROM events WHERE id IN (
			SELECT id FROM events ORDER BY created_at ASC, id ASC LIMIT ?
		)`,
		n,
	)
	if err != nil {
		return fmt.Errorf("evict oldest: %w", err)
	}
	return nil
}
