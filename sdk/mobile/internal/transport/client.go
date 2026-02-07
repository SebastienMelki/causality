package transport

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	causalityv1 "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// SendResult holds the outcome of a batch send operation.
type SendResult struct {
	// StatusCode is the HTTP status code from the server.
	StatusCode int

	// Accepted is the number of events accepted by the server.
	Accepted int
}

// statusCapture wraps an http.RoundTripper to capture the HTTP status code
// and Retry-After header from responses. This enables retry decisions when
// using the generated protobuf client, which doesn't expose raw HTTP details.
type statusCapture struct {
	transport  http.RoundTripper
	mu         sync.Mutex
	lastStatus int
	retryAfter string
}

func (s *statusCapture) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := s.transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	s.mu.Lock()
	s.lastStatus = resp.StatusCode
	s.retryAfter = resp.Header.Get("Retry-After")
	s.mu.Unlock()
	return resp, nil
}

func (s *statusCapture) reset() {
	s.mu.Lock()
	s.lastStatus = 0
	s.retryAfter = ""
	s.mu.Unlock()
}

func (s *statusCapture) getLastStatus() (int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastStatus, s.retryAfter
}

// Client sends event batches to the Causality server using the generated
// protobuf HTTP client. It handles retries with configurable backoff strategies.
type Client struct {
	rpcClient causalityv1.EventServiceClient
	capture   *statusCapture
	retry     RetryStrategy
	endpoint  string
}

// NewClient creates a new transport client backed by the generated protobuf client.
//
// endpoint is the base URL of the Causality server (e.g., "https://analytics.example.com").
// apiKey is the API key for authentication.
// timeout is the HTTP request timeout.
// retry is the retry strategy for transient failures. If nil, DefaultRetry is used.
func NewClient(endpoint, apiKey string, timeout time.Duration, retry RetryStrategy) *Client {
	if retry == nil {
		retry = DefaultRetry
	}

	capture := &statusCapture{transport: http.DefaultTransport}

	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: capture,
	}

	rpcClient := causalityv1.NewEventServiceClient(
		endpoint,
		causalityv1.WithEventServiceHTTPClient(httpClient),
		causalityv1.WithEventServiceContentType(causalityv1.ContentTypeJSON),
		causalityv1.WithEventServiceDefaultHeader("X-API-Key", apiKey),
		causalityv1.WithEventServiceDefaultHeader("User-Agent", "CausalitySDK/1.0.0 Go"),
	)

	return &Client{
		rpcClient: rpcClient,
		capture:   capture,
		retry:     retry,
		endpoint:  endpoint,
	}
}

// SendBatch sends a batch of serialized event JSON strings to the server.
// Each JSON string is an SDK Event with type, properties, and metadata.
// Events are converted to protobuf EventEnvelopes and sent via IngestEventBatch.
//
// It retries on 5xx, 429, and network errors with the configured retry strategy.
// Non-retryable errors (4xx except 429) return immediately.
// The context can be used for cancellation.
func (c *Client) SendBatch(ctx context.Context, events []string) (*SendResult, error) {
	if len(events) == 0 {
		return &SendResult{StatusCode: 200, Accepted: 0}, nil
	}

	// Convert SDK JSON events to protobuf EventEnvelopes
	envelopes, err := convertEvents(events)
	if err != nil {
		return nil, fmt.Errorf("convert events: %w", err)
	}

	req := &causalityv1.IngestEventBatchRequest{
		Events: envelopes,
	}

	log.Printf("[Causality:Transport] IngestEventBatch %s (%d events)", c.endpoint, len(events))

	var lastErr error
	maxAttempts := c.retry.MaxAttempts()

	for attempt := 0; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		// Reset captured status before each attempt
		c.capture.reset()

		resp, err := c.rpcClient.IngestEventBatch(ctx, req)
		if err != nil {
			status, retryAfter := c.capture.getLastStatus()

			log.Printf("[Causality:Transport] Error (HTTP %d): %v", status, err)

			// Non-retryable client error (4xx except 429)
			if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
				return nil, fmt.Errorf("non-retryable error: %w", err)
			}

			// Retryable: network error (status 0), 429, or 5xx
			lastErr = err

			delay := c.retryDelay(attempt, retryAfter)
			if delay == 0 {
				break
			}

			log.Printf("[Causality:Transport] Retrying in %v (attempt %d/%d)", delay, attempt+1, maxAttempts)

			if !sleepWithContext(ctx, delay) {
				return nil, fmt.Errorf("context canceled during retry wait: %w", ctx.Err())
			}
			continue
		}

		log.Printf("[Causality:Transport] Success: accepted=%d, rejected=%d",
			resp.AcceptedCount, resp.RejectedCount)

		return &SendResult{
			StatusCode: 200,
			Accepted:   int(resp.AcceptedCount),
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
	}

	return nil, fmt.Errorf("all retries exhausted")
}

// retryDelay determines the delay before the next retry attempt.
// If a Retry-After header is present and valid, it takes precedence over
// the retry strategy's calculated delay.
func (c *Client) retryDelay(attempt int, retryAfter string) time.Duration {
	strategyDelay := c.retry.NextDelay(attempt)
	if strategyDelay == 0 {
		return 0
	}

	if retryAfter == "" {
		return strategyDelay
	}

	// Try parsing as seconds
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		headerDelay := time.Duration(seconds) * time.Second
		if headerDelay > strategyDelay {
			return headerDelay
		}
		return strategyDelay
	}

	// Try parsing as HTTP-date
	if t, err := http.ParseTime(retryAfter); err == nil {
		headerDelay := time.Until(t)
		if headerDelay > 0 && headerDelay > strategyDelay {
			return headerDelay
		}
	}

	return strategyDelay
}

// sleepWithContext sleeps for the given duration or until the context is canceled.
// Returns true if the full sleep completed, false if canceled.
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
