// Package service tests the dedup service which wraps the bloom filter.
package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/SebastienMelki/causality/internal/observability"
)

// createTestMetrics creates a metrics instance for testing using noop meter.
func createTestMetrics(t *testing.T) *observability.Metrics {
	t.Helper()
	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.NewMetrics(meter)
	if err != nil {
		t.Fatalf("failed to create test metrics: %v", err)
	}
	return m
}

func TestDedupService_EmptyKeyNotDuplicate(t *testing.T) {
	svc := NewDedupService(10*time.Minute, 10000, 0.0001, nil, nil)

	// Empty keys should always return false (pass through)
	if svc.IsDuplicate("") {
		t.Error("IsDuplicate(\"\") = true, want false for empty key")
	}

	// Even after calling it multiple times
	if svc.IsDuplicate("") {
		t.Error("IsDuplicate(\"\") = true on second call, want false for empty key")
	}
}

func TestDedupService_FirstEventNotDuplicate(t *testing.T) {
	svc := NewDedupService(10*time.Minute, 10000, 0.0001, nil, nil)

	key := "unique-idempotency-key-12345"
	if svc.IsDuplicate(key) {
		t.Error("IsDuplicate() = true for first occurrence, want false")
	}
}

func TestDedupService_DuplicateEventDetected(t *testing.T) {
	svc := NewDedupService(10*time.Minute, 10000, 0.0001, nil, nil)

	key := "duplicate-idempotency-key"

	// First call should return false
	if svc.IsDuplicate(key) {
		t.Error("First call: IsDuplicate() = true, want false")
	}

	// Second call should return true (duplicate)
	if !svc.IsDuplicate(key) {
		t.Error("Second call: IsDuplicate() = false, want true")
	}

	// Third call should also return true
	if !svc.IsDuplicate(key) {
		t.Error("Third call: IsDuplicate() = false, want true")
	}
}

// mockMetricCounter implements a simple counter for testing.
type mockMetricCounter struct {
	metric.Int64Counter
	count atomic.Int64
}

func (m *mockMetricCounter) Add(_ context.Context, incr int64, _ ...metric.AddOption) {
	m.count.Add(incr)
}

func TestDedupService_MetricsIncremented(t *testing.T) {
	// Create metrics with noop meter
	metrics := createTestMetrics(t)

	// Replace the DedupDropped counter with our mock
	mockCounter := &mockMetricCounter{}
	metrics.DedupDropped = mockCounter

	svc := NewDedupService(10*time.Minute, 10000, 0.0001, metrics, nil)

	key := "metrics-test-key"

	// First call - not a duplicate, metric should not increment
	svc.IsDuplicate(key)
	if mockCounter.count.Load() != 0 {
		t.Errorf("After first call, counter = %d, want 0", mockCounter.count.Load())
	}

	// Second call - duplicate, metric should increment
	svc.IsDuplicate(key)
	if mockCounter.count.Load() != 1 {
		t.Errorf("After second call (duplicate), counter = %d, want 1", mockCounter.count.Load())
	}

	// Third call - duplicate again
	svc.IsDuplicate(key)
	if mockCounter.count.Load() != 2 {
		t.Errorf("After third call (duplicate), counter = %d, want 2", mockCounter.count.Load())
	}
}

func TestDedupService_NilMetrics(t *testing.T) {
	// Service should work fine with nil metrics
	svc := NewDedupService(10*time.Minute, 10000, 0.0001, nil, nil)

	key := "nil-metrics-test"
	svc.IsDuplicate(key)
	// Duplicate should not panic with nil metrics
	svc.IsDuplicate(key)
}

func TestDedupService_StartStop(t *testing.T) {
	svc := NewDedupService(100*time.Millisecond, 10000, 0.0001, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)

	// Give it time to do at least one rotation
	time.Sleep(150 * time.Millisecond)

	// Stop should work without hanging
	done := make(chan struct{})
	go func() {
		cancel()
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Stop() took too long, may be hanging")
	}
}

func TestDedupService_RotationExpiresDuplicates(t *testing.T) {
	// Use a very short window for testing
	svc := NewDedupService(50*time.Millisecond, 10000, 0.0001, nil, nil)

	key := "rotation-test-key"

	// Add the key
	svc.IsDuplicate(key)

	// Verify it's detected as duplicate
	if !svc.IsDuplicate(key) {
		t.Error("Key should be duplicate immediately after adding")
	}

	// Start the service (this starts the rotation goroutine)
	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)

	// Wait for multiple rotations (window/2 * 3 = ~75ms for 2+ rotations)
	time.Sleep(150 * time.Millisecond)

	// After multiple rotations, the key should be expired
	// Due to the sliding window, we need 2 full rotations for complete expiry
	isDup := svc.IsDuplicate(key)

	cancel()
	svc.Stop()

	if isDup {
		t.Error("After multiple rotations, old key should be expired")
	}
}
