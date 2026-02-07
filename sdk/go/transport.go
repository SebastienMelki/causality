package causality

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// httpTransport handles HTTP communication with the Causality server.
type httpTransport struct {
	client     *http.Client
	endpoint   string
	apiKey     string
	maxRetries int
}

// newHTTPTransport creates a new HTTP transport with the given configuration.
func newHTTPTransport(cfg Config) *httpTransport {
	return &httpTransport{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		endpoint:   cfg.Endpoint,
		apiKey:     cfg.APIKey,
		maxRetries: cfg.MaxRetries,
	}
}

// sendBatch sends a batch of events to the server.
// It retries on 5xx errors with exponential backoff and jitter.
// Returns nil on success (2xx), or an error on failure.
func (t *httpTransport) sendBatch(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	body, err := json.Marshal(batchRequest{Events: events})
	if err != nil {
		return fmt.Errorf("causality: failed to marshal events: %w", err)
	}

	url := t.endpoint + "/v1/events/batch"

	var lastErr error
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			// Wait with exponential backoff and jitter before retry
			delay := exponentialBackoff(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("causality: failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", t.apiKey)

		resp, err := t.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("causality: request failed: %w", err)
			continue
		}

		// Read and discard body to enable connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// Success: 2xx status codes
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		// Client error (4xx): don't retry, return immediately
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("causality: client error: status %d", resp.StatusCode)
		}

		// Server error (5xx): retry
		lastErr = fmt.Errorf("causality: server error: status %d", resp.StatusCode)
	}

	return lastErr
}

// exponentialBackoff calculates the backoff duration for a given attempt.
// Uses exponential backoff with full jitter.
// Base delay is 100ms, max delay is 10s.
func exponentialBackoff(attempt int) time.Duration {
	const (
		baseDelay = 100 * time.Millisecond
		maxDelay  = 10 * time.Second
	)

	// Calculate exponential delay: baseDelay * 2^attempt
	delay := float64(baseDelay) * math.Pow(2, float64(attempt))

	// Cap at max delay
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	// Add full jitter: random value between 0 and delay
	jitter := rand.Float64() * delay

	return time.Duration(jitter)
}
