package mobile

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockCallback implements ErrorCallback for testing.
type mockCallback struct {
	mu       sync.Mutex
	calls    []mockCallbackCall
	received chan struct{}
}

type mockCallbackCall struct {
	Code     string
	Message  string
	Severity int
}

func newMockCallback() *mockCallback {
	return &mockCallback{
		received: make(chan struct{}, 10),
	}
}

func (m *mockCallback) OnError(code string, message string, severity int) {
	m.mu.Lock()
	m.calls = append(m.calls, mockCallbackCall{
		Code:     code,
		Message:  message,
		Severity: severity,
	})
	m.mu.Unlock()
	m.received <- struct{}{}
}

func (m *mockCallback) waitForCalls(n int, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for i := 0; i < n; i++ {
		select {
		case <-m.received:
		case <-deadline:
			return false
		}
	}
	return true
}

func (m *mockCallback) getCalls() []mockCallbackCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockCallbackCall, len(m.calls))
	copy(result, m.calls)
	return result
}

func TestRegisterErrorCallback_ReceivesCritical(t *testing.T) {
	// Clean state
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	err := newCriticalError(ErrCodeAuthFailed, "authentication failed: invalid API key")
	notifyErrorCallbacks(err)

	if !cb.waitForCalls(1, time.Second) {
		t.Fatal("callback not invoked within timeout")
	}

	calls := cb.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Code != ErrCodeAuthFailed {
		t.Errorf("Code = %q, want %q", calls[0].Code, ErrCodeAuthFailed)
	}
	if calls[0].Message != "authentication failed: invalid API key" {
		t.Errorf("Message = %q, want %q", calls[0].Message, "authentication failed: invalid API key")
	}
	if calls[0].Severity != int(SeverityCritical) {
		t.Errorf("Severity = %d, want %d", calls[0].Severity, int(SeverityCritical))
	}
}

func TestRegisterErrorCallback_ReceivesFatal(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	err := newFatalError(ErrCodeDiskFull, "disk full: cannot persist events")
	notifyErrorCallbacks(err)

	if !cb.waitForCalls(1, time.Second) {
		t.Fatal("callback not invoked within timeout")
	}

	calls := cb.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Code != ErrCodeDiskFull {
		t.Errorf("Code = %q, want %q", calls[0].Code, ErrCodeDiskFull)
	}
	if calls[0].Severity != int(SeverityFatal) {
		t.Errorf("Severity = %d, want %d", calls[0].Severity, int(SeverityFatal))
	}
}

func TestRegisterErrorCallback_ReceivesWarning(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	err := newWarningError(ErrCodeRateLimited, "rate limited: slow down")
	notifyErrorCallbacks(err)

	if !cb.waitForCalls(1, time.Second) {
		t.Fatal("callback not invoked within timeout for warning")
	}

	calls := cb.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
}

func TestRegisterErrorCallback_IgnoresDebug(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	err := newDebugError("DEBUG_INFO", "debug message")
	notifyErrorCallbacks(err)

	// Wait briefly to confirm callback is NOT invoked
	time.Sleep(50 * time.Millisecond)

	calls := cb.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 calls for debug severity, got %d", len(calls))
	}
}

func TestMultipleCallbacks_AllNotified(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	cb1 := newMockCallback()
	cb2 := newMockCallback()
	cb3 := newMockCallback()

	RegisterErrorCallback(cb1)
	RegisterErrorCallback(cb2)
	RegisterErrorCallback(cb3)

	err := newCriticalError(ErrCodeServerError, "server returned 500")
	notifyErrorCallbacks(err)

	if !cb1.waitForCalls(1, time.Second) {
		t.Fatal("callback 1 not invoked within timeout")
	}
	if !cb2.waitForCalls(1, time.Second) {
		t.Fatal("callback 2 not invoked within timeout")
	}
	if !cb3.waitForCalls(1, time.Second) {
		t.Fatal("callback 3 not invoked within timeout")
	}

	// Verify all three received the same error
	for i, cb := range []*mockCallback{cb1, cb2, cb3} {
		calls := cb.getCalls()
		if len(calls) != 1 {
			t.Errorf("callback %d: expected 1 call, got %d", i+1, len(calls))
		}
		if calls[0].Code != ErrCodeServerError {
			t.Errorf("callback %d: Code = %q, want %q", i+1, calls[0].Code, ErrCodeServerError)
		}
	}
}

func TestUnregisterCallbacks_ClearsAll(t *testing.T) {
	UnregisterErrorCallbacks()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	// Unregister all
	UnregisterErrorCallbacks()

	err := newCriticalError(ErrCodeAuthFailed, "should not be received")
	notifyErrorCallbacks(err)

	// Wait briefly to confirm callback is NOT invoked
	time.Sleep(50 * time.Millisecond)

	calls := cb.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 calls after unregister, got %d", len(calls))
	}
}

