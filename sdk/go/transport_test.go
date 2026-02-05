// Package causality tests the HTTP transport for the Go SDK.
package causality

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestSendBatch_Success verifies successful batch sending.
func TestSendBatch_Success(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		// Verify the request
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/events/batch" {
			t.Errorf("Path = %q, want %q", r.URL.Path, "/v1/events/batch")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Timeout:    5 * time.Second,
		MaxRetries: 3,
	})

	events := []Event{
		{EventType: "screenView", AppID: "test-app"},
		{EventType: "buttonTap", AppID: "test-app"},
	}

	err := transport.sendBatch(context.Background(), events)
	if err != nil {
		t.Errorf("sendBatch() returned error: %v", err)
	}

	if requestCount.Load() != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount.Load())
	}
}

// TestSendBatch_ServerError_Retries verifies retry behavior on 5xx errors.
func TestSendBatch_ServerError_Retries(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		// First two requests return 500, third returns 200
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Timeout:    5 * time.Second,
		MaxRetries: 5, // Allow enough retries
	})

	events := []Event{
		{EventType: "screenView", AppID: "test-app"},
	}

	err := transport.sendBatch(context.Background(), events)
	if err != nil {
		t.Errorf("sendBatch() should succeed after retries: %v", err)
	}

	// Should have made 3 requests (2 failures + 1 success)
	if requestCount.Load() != 3 {
		t.Errorf("Expected 3 requests (2 retries + 1 success), got %d", requestCount.Load())
	}
}

// TestSendBatch_ClientError_NoRetry verifies no retry on 4xx errors.
func TestSendBatch_ClientError_NoRetry(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusBadRequest) // 400
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Timeout:    5 * time.Second,
		MaxRetries: 5,
	})

	events := []Event{
		{EventType: "screenView", AppID: "test-app"},
	}

	err := transport.sendBatch(context.Background(), events)
	if err == nil {
		t.Error("sendBatch() should return error for 4xx response")
	}

	// Should have made only 1 request (no retries for client errors)
	if requestCount.Load() != 1 {
		t.Errorf("Expected 1 request (no retry for 4xx), got %d", requestCount.Load())
	}
}

// TestSendBatch_SetsHeaders verifies that required headers are set.
func TestSendBatch_SetsHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}
		if r.Header.Get("X-API-Key") != "my-secret-api-key" {
			t.Errorf("X-API-Key = %q, want %q", r.Header.Get("X-API-Key"), "my-secret-api-key")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "my-secret-api-key",
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	events := []Event{
		{EventType: "screenView", AppID: "test-app"},
	}

	err := transport.sendBatch(context.Background(), events)
	if err != nil {
		t.Errorf("sendBatch() returned error: %v", err)
	}
}

// TestSendBatch_EmptyBatch verifies empty batch handling.
func TestSendBatch_EmptyBatch(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	// Send empty batch
	err := transport.sendBatch(context.Background(), []Event{})
	if err != nil {
		t.Errorf("sendBatch() with empty batch should not error: %v", err)
	}

	// Should not have made any request
	if requestCount.Load() != 0 {
		t.Errorf("Expected 0 requests for empty batch, got %d", requestCount.Load())
	}
}

// TestSendBatch_NilBatch verifies nil batch handling.
func TestSendBatch_NilBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	err := transport.sendBatch(context.Background(), nil)
	if err != nil {
		t.Errorf("sendBatch() with nil batch should not error: %v", err)
	}
}

// TestSendBatch_ContextCancellation verifies context cancellation is respected.
func TestSendBatch_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	events := []Event{
		{EventType: "screenView", AppID: "test-app"},
	}

	err := transport.sendBatch(ctx, events)
	if err == nil {
		t.Error("sendBatch() should return error for cancelled context")
	}
}

// TestSendBatch_MaxRetriesExhausted verifies behavior when all retries fail.
func TestSendBatch_MaxRetriesExhausted(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError) // Always fail
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Timeout:    5 * time.Second,
		MaxRetries: 2, // Allow 2 retries
	})

	events := []Event{
		{EventType: "screenView", AppID: "test-app"},
	}

	err := transport.sendBatch(context.Background(), events)
	if err == nil {
		t.Error("sendBatch() should return error when all retries exhausted")
	}

	// Should have made 3 requests (1 initial + 2 retries)
	if requestCount.Load() != 3 {
		t.Errorf("Expected 3 requests (1 initial + 2 retries), got %d", requestCount.Load())
	}
}

// TestSendBatch_Various2xxStatusCodes verifies 2xx responses are success.
func TestSendBatch_Various2xxStatusCodes(t *testing.T) {
	statusCodes := []int{200, 201, 202, 204}

	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			transport := newHTTPTransport(Config{
				Endpoint:   server.URL,
				APIKey:     "test-api-key",
				Timeout:    5 * time.Second,
				MaxRetries: 0,
			})

			events := []Event{
				{EventType: "screenView", AppID: "test-app"},
			}

			err := transport.sendBatch(context.Background(), events)
			if err != nil {
				t.Errorf("sendBatch() should succeed for %d: %v", code, err)
			}
		})
	}
}

// TestExponentialBackoff verifies backoff calculation.
func TestExponentialBackoff(t *testing.T) {
	// Test that backoff increases with attempts
	delays := make([]time.Duration, 5)
	for i := range 5 {
		delays[i] = exponentialBackoff(i)
	}

	// Due to jitter, we can't test exact values, but we can verify:
	// 1. Delays should be within expected ranges
	// 2. They should not exceed max delay (10s)

	const maxDelay = 10 * time.Second

	for i, delay := range delays {
		if delay > maxDelay {
			t.Errorf("Attempt %d: delay %v exceeds max delay %v", i, delay, maxDelay)
		}
		if delay < 0 {
			t.Errorf("Attempt %d: delay %v should not be negative", i, delay)
		}
	}
}

// TestNewHTTPTransport verifies transport creation.
func TestNewHTTPTransport(t *testing.T) {
	cfg := Config{
		Endpoint:   "http://localhost:8080",
		APIKey:     "my-api-key",
		Timeout:    30 * time.Second,
		MaxRetries: 5,
	}

	transport := newHTTPTransport(cfg)

	if transport.endpoint != "http://localhost:8080" {
		t.Errorf("endpoint = %q, want %q", transport.endpoint, "http://localhost:8080")
	}
	if transport.apiKey != "my-api-key" {
		t.Errorf("apiKey = %q, want %q", transport.apiKey, "my-api-key")
	}
	if transport.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want %d", transport.maxRetries, 5)
	}
	if transport.client.Timeout != 30*time.Second {
		t.Errorf("client.Timeout = %v, want %v", transport.client.Timeout, 30*time.Second)
	}
}
