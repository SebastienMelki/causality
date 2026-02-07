// Package transport provides HTTP transport for sending event batches
// to the Causality server with retry and exponential backoff.
package transport

import (
	"math"
	"math/rand"
	"time"
)

// RetryStrategy defines how retries are scheduled after transient failures.
type RetryStrategy interface {
	// NextDelay returns the delay before the next retry attempt.
	// Returns 0 if no more retries should be attempted.
	NextDelay(attempt int) time.Duration

	// MaxAttempts returns the maximum number of retry attempts.
	MaxAttempts() int
}

// ExponentialBackoff implements RetryStrategy with exponential delays,
// a maximum delay cap, and random jitter to prevent thundering herd.
type ExponentialBackoff struct {
	// BaseDelay is the initial delay before the first retry.
	BaseDelay time.Duration

	// MaxDelay is the upper bound on retry delay.
	MaxDelay time.Duration

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// Jitter is the proportion of randomness applied to the delay (0.0 to 1.0).
	// A jitter of 0.2 means the delay varies by +/- 20%.
	Jitter float64
}

// NextDelay returns the delay for the given attempt number (0-indexed).
// Returns 0 when attempt >= MaxRetries, signaling no more retries.
func (e *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	if attempt >= e.MaxRetries {
		return 0
	}

	// Calculate exponential delay: BaseDelay * 2^attempt
	delay := float64(e.BaseDelay) * math.Pow(2, float64(attempt))

	// Cap at MaxDelay
	if delay > float64(e.MaxDelay) {
		delay = float64(e.MaxDelay)
	}

	// Apply jitter: delay += delay * Jitter * (random value in [-1, 1])
	if e.Jitter > 0 {
		jitterRange := delay * e.Jitter
		//nolint:gosec // math/rand is fine for jitter; no security requirement
		delay += jitterRange * (rand.Float64()*2 - 1)
	}

	// Ensure delay is never negative after jitter
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// MaxAttempts returns the configured maximum number of retry attempts.
func (e *ExponentialBackoff) MaxAttempts() int {
	return e.MaxRetries
}

// DefaultRetry is a sensible default retry strategy for mobile SDK use.
// It retries up to 10 times with exponential backoff from 1s to 5m,
// with 20% jitter to spread out retries across devices.
var DefaultRetry = &ExponentialBackoff{
	BaseDelay:  1 * time.Second,
	MaxDelay:   5 * time.Minute,
	MaxRetries: 10,
	Jitter:     0.2,
}
