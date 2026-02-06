package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SendResult holds the outcome of a batch send operation.
type SendResult struct {
	// StatusCode is the HTTP status code from the server.
	StatusCode int

	// Accepted is the number of events accepted by the server.
	Accepted int
}

// Client sends event batches to the Causality server over HTTP.
// It handles retries with configurable backoff strategies.
type Client struct {
	httpClient *http.Client
	endpoint   string
	apiKey     string
	retry      RetryStrategy
	userAgent  string
}

// NewClient creates a new HTTP transport client.
//
// endpoint is the base URL of the Causality server (e.g., "https://analytics.example.com").
// apiKey is the API key for authentication.
// timeout is the HTTP request timeout.
// retry is the retry strategy for transient failures. If nil, DefaultRetry is used.
func NewClient(endpoint, apiKey string, timeout time.Duration, retry RetryStrategy) *Client {
	if retry == nil {
		retry = DefaultRetry
	}

	// Normalize endpoint: strip trailing slash
	endpoint = strings.TrimRight(endpoint, "/")

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		endpoint:  endpoint,
		apiKey:    apiKey,
		retry:     retry,
		userAgent: "CausalitySDK/1.0.0 Go",
	}
}

// SendBatch sends a batch of serialized event JSON strings to the server.
// It retries on 5xx and 429 responses with the configured retry strategy.
// Non-retryable errors (4xx except 429) return immediately.
// The context can be used for cancellation.
func (c *Client) SendBatch(ctx context.Context, events []string) (*SendResult, error) {
	if len(events) == 0 {
		return &SendResult{StatusCode: 200, Accepted: 0}, nil
	}

	// Build request body: JSON array of raw event JSON strings.
	// Each event is already a JSON string, so we parse them into raw messages
	// to build a proper JSON array.
	rawEvents := make([]json.RawMessage, len(events))
	for i, e := range events {
		rawEvents[i] = json.RawMessage(e)
	}

	body, err := json.Marshal(rawEvents)
	if err != nil {
		return nil, fmt.Errorf("marshal batch: %w", err)
	}

	url := c.endpoint + "/v1/events/batch"

	var lastErr error
	maxAttempts := c.retry.MaxAttempts()

	for attempt := 0; attempt <= maxAttempts; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		// Build request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", c.apiKey)
		req.Header.Set("User-Agent", c.userAgent)

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)

			// Network errors are retryable
			delay := c.retry.NextDelay(attempt)
			if delay == 0 {
				break
			}
			if !sleepWithContext(ctx, delay) {
				return nil, fmt.Errorf("context canceled during retry wait: %w", ctx.Err())
			}
			continue
		}

		// Read and close body
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			result := &SendResult{
				StatusCode: resp.StatusCode,
				Accepted:   len(events),
			}

			// Try to parse accepted count from response if available
			var respData struct {
				Accepted int `json:"accepted"`
			}
			if json.Unmarshal(respBody, &respData) == nil && respData.Accepted > 0 {
				result.Accepted = respData.Accepted
			}

			return result, nil
		}

		// Non-retryable client error (4xx except 429)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return nil, fmt.Errorf("non-retryable error: HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		// Retryable: 429 or 5xx
		lastErr = fmt.Errorf("retryable error: HTTP %d: %s", resp.StatusCode, string(respBody))

		// Check for Retry-After header
		delay := c.retryDelay(attempt, resp.Header.Get("Retry-After"))
		if delay == 0 {
			break
		}

		if !sleepWithContext(ctx, delay) {
			return nil, fmt.Errorf("context canceled during retry wait: %w", ctx.Err())
		}
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
		return 0 // No more retries
	}

	if retryAfter == "" {
		return strategyDelay
	}

	// Try parsing as seconds
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		headerDelay := time.Duration(seconds) * time.Second
		// Use the larger of the two delays
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

// isRetryableStatus returns true for HTTP status codes that warrant a retry.
func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
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
