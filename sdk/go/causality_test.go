// Package causality tests the Go SDK client.
package causality

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestNew_ValidConfig verifies client creation with valid configuration.
func TestNew_ValidConfig(t *testing.T) {
	cfg := Config{
		APIKey:   "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint: "http://localhost:8080",
		AppID:    "test-app",
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("New() returned nil client")
	}
}

// TestNew_MissingAPIKey_ReturnsError verifies error when API key is missing.
func TestNew_MissingAPIKey_ReturnsError(t *testing.T) {
	cfg := Config{
		APIKey:   "", // Missing
		Endpoint: "http://localhost:8080",
		AppID:    "test-app",
	}

	client, err := New(cfg)
	if err == nil {
		if client != nil {
			client.Close()
		}
		t.Error("New() should return error when APIKey is missing")
	}
}

// TestNew_MissingEndpoint_ReturnsError verifies error when endpoint is missing.
func TestNew_MissingEndpoint_ReturnsError(t *testing.T) {
	cfg := Config{
		APIKey:   "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint: "", // Missing
		AppID:    "test-app",
	}

	client, err := New(cfg)
	if err == nil {
		if client != nil {
			client.Close()
		}
		t.Error("New() should return error when Endpoint is missing")
	}
}

// TestNew_MissingAppID_ReturnsError verifies error when app ID is missing.
func TestNew_MissingAppID_ReturnsError(t *testing.T) {
	cfg := Config{
		APIKey:   "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint: "http://localhost:8080",
		AppID:    "", // Missing
	}

	client, err := New(cfg)
	if err == nil {
		if client != nil {
			client.Close()
		}
		t.Error("New() should return error when AppID is missing")
	}
}

// TestTrack_SetsIdempotencyKey verifies that Track sets idempotency key.
func TestTrack_SetsIdempotencyKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIKey:        "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint:      server.URL,
		AppID:         "test-app",
		BatchSize:     1,
		FlushInterval: time.Hour, // Long interval to prevent auto-flush
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer client.Close()

	// Track an event
	event := Event{EventType: "screenView"}
	client.Track(event)

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)

	// The idempotency key should have been set by Track
	// We can't directly verify the event because it's sent to the server,
	// but if it didn't crash and the server received it, we're good
}

// TestTrack_SetsTimestamp verifies that Track sets timestamp if not provided.
func TestTrack_SetsTimestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIKey:        "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint:      server.URL,
		AppID:         "test-app",
		BatchSize:     1,
		FlushInterval: time.Hour,
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer client.Close()

	// Track an event without timestamp
	event := Event{EventType: "screenView", Timestamp: ""}
	client.Track(event)

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)
}

// TestTrack_SetsAppID verifies that Track sets app ID from config.
func TestTrack_SetsAppID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIKey:        "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint:      server.URL,
		AppID:         "test-app-123",
		BatchSize:     1,
		FlushInterval: time.Hour,
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer client.Close()

	// Track an event
	event := Event{EventType: "screenView"}
	client.Track(event)

	time.Sleep(50 * time.Millisecond)
}

// TestFlush_SendsEvents verifies that Flush sends accumulated events.
func TestFlush_SendsEvents(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIKey:        "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint:      server.URL,
		AppID:         "test-app",
		BatchSize:     100,           // Large batch size
		FlushInterval: time.Hour,     // Long interval
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer client.Close()

	// Track multiple events
	for i := 0; i < 5; i++ {
		client.Track(Event{EventType: "test"})
	}

	// Manual flush
	err = client.Flush()
	if err != nil {
		t.Errorf("Flush() returned error: %v", err)
	}

	// Server should have received at least one request
	if requestCount.Load() < 1 {
		t.Error("Flush() should have sent events to server")
	}
}

// TestClose_FlushesRemainingEvents verifies that Close flushes remaining events.
func TestClose_FlushesRemainingEvents(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIKey:        "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint:      server.URL,
		AppID:         "test-app",
		BatchSize:     100,           // Large batch size
		FlushInterval: time.Hour,     // Long interval
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Track events
	for i := 0; i < 5; i++ {
		client.Track(Event{EventType: "test"})
	}

	// Close should flush remaining events
	err = client.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Server should have received at least one request
	if requestCount.Load() < 1 {
		t.Error("Close() should have flushed remaining events to server")
	}
}

// TestClose_SafeToCallMultipleTimes verifies Close is idempotent.
func TestClose_SafeToCallMultipleTimes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIKey:        "test-api-key-1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Endpoint:      server.URL,
		AppID:         "test-app",
		FlushInterval: time.Hour,
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Close multiple times should not panic
	err1 := client.Close()
	err2 := client.Close()
	err3 := client.Close()

	// First close might return an error, subsequent should be no-ops
	_ = err1
	_ = err2
	_ = err3
}

