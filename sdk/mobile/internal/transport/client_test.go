package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fastRetry is a quick retry strategy for tests to avoid slow test runs.
var fastRetry = &ExponentialBackoff{
	BaseDelay:  1 * time.Millisecond,
	MaxDelay:   10 * time.Millisecond,
	MaxRetries: 3,
	Jitter:     0,
}

func TestNewClient(t *testing.T) {
	c := NewClient("https://example.com/", "test-key", 5*time.Second, nil)

	if c.endpoint != "https://example.com" {
		t.Errorf("endpoint: got %q, want %q", c.endpoint, "https://example.com")
	}
	if c.apiKey != "test-key" {
		t.Errorf("apiKey: got %q, want %q", c.apiKey, "test-key")
	}
	if c.userAgent != "CausalitySDK/1.0.0 Go" {
		t.Errorf("userAgent: got %q, want %q", c.userAgent, "CausalitySDK/1.0.0 Go")
	}
	if c.retry == nil {
		t.Error("retry should default to DefaultRetry when nil")
	}
}

func TestSendBatch_EmptyBatch(t *testing.T) {
	c := NewClient("https://example.com", "key", 5*time.Second, fastRetry)

	result, err := c.SendBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", result.StatusCode)
	}
	if result.Accepted != 0 {
		t.Errorf("Accepted: got %d, want 0", result.Accepted)
	}
}

func TestSendBatch_Success(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/events/batch" {
			t.Errorf("path: got %s, want /v1/events/batch", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("X-API-Key: got %q, want %q", r.Header.Get("X-API-Key"), "test-key")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type: got %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}
		if r.Header.Get("User-Agent") != "CausalitySDK/1.0.0 Go" {
			t.Errorf("User-Agent: got %q, want %q", r.Header.Get("User-Agent"), "CausalitySDK/1.0.0 Go")
		}

		// Verify body is a JSON array
		var events []json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("events: got %d, want 2", len(events))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted": 2}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	events := []string{
		`{"type":"screen_view","properties":{}}`,
		`{"type":"button_tap","properties":{}}`,
	}

	result, err := c.SendBatch(context.Background(), events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", result.StatusCode)
	}
	if result.Accepted != 2 {
		t.Errorf("Accepted: got %d, want 2", result.Accepted)
	}
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("requests: got %d, want 1", requestCount)
	}
}

func TestSendBatch_Retry5xx(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			// First request: 500
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal server error"}`))
			return
		}
		// Second request: 200
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted": 1}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	events := []string{`{"type":"test"}`}
	result, err := c.SendBatch(context.Background(), events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", result.StatusCode)
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("requests: got %d, want 2 (1 retry)", requestCount)
	}
}

func TestSendBatch_Retry429(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			// First request: 429 with no Retry-After (avoid slow test)
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		// Second request: 200
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted": 1}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	events := []string{`{"type":"test"}`}
	result, err := c.SendBatch(context.Background(), events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", result.StatusCode)
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("requests: got %d, want 2 (1 retry after 429)", requestCount)
	}
}

func TestSendBatch_NoRetry4xx(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	events := []string{`{"type":"test"}`}
	_, err := c.SendBatch(context.Background(), events)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("requests: got %d, want 1 (no retry for 400)", requestCount)
	}
}

func TestSendBatch_NoRetry403(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	events := []string{`{"type":"test"}`}
	_, err := c.SendBatch(context.Background(), events)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}

	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("requests: got %d, want 1 (no retry for 403)", requestCount)
	}
}

func TestSendBatch_AllRetriesExhausted(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"service unavailable"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	events := []string{`{"type":"test"}`}
	_, err := c.SendBatch(context.Background(), events)
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}

	// Should be initial attempt + maxRetries retries = 1 + 3 = 4
	expected := int32(fastRetry.MaxRetries + 1)
	if got := atomic.LoadInt32(&requestCount); got != expected {
		t.Errorf("requests: got %d, want %d (initial + %d retries)", got, expected, fastRetry.MaxRetries)
	}
}

func TestSendBatch_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, &ExponentialBackoff{
		BaseDelay:  1 * time.Second, // Long delay so context cancels first
		MaxDelay:   10 * time.Second,
		MaxRetries: 10,
		Jitter:     0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	events := []string{`{"type":"test"}`}
	_, err := c.SendBatch(ctx, events)
	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
}

