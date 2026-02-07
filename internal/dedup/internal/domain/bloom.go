// Package domain contains the core bloom filter data structure for event
// deduplication. The BloomFilterSet uses a sliding window approach with two
// bloom filters (current and previous) that rotate periodically.
package domain

import (
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
)

// BloomFilterSet implements a sliding window bloom filter using two
// underlying bloom filters. Keys are always added to the "current" filter,
// while lookups check both "current" and "previous". Periodic rotation
// swaps current to previous and creates a fresh current filter, providing
// a bounded time window for deduplication.
type BloomFilterSet struct {
	current  *bloom.BloomFilter
	previous *bloom.BloomFilter
	mu       sync.RWMutex
	window   time.Duration
	capacity uint
	fpRate   float64
}

// NewBloomFilterSet creates a BloomFilterSet with the given sliding window
// duration, expected capacity (events per window), and false positive rate.
//
// Typical defaults:
//   - window: 10 minutes
//   - capacity: 1,000,000
//   - fpRate: 0.0001 (0.01%)
func NewBloomFilterSet(window time.Duration, capacity uint, fpRate float64) *BloomFilterSet {
	return &BloomFilterSet{
		current:  bloom.NewWithEstimates(capacity, fpRate),
		previous: bloom.NewWithEstimates(capacity, fpRate),
		window:   window,
		capacity: capacity,
		fpRate:   fpRate,
	}
}

// IsDuplicate checks whether the key exists in either the current or
// previous bloom filter. If found, it returns true (duplicate). If not
// found, it adds the key to the current filter and returns false.
//
// This method is safe for concurrent use.
func (b *BloomFilterSet) IsDuplicate(key string) bool {
	data := []byte(key)

	b.mu.RLock()
	if b.current.Test(data) || b.previous.Test(data) {
		b.mu.RUnlock()
		return true
	}
	b.mu.RUnlock()

	b.mu.Lock()
	// Double-check after acquiring write lock to avoid race where
	// another goroutine added the same key between RUnlock and Lock.
	if b.current.Test(data) || b.previous.Test(data) {
		b.mu.Unlock()
		return true
	}
	b.current.Add(data)
	b.mu.Unlock()

	return false
}

// Rotate swaps the current filter to previous and creates a fresh current
// filter. This should be called every window/2 to create a sliding overlap
// that ensures keys are visible for at least one full window duration.
func (b *BloomFilterSet) Rotate() {
	b.mu.Lock()
	b.previous = b.current
	b.current = bloom.NewWithEstimates(b.capacity, b.fpRate)
	b.mu.Unlock()
}

// Window returns the configured dedup window duration.
func (b *BloomFilterSet) Window() time.Duration {
	return b.window
}
