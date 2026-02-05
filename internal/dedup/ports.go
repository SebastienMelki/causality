// Package dedup provides server-side event deduplication using a sliding
// window bloom filter. Events with duplicate idempotency keys are silently
// dropped while non-duplicate events pass through unchanged.
package dedup

import "context"

// Deduplicator checks whether an event's idempotency key has been seen
// within the configured time window. Implementations must be safe for
// concurrent use.
type Deduplicator interface {
	// IsDuplicate returns true if the given key was already seen within
	// the sliding window. An empty key always returns false (events
	// without idempotency keys are never treated as duplicates).
	IsDuplicate(key string) bool

	// Start begins the background bloom filter rotation goroutine.
	// The goroutine stops when ctx is cancelled or Stop is called.
	Start(ctx context.Context)

	// Stop signals the rotation goroutine to stop and waits for it
	// to finish.
	Stop()
}