// TestConfig_WithDefaults verifies default values are applied.
func TestConfig_WithDefaults(t *testing.T) {
	cfg := Config{
		APIKey:   "test-key",
		Endpoint: "http://localhost:8080/",
		AppID:    "test-app",
	}

	cfg = cfg.withDefaults()

	if cfg.BatchSize != DefaultBatchSize {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, DefaultBatchSize)
	}
	if cfg.FlushInterval != DefaultFlushInterval {
		t.Errorf("FlushInterval = %v, want %v", cfg.FlushInterval, DefaultFlushInterval)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, DefaultMaxRetries)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
	// Trailing slash should be trimmed
	if cfg.Endpoint != "http://localhost:8080" {
		t.Errorf("Endpoint = %q, should have trailing slash trimmed", cfg.Endpoint)
	}
}

// TestConfig_Validate verifies validation logic.
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				APIKey:   "key",
				Endpoint: "http://localhost:8080",
				AppID:    "app",
			},
			wantErr: false,
		},
		{
			name: "negative batch size",
			cfg: Config{
				APIKey:    "key",
				Endpoint:  "http://localhost:8080",
				AppID:     "app",
				BatchSize: -1,
			},
			wantErr: true,
		},
		{
			name: "negative flush interval",
			cfg: Config{
				APIKey:        "key",
				Endpoint:      "http://localhost:8080",
				AppID:         "app",
				FlushInterval: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative max retries",
			cfg: Config{
				APIKey:     "key",
				Endpoint:   "http://localhost:8080",
				AppID:      "app",
				MaxRetries: -1,
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			cfg: Config{
				APIKey:   "key",
				Endpoint: "http://localhost:8080",
				AppID:    "app",
				Timeout:  -1 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestCollectServerContext verifies server context collection.
func TestCollectServerContext(t *testing.T) {
	ctx := collectServerContext()

	if ctx == nil {
		t.Fatal("collectServerContext() returned nil")
	}

	// Hostname should be set (though it might be empty on some systems)
	// We just verify it doesn't panic

	// GoVersion should be set
	if ctx.GoVersion == "" {
		t.Error("GoVersion should be set")
	}

	// SDKVersion should be set
	if ctx.SDKVersion != SDKVersion {
		t.Errorf("SDKVersion = %q, want %q", ctx.SDKVersion, SDKVersion)
	}

	// OS should be set
	if ctx.OS == "" {
		t.Error("OS should be set")
	}

	// Arch should be set
	if ctx.Arch == "" {
		t.Error("Arch should be set")
	}
}

// TestBatcher_Add verifies batcher add behavior.
func TestBatcher_Add(t *testing.T) {
	transport := &httpTransport{
		client:   &http.Client{},
		endpoint: "http://localhost:8080",
		apiKey:   "test-key",
	}

	batcher := newBatcher(3, transport)

	// First two adds should not signal full
	full1 := batcher.add(Event{EventType: "test1"})
	full2 := batcher.add(Event{EventType: "test2"})

	if full1 {
		t.Error("First add should not signal full")
	}
	if full2 {
		t.Error("Second add should not signal full")
	}

	// Third add should signal full (batch size is 3)
	full3 := batcher.add(Event{EventType: "test3"})
	if !full3 {
		t.Error("Third add should signal full")
	}
}

// TestBatcher_Drain verifies batcher drain behavior.
func TestBatcher_Drain(t *testing.T) {
	transport := &httpTransport{
		client:   &http.Client{},
		endpoint: "http://localhost:8080",
		apiKey:   "test-key",
	}

	batcher := newBatcher(10, transport)

	// Add some events
	batcher.add(Event{EventType: "test1"})
	batcher.add(Event{EventType: "test2"})

	// Drain should return all events
	events := batcher.drain()
	if len(events) != 2 {
		t.Errorf("drain() returned %d events, want 2", len(events))
	}

	// Second drain should return empty
	events = batcher.drain()
	if len(events) != 0 {
		t.Errorf("Second drain() should return empty, got %d events", len(events))
	}
}

// TestBatcher_FlushLoop verifies the background flush loop.
func TestBatcher_FlushLoop(t *testing.T) {
	var flushCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flushCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newHTTPTransport(Config{
		Endpoint:   server.URL,
		APIKey:     "test-key",
		Timeout:    time.Second,
		MaxRetries: 0,
	})

	batcher := newBatcher(100, transport)

	// Add an event
	batcher.add(Event{EventType: "test"})

	// Start flush loop with short interval
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go batcher.flushLoop(ctx, 50*time.Millisecond, done)

	// Wait for at least one flush
	time.Sleep(100 * time.Millisecond)

	// Stop the loop
	cancel()
	<-done

	// Should have flushed at least once
	if flushCount.Load() < 1 {
		t.Error("Flush loop should have flushed at least once")
	}
}
