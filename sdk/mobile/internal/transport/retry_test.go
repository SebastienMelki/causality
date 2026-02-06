package transport

import (
	"testing"
	"time"
)

func TestExponentialBackoff_DelaysIncrease(t *testing.T) {
	eb := &ExponentialBackoff{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		MaxRetries: 5,
		Jitter:     0, // No jitter for deterministic test
	}

	prev := time.Duration(0)
	for attempt := 0; attempt < eb.MaxRetries; attempt++ {
		delay := eb.NextDelay(attempt)
		if delay <= 0 {
			t.Fatalf("attempt %d: expected positive delay, got %v", attempt, delay)
		}
		if attempt > 0 && delay <= prev {
			t.Fatalf("attempt %d: delay %v should be greater than previous %v", attempt, delay, prev)
		}
		prev = delay
	}
}

func TestExponentialBackoff_ExactDelaysWithoutJitter(t *testing.T) {
	eb := &ExponentialBackoff{
		BaseDelay:  1 * time.Second,
		MaxDelay:   1 * time.Minute,
		MaxRetries: 5,
		Jitter:     0,
	}

	expected := []time.Duration{
		1 * time.Second,  // 1 * 2^0
		2 * time.Second,  // 1 * 2^1
		4 * time.Second,  // 1 * 2^2
		8 * time.Second,  // 1 * 2^3
		16 * time.Second, // 1 * 2^4
	}

	for i, want := range expected {
		got := eb.NextDelay(i)
		if got != want {
			t.Errorf("attempt %d: got %v, want %v", i, got, want)
		}
	}
}

func TestExponentialBackoff_CapsAtMaxDelay(t *testing.T) {
	eb := &ExponentialBackoff{
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Second,
		MaxRetries: 20,
		Jitter:     0,
	}

	// Attempt 10: 1s * 2^10 = 1024s, should be capped at 5s
	delay := eb.NextDelay(10)
	if delay != 5*time.Second {
		t.Errorf("expected %v, got %v", 5*time.Second, delay)
	}

	// Verify all attempts after cap are at max
	for attempt := 5; attempt < 20; attempt++ {
		d := eb.NextDelay(attempt)
		if d > 5*time.Second {
			t.Errorf("attempt %d: delay %v exceeds max %v", attempt, d, 5*time.Second)
		}
	}
}

func TestExponentialBackoff_StopsAfterMaxRetries(t *testing.T) {
	eb := &ExponentialBackoff{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		MaxRetries: 3,
		Jitter:     0,
	}

	// Attempts 0, 1, 2 should return positive delays
	for i := 0; i < 3; i++ {
		d := eb.NextDelay(i)
		if d <= 0 {
			t.Errorf("attempt %d: expected positive delay, got %v", i, d)
		}
	}

	// Attempt 3 (== MaxRetries) should return 0
	d := eb.NextDelay(3)
	if d != 0 {
		t.Errorf("attempt 3: expected 0, got %v", d)
	}

	// Attempt 10 should also return 0
	d = eb.NextDelay(10)
	if d != 0 {
		t.Errorf("attempt 10: expected 0, got %v", d)
	}
}

func TestExponentialBackoff_JitterVariation(t *testing.T) {
	eb := &ExponentialBackoff{
		BaseDelay:  1 * time.Second,
		MaxDelay:   1 * time.Minute,
		MaxRetries: 10,
		Jitter:     0.5, // 50% jitter for visible variation
	}

	// Run 100 iterations and verify we see variation
	delays := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		d := eb.NextDelay(0) // Same attempt, should vary due to jitter
		delays[d] = true
	}

	// With 50% jitter on a 1s base delay, we should see many distinct values
	if len(delays) < 5 {
		t.Errorf("expected significant variation with 50%% jitter, got only %d distinct values", len(delays))
	}

	// All delays should be in the range [base * (1 - jitter), base * (1 + jitter)]
	minExpected := time.Duration(float64(1*time.Second) * (1 - 0.5))
	maxExpected := time.Duration(float64(1*time.Second) * (1 + 0.5))

	for d := range delays {
		if d < minExpected || d > maxExpected {
			t.Errorf("delay %v outside expected range [%v, %v]", d, minExpected, maxExpected)
		}
	}
}

func TestExponentialBackoff_ZeroJitter(t *testing.T) {
	eb := &ExponentialBackoff{
		BaseDelay:  1 * time.Second,
		MaxDelay:   1 * time.Minute,
		MaxRetries: 5,
		Jitter:     0,
	}

	// All calls with same attempt should return same delay
	first := eb.NextDelay(2)
	for i := 0; i < 10; i++ {
		d := eb.NextDelay(2)
		if d != first {
			t.Errorf("iteration %d: expected %v, got %v (should be deterministic with 0 jitter)", i, first, d)
		}
	}
}

func TestExponentialBackoff_MaxAttempts(t *testing.T) {
	eb := &ExponentialBackoff{
		BaseDelay:  1 * time.Second,
		MaxDelay:   1 * time.Minute,
		MaxRetries: 7,
		Jitter:     0,
	}

	if got := eb.MaxAttempts(); got != 7 {
		t.Errorf("MaxAttempts: got %d, want 7", got)
	}
}

func TestDefaultRetry(t *testing.T) {
	if DefaultRetry.BaseDelay != 1*time.Second {
		t.Errorf("BaseDelay: got %v, want 1s", DefaultRetry.BaseDelay)
	}
	if DefaultRetry.MaxDelay != 5*time.Minute {
		t.Errorf("MaxDelay: got %v, want 5m", DefaultRetry.MaxDelay)
	}
	if DefaultRetry.MaxRetries != 10 {
		t.Errorf("MaxRetries: got %d, want 10", DefaultRetry.MaxRetries)
	}
	if DefaultRetry.Jitter != 0.2 {
		t.Errorf("Jitter: got %f, want 0.2", DefaultRetry.Jitter)
	}
}
