// Package gateway tests the HTTP middleware for rate limiting, body size limits, etc.
package gateway

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SebastienMelki/causality/internal/auth"
)

// TestPerKeyRateLimit_AllowsUnderLimit verifies requests under the rate limit pass through.
func TestPerKeyRateLimit_AllowsUnderLimit(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:     true,
		PerKeyRPS:   100, // High limit
		PerKeyBurst: 100,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := PerKeyRateLimit(cfg)(handler)

	// Create request with app_id in context
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	ctx := context.WithValue(req.Context(), auth.AppIDContextKey, "test-app")
	req = req.WithContext(ctx)

	// Should allow multiple requests under the limit
	for i := range 10 {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i, rec.Code, http.StatusOK)
		}
	}
}

// TestPerKeyRateLimit_BlocksOverLimit verifies requests over the rate limit are blocked.
func TestPerKeyRateLimit_BlocksOverLimit(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:     true,
		PerKeyRPS:   1,  // Very low limit
		PerKeyBurst: 1,  // Only 1 request allowed
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := PerKeyRateLimit(cfg)(handler)

	// Create request with app_id in context
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	ctx := context.WithValue(req.Context(), auth.AppIDContextKey, "test-app")
	req = req.WithContext(ctx)

	// First request should succeed (burst of 1)
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Errorf("First request: got status %d, want %d", rec1.Code, http.StatusOK)
	}

	// Second request should be rate limited
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: got status %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}

// TestPerKeyRateLimit_DifferentKeysIndependent verifies different app_ids have separate limits.
func TestPerKeyRateLimit_DifferentKeysIndependent(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:     true,
		PerKeyRPS:   1,
		PerKeyBurst: 1,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := PerKeyRateLimit(cfg)(handler)

	// Request from app-1
	req1 := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	ctx1 := context.WithValue(req1.Context(), auth.AppIDContextKey, "app-1")
	req1 = req1.WithContext(ctx1)

	// Request from app-2
	req2 := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	ctx2 := context.WithValue(req2.Context(), auth.AppIDContextKey, "app-2")
	req2 = req2.WithContext(ctx2)

	// First request from app-1 should succeed
	rec1a := httptest.NewRecorder()
	middleware.ServeHTTP(rec1a, req1)
	if rec1a.Code != http.StatusOK {
		t.Errorf("app-1 first request: got status %d, want %d", rec1a.Code, http.StatusOK)
	}

	// First request from app-2 should also succeed (independent limit)
	rec2a := httptest.NewRecorder()
	middleware.ServeHTTP(rec2a, req2)
	if rec2a.Code != http.StatusOK {
		t.Errorf("app-2 first request: got status %d, want %d", rec2a.Code, http.StatusOK)
	}

	// Second request from app-1 should be rate limited
	rec1b := httptest.NewRecorder()
	middleware.ServeHTTP(rec1b, req1)
	if rec1b.Code != http.StatusTooManyRequests {
		t.Errorf("app-1 second request: got status %d, want %d", rec1b.Code, http.StatusTooManyRequests)
	}

	// Second request from app-2 should also be rate limited
	rec2b := httptest.NewRecorder()
	middleware.ServeHTTP(rec2b, req2)
	if rec2b.Code != http.StatusTooManyRequests {
		t.Errorf("app-2 second request: got status %d, want %d", rec2b.Code, http.StatusTooManyRequests)
	}
}

// TestPerKeyRateLimit_NoAppIDPassesThrough verifies requests without app_id pass through.
func TestPerKeyRateLimit_NoAppIDPassesThrough(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:     true,
		PerKeyRPS:   1,
		PerKeyBurst: 1,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := PerKeyRateLimit(cfg)(handler)

	// Request without app_id in context
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	// Multiple requests should all pass (no rate limiting without app_id)
	for i := range 10 {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d without app_id: got status %d, want %d", i, rec.Code, http.StatusOK)
		}
	}
}