func TestCallbackDoesNotBlock(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	// Register a slow callback
	var callCount atomic.Int32
	slowCb := &slowCallback{callCount: &callCount}
	RegisterErrorCallback(slowCb)

	// notifyErrorCallbacks should return immediately (fire and forget)
	start := time.Now()
	err := newCriticalError(ErrCodeNetworkError, "network timeout")
	notifyErrorCallbacks(err)
	elapsed := time.Since(start)

	// Should return almost immediately (< 10ms), not wait for 100ms slow callback
	if elapsed > 50*time.Millisecond {
		t.Errorf("notifyErrorCallbacks blocked for %v, should return immediately", elapsed)
	}

	// Wait for the slow callback to complete
	time.Sleep(200 * time.Millisecond)
	if callCount.Load() != 1 {
		t.Errorf("slow callback invocation count = %d, want 1", callCount.Load())
	}
}

// slowCallback simulates a slow callback handler.
type slowCallback struct {
	callCount *atomic.Int32
}

func (s *slowCallback) OnError(code string, message string, severity int) {
	time.Sleep(100 * time.Millisecond)
	s.callCount.Add(1)
}

func TestRegisterNilCallback_NoOp(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	// Should not panic
	RegisterErrorCallback(nil)

	err := newCriticalError(ErrCodeServerError, "test")
	notifyErrorCallbacks(err)

	// No callbacks should be registered
	time.Sleep(50 * time.Millisecond)
}

func TestNotifyNilError_NoOp(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	// Should not panic or invoke callback
	notifyErrorCallbacks(nil)

	time.Sleep(50 * time.Millisecond)
	calls := cb.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 calls for nil error, got %d", len(calls))
	}
}

func TestLogError_CriticalTriggersCallback(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	err := newCriticalError(ErrCodeAuthFailed, "auth failed via logError")
	logError(err, false)

	if !cb.waitForCalls(1, time.Second) {
		t.Fatal("callback not invoked via logError for critical error")
	}
}

func TestLogError_DebugNoCallback(t *testing.T) {
	UnregisterErrorCallbacks()
	defer UnregisterErrorCallbacks()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	err := newDebugError("DEBUG_TEST", "debug via logError")
	logError(err, true) // Even with debug mode on, debug errors don't trigger callbacks

	time.Sleep(50 * time.Millisecond)
	calls := cb.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 callbacks for debug error, got %d", len(calls))
	}
}

func TestLogError_NilNoOp(t *testing.T) {
	// Should not panic
	logError(nil, true)
	logError(nil, false)
}

func TestErrorSeverity_String(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		want     string
	}{
		{SeverityDebug, "debug"},
		{SeverityWarning, "warning"},
		{SeverityCritical, "critical"},
		{SeverityFatal, "fatal"},
		{ErrorSeverity(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSDKError_Error(t *testing.T) {
	err := &SDKError{Code: "TEST", Message: "test message", Severity: SeverityWarning}
	if err.Error() != "test message" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test message")
	}
}

func TestSDKError_ToJSON(t *testing.T) {
	err := &SDKError{Code: ErrCodeAuthFailed, Message: "auth failed", Severity: SeverityCritical}
	jsonStr := err.ToJSON()

	var parsed SDKError
	if unmarshalErr := json.Unmarshal([]byte(jsonStr), &parsed); unmarshalErr != nil {
		t.Fatalf("ToJSON produced invalid JSON: %v", unmarshalErr)
	}
	if parsed.Code != ErrCodeAuthFailed {
		t.Errorf("Code = %q, want %q", parsed.Code, ErrCodeAuthFailed)
	}
	if parsed.Message != "auth failed" {
		t.Errorf("Message = %q, want %q", parsed.Message, "auth failed")
	}
	if parsed.Severity != SeverityCritical {
		t.Errorf("Severity = %d, want %d", parsed.Severity, SeverityCritical)
	}
}

func TestWrapError(t *testing.T) {
	if got := wrapError(nil); got != "" {
		t.Errorf("wrapError(nil) = %q, want empty", got)
	}

	sdkErr := &SDKError{Message: "test error"}
	if got := wrapError(sdkErr); got != "test error" {
		t.Errorf("wrapError(err) = %q, want %q", got, "test error")
	}
}

func TestConvenienceConstructors(t *testing.T) {
	tests := []struct {
		name     string
		err      *SDKError
		wantCode string
		wantSev  ErrorSeverity
	}{
		{"auth error", newAuthError("auth failed"), ErrCodeAuthFailed, SeverityCritical},
		{"server error", newServerError("500 error"), ErrCodeServerError, SeverityCritical},
		{"rate limit", newRateLimitError("429"), ErrCodeRateLimited, SeverityWarning},
		{"disk full", newDiskFullError("no space"), ErrCodeDiskFull, SeverityCritical},
		{"queue full", newQueueFullError("queue full"), ErrCodeQueueFull, SeverityWarning},
		{"network error", newNetworkError("timeout"), ErrCodeNetworkError, SeverityWarning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.wantCode)
			}
			if tt.err.Severity != tt.wantSev {
				t.Errorf("Severity = %v, want %v", tt.err.Severity, tt.wantSev)
			}
		})
	}
}
