package batch

import (
	"context"
	"time"
)

// StartFlushLoop runs the periodic flush loop in a background goroutine.
// The loop flushes on either:
//   - The flushInterval ticker (time-based trigger)
//   - A signal from Add when batch size is reached (count-based trigger)
//
// The loop exits when Stop() is called or the context is canceled.
// It performs a final flush attempt before exiting on stop.
func (b *Batcher) StartFlushLoop(ctx context.Context) {
	go b.runFlushLoop(ctx)
}

// runFlushLoop is the internal flush loop goroutine.
func (b *Batcher) runFlushLoop(ctx context.Context) {
	defer close(b.doneCh)

	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Time-based flush trigger
			b.mu.Lock()
			if err := b.flushLocked(ctx); err != nil {
				if b.onError != nil {
					b.onError(err)
				}
			}
			b.mu.Unlock()

		case <-b.flushCh:
			// Count-based flush trigger (batch size reached)
			b.mu.Lock()
			if err := b.flushLocked(ctx); err != nil {
				if b.onError != nil {
					b.onError(err)
				}
			}
			b.mu.Unlock()

		case <-b.stopCh:
			// Final flush before exit
			b.mu.Lock()
			if err := b.flushLocked(ctx); err != nil {
				if b.onError != nil {
					b.onError(err)
				}
			}
			b.mu.Unlock()
			return

		case <-ctx.Done():
			// Context canceled, exit without flushing
			return
		}
	}
}
