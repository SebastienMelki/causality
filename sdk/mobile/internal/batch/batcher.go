// Package batch provides event batching with dual triggers (count + time)
// for the Causality mobile SDK. Events are persisted to the storage queue
// first, then batched for sending via the transport layer.
package batch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SebastienMelki/causality/sdk/mobile/internal/storage"
	"github.com/SebastienMelki/causality/sdk/mobile/internal/transport"
)

// EventQueue is the interface for the persistent event storage queue.
// It abstracts storage.Queue to enable unit testing with mocks.
type EventQueue interface {
	Enqueue(eventJSON string, idempotencyKey string) error
	DequeueBatch(n int) ([]storage.QueuedEvent, error)
	Delete(ids []int64) error
	MarkRetry(id int64) error
	Count() (int, error)
}

// EventSender is the interface for the HTTP transport client.
// It abstracts transport.Client to enable unit testing with mocks.
type EventSender interface {
	SendBatch(ctx context.Context, events []string) (*transport.SendResult, error)
}

// Batcher batches events by count and time, whichever trigger fires first.
// Events are enqueued to the persistent queue immediately, then dequeued
// and sent in batches. Failed events remain in the queue for retry.
type Batcher struct {
	queue         EventQueue
	sender        EventSender
	batchSize     int
	flushInterval time.Duration

	mu           sync.Mutex
	pendingCount int
	lastFlush    time.Time

	flushCh chan struct{} // signals an async flush request
	stopCh  chan struct{} // signals stop
	doneCh  chan struct{} // closed when flush loop exits

	onError func(err error) // optional error callback
}

// NewBatcher creates a new Batcher that batches events by count and time.
//
// batchSize is the number of events that triggers a flush (minimum 5).
// flushInterval is the time between periodic flushes (minimum 5s).
func NewBatcher(queue EventQueue, sender EventSender, batchSize int, flushInterval time.Duration) *Batcher {
	if batchSize < 5 {
		batchSize = 5
	}
	if flushInterval < 5*time.Second {
		flushInterval = 5 * time.Second
	}

	return &Batcher{
		queue:         queue,
		sender:        sender,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		lastFlush:     time.Now(),
		flushCh:       make(chan struct{}, 1), // buffered so Add never blocks
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
}

// SetOnError sets an optional error callback that is called when a flush fails.
func (b *Batcher) SetOnError(fn func(err error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onError = fn
}

// Add enqueues an event to the persistent queue and checks if a
// batch-size flush should be triggered. This method is non-blocking.
func (b *Batcher) Add(eventJSON, idempotencyKey string) error {
	if err := b.queue.Enqueue(eventJSON, idempotencyKey); err != nil {
		return fmt.Errorf("enqueue event: %w", err)
	}

	b.mu.Lock()
	b.pendingCount++
	shouldFlush := b.pendingCount >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		// Non-blocking send to flush channel
		select {
		case b.flushCh <- struct{}{}:
		default:
			// Flush already pending, skip
		}
	}

	return nil
}

// Flush dequeues events from the persistent queue and sends them.
// On success, events are deleted from the queue.
// On failure, events are marked for retry and remain in the queue.
func (b *Batcher) Flush(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.flushLocked(ctx)
}

// flushLocked performs the actual flush. Caller must hold b.mu.
func (b *Batcher) flushLocked(ctx context.Context) error {
	events, err := b.queue.DequeueBatch(b.batchSize)
	if err != nil {
		return fmt.Errorf("dequeue batch: %w", err)
	}

	if len(events) == 0 {
		b.pendingCount = 0
		b.lastFlush = time.Now()
		return nil
	}

	// Extract JSON payloads
	payloads := make([]string, len(events))
	for i, e := range events {
		payloads[i] = e.EventJSON
	}

	// Send batch
	_, sendErr := b.sender.SendBatch(ctx, payloads)
	if sendErr != nil {
		// Mark each event for retry (increment retry_count)
		for _, e := range events {
			if markErr := b.queue.MarkRetry(e.ID); markErr != nil {
				// Log but don't fail: event stays in queue either way
				if b.onError != nil {
					b.onError(fmt.Errorf("mark retry for event %d: %w", e.ID, markErr))
				}
			}
		}

		b.lastFlush = time.Now()
		return fmt.Errorf("send batch: %w", sendErr)
	}

	// Delete successfully sent events
	ids := make([]int64, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}

	if delErr := b.queue.Delete(ids); delErr != nil {
		return fmt.Errorf("delete sent events: %w", delErr)
	}

	b.pendingCount = 0
	b.lastFlush = time.Now()

	return nil
}

// Stop signals the flush loop to stop and waits for it to exit.
// It performs a final flush attempt before returning.
func (b *Batcher) Stop() {
	close(b.stopCh)
	<-b.doneCh
}
