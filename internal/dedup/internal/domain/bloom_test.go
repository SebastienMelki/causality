// Package domain tests the sliding window bloom filter implementation.
package domain

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBloomFilterSet_FirstOccurrence(t *testing.T) {
	bf := NewBloomFilterSet(10*time.Minute, 10000, 0.0001)

	key := "unique-event-key-12345"
	isDup := bf.IsDuplicate(key)

	if isDup {
		t.Error("IsDuplicate() = true for first occurrence, want false")
	}
}

func TestBloomFilterSet_SecondOccurrence(t *testing.T) {
	bf := NewBloomFilterSet(10*time.Minute, 10000, 0.0001)

	key := "duplicate-event-key-12345"

	// First call should return false (not a duplicate)
	if bf.IsDuplicate(key) {
		t.Error("First call: IsDuplicate() = true, want false")
	}

	// Second call should return true (is a duplicate)
	if !bf.IsDuplicate(key) {
		t.Error("Second call: IsDuplicate() = false, want true")
	}
}

func TestBloomFilterSet_DifferentKeys(t *testing.T) {
	bf := NewBloomFilterSet(10*time.Minute, 10000, 0.0001)

	key1 := "event-key-alpha"
	key2 := "event-key-beta"

	// Both should return false on first occurrence
	if bf.IsDuplicate(key1) {
		t.Error("IsDuplicate(key1) = true for first occurrence, want false")
	}
	if bf.IsDuplicate(key2) {
		t.Error("IsDuplicate(key2) = true for first occurrence, want false")
	}

	// Now they should be duplicates
	if !bf.IsDuplicate(key1) {
		t.Error("IsDuplicate(key1) = false on second call, want true")
	}
	if !bf.IsDuplicate(key2) {
		t.Error("IsDuplicate(key2) = false on second call, want true")
	}
}

func TestBloomFilterSet_Rotate_PreservesCurrentInPrevious(t *testing.T) {
	bf := NewBloomFilterSet(10*time.Minute, 10000, 0.0001)

	// Add a key before rotation
	key := "pre-rotation-key"
	bf.IsDuplicate(key) // adds to current filter

	// Rotate: current becomes previous, new empty current is created
	bf.Rotate()

	// Key should still be found (now in previous filter)
	if !bf.IsDuplicate(key) {
		t.Error("After rotation, key should still be found in previous filter")
	}
}

func TestBloomFilterSet_DoubleRotate_ExpiresPrevious(t *testing.T) {
	bf := NewBloomFilterSet(10*time.Minute, 10000, 0.0001)

	// Add a key to current filter
	oldKey := "old-key-to-expire"
	bf.IsDuplicate(oldKey)

	// First rotation: oldKey moves from current to previous
	bf.Rotate()

	// Add a new key after first rotation
	newKey := "new-key-after-rotation"
	bf.IsDuplicate(newKey)

	// Second rotation: previous is discarded, current (with newKey) becomes previous
	bf.Rotate()

	// oldKey should be gone (was in previous, now discarded)
	// Note: Due to bloom filter false positives, we can only check that
	// the key is likely not found. With low FP rate it should be false.
	isDup := bf.IsDuplicate(oldKey)
	// After double rotation, oldKey should not be found (returns false = not duplicate)
	if isDup {
		// This could be a false positive, but with FP rate 0.0001 it's very unlikely
		t.Error("After double rotation, old key should be expired (not found)")
	}

	// newKey should still be found (in previous after second rotation)
	if !bf.IsDuplicate(newKey) {
		t.Error("After double rotation, key from first rotation should still be in previous")
	}
}

func TestBloomFilterSet_ConcurrentAccess(t *testing.T) {
	bf := NewBloomFilterSet(10*time.Minute, 100000, 0.0001)

	const goroutines = 100
	const keysPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range keysPerGoroutine {
				key := string(rune('A'+id)) + "-" + string(rune('0'+j%10))
				bf.IsDuplicate(key)
			}
		}(i)
	}

	// Add some goroutines that rotate during concurrent access
	wg.Add(5)
	for range 5 {
		go func() {
			defer wg.Done()
			for range 10 {
				bf.Rotate()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()
	// Test passes if no panic or race is detected
}

func TestBloomFilterSet_Window(t *testing.T) {
	window := 15 * time.Minute
	bf := NewBloomFilterSet(window, 10000, 0.0001)

	if bf.Window() != window {
		t.Errorf("Window() = %v, want %v", bf.Window(), window)
	}
}

func TestBloomFilterSet_EmptyKey(t *testing.T) {
	bf := NewBloomFilterSet(10*time.Minute, 10000, 0.0001)

	// Empty key should behave like any other key
	emptyKey := ""
	first := bf.IsDuplicate(emptyKey)
	second := bf.IsDuplicate(emptyKey)

	if first {
		t.Error("Empty key first check should return false")
	}
	if !second {
		t.Error("Empty key second check should return true (duplicate)")
	}
}

func TestBloomFilterSet_FalsePositiveRate(t *testing.T) {
	// This test verifies that the false positive rate is approximately as configured
	bf := NewBloomFilterSet(10*time.Minute, 10000, 0.01) // 1% FP rate

	// Add 5000 unique keys with a distinct prefix
	for i := range 5000 {
		key := fmt.Sprintf("added-key-%d", i)
		bf.IsDuplicate(key)
	}

	// Test 1000 keys that were never added (using different prefix and range)
	falsePositives := 0
	for i := range 1000 {
		key := fmt.Sprintf("never-added-key-%d", i+100000) // offset to avoid any overlap
		if bf.IsDuplicate(key) {
			falsePositives++
		}
	}

	// With 1% FP rate, we expect roughly 10 false positives from 1000 tests
	// Allow for statistical variance - anything under 5% is acceptable
	fpRate := float64(falsePositives) / 1000.0
	if fpRate > 0.05 {
		t.Errorf("False positive rate too high: %.2f%% (expected ~1%%)", fpRate*100)
	}
}
