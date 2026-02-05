package causality

import (
	"context"
	"sync"
	"time"
)

// batcher handles event batching with size and time-based triggers.
type batcher struct {
	mu        sync.Mutex
	events    []Event
	batchSize int
	transport *httpTransport
}

// newBatcher creates a new batcher with the given configuration and transport.
func newBatcher(batchSize int, transport *httpTransport) *batcher {
	return &batcher{
		events:    make([]Event, 0, batchSize),
		batchSize: batchSize,
		transport: transport,
	}
}

// add adds an event to the batch.
// Returns true if the batch is full and should be flushed.
func (b *batcher) add(event Event) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = append(b.events, event)
	return len(b.events) >= b.batchSize
}

// drain atomically swaps and returns all buffered events.
// The internal buffer is reset to empty.
func (b *batcher) drain() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.events) == 0 {
		return nil
	}

	events := b.events
	b.events = make([]Event, 0, b.batchSize)
	return events
}

// flush sends all queued events to the server.
// Returns nil on success or empty batch.
func (b *batcher) flush(ctx context.Context) error {
	events := b.drain()
	if len(events) == 0 {
		return nil
	}
	return b.transport.sendBatch(ctx, events)
}

// flushLoop runs a timer-based flush loop that periodically sends events.
// It stops when the context is canceled.
// Signals completion via the done channel when the loop exits.
func (b *batcher) flushLoop(ctx context.Context, interval time.Duration, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Ignore errors in background flush - events will be retried on next flush
			_ = b.flush(ctx)
		}
	}
}