// TestPerKeyRateLimit_Disabled verifies disabled rate limiting passes all requests.
func TestPerKeyRateLimit_Disabled(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled: false,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := PerKeyRateLimit(cfg)(handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	ctx := context.WithValue(req.Context(), auth.AppIDContextKey, "test-app")
	req = req.WithContext(ctx)

	// All requests should pass when disabled
	for i := range 100 {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d with disabled rate limit: got status %d, want %d", i, rec.Code, http.StatusOK)
		}
	}
}

// TestBodySizeLimit_UnderLimit verifies requests under the body size limit pass through.
func TestBodySizeLimit_UnderLimit(t *testing.T) {
	maxSize := int64(1024) // 1KB

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the body to verify it's accessible
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if len(body) != 100 {
			t.Errorf("Body length = %d, want 100", len(body))
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := BodySizeLimit(maxSize)(handler)

	// Small body (100 bytes)
	body := bytes.Repeat([]byte("a"), 100)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))

	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Small body request: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestBodySizeLimit_OverLimit verifies requests over the body size limit are rejected.
func TestBodySizeLimit_OverLimit(t *testing.T) {
	maxSize := int64(100) // 100 bytes

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to read the body - should fail or be truncated
		_, err := io.ReadAll(r.Body)
		if err != nil {
			// MaxBytesReader returns an error when limit is exceeded
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := BodySizeLimit(maxSize)(handler)

	// Large body (200 bytes - over limit)
	body := bytes.Repeat([]byte("a"), 200)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))

	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	// Should return 413 Request Entity Too Large
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Large body request: got status %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

// TestBodySizeLimit_ExactLimit verifies requests at exactly the body size limit pass.
func TestBodySizeLimit_ExactLimit(t *testing.T) {
	maxSize := int64(100) // 100 bytes

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
			return
		}
		if len(body) != 100 {
			t.Errorf("Body length = %d, want 100", len(body))
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := BodySizeLimit(maxSize)(handler)

	// Exact limit body (100 bytes)
	body := bytes.Repeat([]byte("a"), 100)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))

	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Exact limit body request: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestRequestID_Generated verifies that a request ID is generated when not provided.
func TestRequestID_Generated(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request ID is in context
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("Request ID should be in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestID(handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Verify X-Request-ID is in response header
	respID := rec.Header().Get("X-Request-ID")
	if respID == "" {
		t.Error("X-Request-ID should be in response header")
	}
}

// TestRequestID_Preserved verifies that an existing request ID is preserved.
func TestRequestID_Preserved(t *testing.T) {
	existingID := "existing-request-id-12345"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID != existingID {
			t.Errorf("Request ID = %q, want %q", requestID, existingID)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestID(handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.Header.Set("X-Request-ID", existingID)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Verify same ID is in response
	respID := rec.Header().Get("X-Request-ID")
	if respID != existingID {
		t.Errorf("Response X-Request-ID = %q, want %q", respID, existingID)
	}
}

// TestRecovery_PanicRecovered verifies that panics are recovered and return 500.
func TestRecovery_PanicRecovered(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Use slog.Default() instead of nil to avoid nil pointer dereference
	logger := slog.Default()
	middleware := Recovery(logger)(handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Panic recovery: got status %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestChain_MiddlewareOrder verifies that middleware is applied in the correct order.
func TestChain_MiddlewareOrder(t *testing.T) {
	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chained := Chain(handler, mw1, mw2)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	chained.ServeHTTP(rec, req)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("Order length = %d, want %d", len(order), len(expected))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("Order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

// TestContentType_SetsJSON verifies that the Content-Type header is set to application/json.
func TestContentType_SetsJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ContentType(handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestGlobalRateLimit_AllowsUnderLimit verifies global rate limiting under limit.
func TestGlobalRateLimit_AllowsUnderLimit(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 100,
		BurstSize:         100,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimit(cfg)(handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)

	for i := range 10 {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i, rec.Code, http.StatusOK)
		}
	}
}

// TestGlobalRateLimit_BlocksOverLimit verifies global rate limiting over limit.
func TestGlobalRateLimit_BlocksOverLimit(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 1,
		BurstSize:         1,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimit(cfg)(handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)

	// First request should succeed
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Errorf("First request: got status %d, want %d", rec1.Code, http.StatusOK)
	}

	// Second request should be rate limited
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: got status %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}