func TestSendBatch_Retry502(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted":1}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	result, err := c.SendBatch(context.Background(), []string{`{"type":"test"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", result.StatusCode)
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("requests: got %d, want 2", requestCount)
	}
}

func TestSendBatch_Retry504(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted":1}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	result, err := c.SendBatch(context.Background(), []string{`{"type":"test"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", result.StatusCode)
	}
}

func TestIsRetryableStatus(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{200, false},
		{201, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}

	for _, tt := range tests {
		got := isRetryableStatus(tt.code)
		if got != tt.want {
			t.Errorf("isRetryableStatus(%d): got %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestRetryDelay_NoRetryAfterHeader(t *testing.T) {
	c := NewClient("https://example.com", "key", 5*time.Second, fastRetry)

	delay := c.retryDelay(0, "")
	if delay <= 0 {
		t.Errorf("expected positive delay without Retry-After header, got %v", delay)
	}
}

func TestRetryDelay_RetryAfterSeconds(t *testing.T) {
	// Use a retry strategy with tiny delays so the Retry-After header dominates
	c := NewClient("https://example.com", "key", 5*time.Second, &ExponentialBackoff{
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		MaxRetries: 5,
		Jitter:     0,
	})

	// Retry-After: 5 (seconds) is larger than strategy delay (1ms)
	delay := c.retryDelay(0, "5")
	if delay != 5*time.Second {
		t.Errorf("expected 5s from Retry-After header, got %v", delay)
	}
}

func TestRetryDelay_RetryAfterSmallerThanStrategy(t *testing.T) {
	// Strategy delay is much larger than Retry-After
	c := NewClient("https://example.com", "key", 5*time.Second, &ExponentialBackoff{
		BaseDelay:  10 * time.Second,
		MaxDelay:   1 * time.Minute,
		MaxRetries: 5,
		Jitter:     0,
	})

	// Retry-After: 1 (second) is smaller than strategy delay (10s)
	delay := c.retryDelay(0, "1")
	if delay != 10*time.Second {
		t.Errorf("expected 10s from strategy (larger), got %v", delay)
	}
}

func TestRetryDelay_RetryAfterHTTPDate(t *testing.T) {
	c := NewClient("https://example.com", "key", 5*time.Second, &ExponentialBackoff{
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		MaxRetries: 5,
		Jitter:     0,
	})

	// HTTP-date 1 hour from now should be larger than strategy delay
	futureDate := time.Now().Add(1 * time.Hour).UTC().Format(http.TimeFormat)
	delay := c.retryDelay(0, futureDate)
	// Should use the HTTP-date delay (roughly 1 hour)
	if delay < 59*time.Minute {
		t.Errorf("expected ~1h from Retry-After date, got %v", delay)
	}
}

func TestRetryDelay_RetryAfterInvalidValue(t *testing.T) {
	c := NewClient("https://example.com", "key", 5*time.Second, fastRetry)

	// Invalid Retry-After should fall back to strategy delay
	delay := c.retryDelay(0, "not-a-number-or-date")
	strategyDelay := fastRetry.NextDelay(0)
	if delay != strategyDelay {
		t.Errorf("expected strategy delay %v for invalid Retry-After, got %v", strategyDelay, delay)
	}
}

func TestRetryDelay_MaxRetriesExhausted(t *testing.T) {
	c := NewClient("https://example.com", "key", 5*time.Second, fastRetry)

	// Attempt beyond MaxRetries should return 0
	delay := c.retryDelay(fastRetry.MaxRetries, "5")
	if delay != 0 {
		t.Errorf("expected 0 for exhausted retries, got %v", delay)
	}
}

func TestSendBatch_Retry429WithRetryAfterHeader(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			// 429 with a small Retry-After value
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted": 1}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	result, err := c.SendBatch(context.Background(), []string{`{"type":"test"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", result.StatusCode)
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("requests: got %d, want 2", requestCount)
	}
}

func TestSendBatch_ContextCanceledBeforeFirstRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	_, err := c.SendBatch(ctx, []string{`{"type":"test"}`})
	if err == nil {
		t.Fatal("expected error when context is already canceled")
	}
}

func TestSendBatch_ResponseWithoutAcceptedField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second, fastRetry)

	events := []string{`{"type":"test1"}`, `{"type":"test2"}`}
	result, err := c.SendBatch(context.Background(), events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should default to len(events) when response doesn't contain accepted field
	if result.Accepted != 2 {
		t.Errorf("Accepted: got %d, want 2 (default to len(events))", result.Accepted)
	}
}
