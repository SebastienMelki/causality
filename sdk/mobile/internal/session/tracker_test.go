package session

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testClock provides a controllable clock for deterministic tests.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock(t time.Time) *testClock {
	return &testClock{now: t}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// callbackRecorder captures session start/end callbacks for verification.
type callbackRecorder struct {
	mu        sync.Mutex
	starts    []string
	ends      []endRecord
	startCh   chan string
	endCh     chan endRecord
}

type endRecord struct {
	sessionID  string
	durationMs int64
}

func newCallbackRecorder() *callbackRecorder {
	return &callbackRecorder{
		startCh: make(chan string, 100),
		endCh:   make(chan endRecord, 100),
	}
}

func (r *callbackRecorder) onStart(sessionID string) {
	r.mu.Lock()
	r.starts = append(r.starts, sessionID)
	r.mu.Unlock()
	r.startCh <- sessionID
}

func (r *callbackRecorder) onEnd(sessionID string, durationMs int64) {
	r.mu.Lock()
	r.ends = append(r.ends, endRecord{sessionID: sessionID, durationMs: durationMs})
	r.mu.Unlock()
	r.endCh <- endRecord{sessionID: sessionID, durationMs: durationMs}
}

func (r *callbackRecorder) startCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.starts)
}

func (r *callbackRecorder) endCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ends)
}

func (r *callbackRecorder) lastEnd() endRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.ends) == 0 {
		return endRecord{}
	}
	return r.ends[len(r.ends)-1]
}

// newTestTracker creates a tracker with a test clock and callback recorder.
func newTestTracker(timeout time.Duration) (*Tracker, *testClock, *callbackRecorder) {
	rec := newCallbackRecorder()
	t := NewTracker(timeout, rec.onStart, rec.onEnd)
	clk := newTestClock(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))
	t.setClockForTesting(clk.Now)
	return t, clk, rec
}

func TestNewTracker_DefaultsEnabled(t *testing.T) {
	tracker := NewTracker(0, nil, nil)

	if id := tracker.CurrentSessionID(); id != "" {
		t.Errorf("expected empty session ID initially, got %q", id)
	}

	// Verify enabled by default: RecordActivity should produce a session
	sid := tracker.RecordActivity()
	if sid == "" {
		t.Error("expected RecordActivity to return a session ID when enabled")
	}
}

func TestNewTracker_DefaultTimeout(t *testing.T) {
	tracker := NewTracker(0, nil, nil)
	if tracker.timeout != DefaultTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultTimeout, tracker.timeout)
	}
}

func TestNewTracker_CustomTimeout(t *testing.T) {
	tracker := NewTracker(5 * time.Second, nil, nil)
	if tracker.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", tracker.timeout)
	}
}

func TestRecordActivity_StartsNewSession(t *testing.T) {
	tracker, _, rec := newTestTracker(30 * time.Second)

	sid := tracker.RecordActivity()
	if sid == "" {
		t.Fatal("expected non-empty session ID")
	}

	if rec.startCount() != 1 {
		t.Errorf("expected 1 onSessionStart callback, got %d", rec.startCount())
	}

	if tracker.CurrentSessionID() != sid {
		t.Errorf("CurrentSessionID() = %q, want %q", tracker.CurrentSessionID(), sid)
	}
}

func TestRecordActivity_SameSessionWithinTimeout(t *testing.T) {
	tracker, clk, rec := newTestTracker(30 * time.Second)

	sid1 := tracker.RecordActivity()

	// Advance less than timeout
	clk.Advance(10 * time.Second)
	sid2 := tracker.RecordActivity()

	if sid1 != sid2 {
		t.Errorf("expected same session ID, got %q and %q", sid1, sid2)
	}

	if rec.startCount() != 1 {
		t.Errorf("expected exactly 1 onSessionStart callback, got %d", rec.startCount())
	}

	if rec.endCount() != 0 {
		t.Errorf("expected 0 onSessionEnd callbacks, got %d", rec.endCount())
	}
}

func TestRecordActivity_NewSessionAfterTimeout(t *testing.T) {
	tracker, clk, rec := newTestTracker(30 * time.Second)

	sid1 := tracker.RecordActivity()

	// Advance past timeout
	clk.Advance(31 * time.Second)
	sid2 := tracker.RecordActivity()

	if sid1 == sid2 {
		t.Error("expected different session IDs after timeout")
	}

	if rec.startCount() != 2 {
		t.Errorf("expected 2 onSessionStart callbacks, got %d", rec.startCount())
	}

	if rec.endCount() != 1 {
		t.Errorf("expected 1 onSessionEnd callback, got %d", rec.endCount())
	}

	// Session end should have duration based on last activity
	end := rec.lastEnd()
	if end.sessionID != sid1 {
		t.Errorf("ended session ID = %q, want %q", end.sessionID, sid1)
	}
	// Duration should be 0ms because the session had only one RecordActivity at start
	if end.durationMs != 0 {
		t.Errorf("expected 0ms duration (single activity), got %dms", end.durationMs)
	}
}

func TestRecordActivity_NewSessionAfterTimeout_WithActivity(t *testing.T) {
	tracker, clk, rec := newTestTracker(30 * time.Second)

	tracker.RecordActivity()

	// Some activity within session
	clk.Advance(10 * time.Second)
	tracker.RecordActivity()

	clk.Advance(15 * time.Second)
	tracker.RecordActivity() // last activity at +25s

	// Timeout from last activity
	clk.Advance(31 * time.Second)
	tracker.RecordActivity() // new session

	if rec.endCount() != 1 {
		t.Fatalf("expected 1 onSessionEnd callback, got %d", rec.endCount())
	}

	end := rec.lastEnd()
	// Duration from start to last activity: 25 seconds = 25000ms
	if end.durationMs != 25000 {
		t.Errorf("expected duration 25000ms, got %dms", end.durationMs)
	}
}

func TestAppLifecycle_BackgroundForegroundQuick(t *testing.T) {
	tracker, clk, rec := newTestTracker(30 * time.Second)

	sid := tracker.RecordActivity()

	// Go to background
	tracker.AppDidEnterBackground()

	// Quick return (less than timeout)
	clk.Advance(5 * time.Second)
	tracker.AppWillEnterForeground()

	// Session should continue
	if tracker.CurrentSessionID() != sid {
		t.Error("expected same session after quick background/foreground")
	}

	if rec.endCount() != 0 {
		t.Errorf("expected 0 onSessionEnd callbacks, got %d", rec.endCount())
	}
}

func TestAppLifecycle_BackgroundForegroundLong(t *testing.T) {
	tracker, clk, rec := newTestTracker(30 * time.Second)

	sid1 := tracker.RecordActivity()

	// Go to background
	tracker.AppDidEnterBackground()

	// Long background (exceeds timeout)
	clk.Advance(60 * time.Second)
	tracker.AppWillEnterForeground()

	// Session should be ended
	if tracker.CurrentSessionID() != "" {
		t.Error("expected empty session after long background")
	}

	if rec.endCount() != 1 {
		t.Errorf("expected 1 onSessionEnd callback, got %d", rec.endCount())
	}

	end := rec.lastEnd()
	if end.sessionID != sid1 {
		t.Errorf("ended session = %q, want %q", end.sessionID, sid1)
	}

	// Next RecordActivity should start new session
	sid2 := tracker.RecordActivity()
	if sid2 == "" {
		t.Fatal("expected new session after foreground")
	}
	if sid2 == sid1 {
		t.Error("expected different session ID after long background")
	}

	if rec.startCount() != 2 {
		t.Errorf("expected 2 start callbacks, got %d", rec.startCount())
	}
}

func TestAppLifecycle_NoSessionNoOp(t *testing.T) {
	tracker, _, rec := newTestTracker(30 * time.Second)

	// Foreground without a session should be a no-op
	tracker.AppWillEnterForeground()

	if rec.endCount() != 0 {
		t.Error("expected no callbacks when no session exists")
	}
}

func TestAppLifecycle_NoBackgroundEvent(t *testing.T) {
	tracker, _, rec := newTestTracker(30 * time.Second)

	tracker.RecordActivity()

	// Foreground without a prior background event should be a no-op
	tracker.AppWillEnterForeground()

	if rec.endCount() != 0 {
		t.Error("expected no session end when no background event recorded")
	}
}

func TestSetEnabled_DisablingEndsSession(t *testing.T) {
	tracker, _, rec := newTestTracker(30 * time.Second)

	tracker.RecordActivity()

	tracker.SetEnabled(false)

	if tracker.CurrentSessionID() != "" {
		t.Error("expected empty session after disabling")
	}

	if rec.endCount() != 1 {
		t.Errorf("expected 1 onSessionEnd callback, got %d", rec.endCount())
	}
}

func TestSetEnabled_DisabledRecordActivityReturnsEmpty(t *testing.T) {
	tracker, _, _ := newTestTracker(30 * time.Second)

	tracker.SetEnabled(false)

	sid := tracker.RecordActivity()
	if sid != "" {
		t.Errorf("expected empty session ID when disabled, got %q", sid)
	}
}

func TestSetEnabled_DisabledLifecycleNoOp(t *testing.T) {
	tracker, _, rec := newTestTracker(30 * time.Second)

	tracker.SetEnabled(false)

	tracker.AppDidEnterBackground()
	tracker.AppWillEnterForeground()

	if rec.startCount() != 0 || rec.endCount() != 0 {
		t.Error("expected no callbacks when tracker is disabled")
	}
}

func TestSetEnabled_ReenableThenRecordActivity(t *testing.T) {
	tracker, _, rec := newTestTracker(30 * time.Second)

	tracker.RecordActivity()
	tracker.SetEnabled(false)

	// Re-enable
	tracker.SetEnabled(true)

	sid := tracker.RecordActivity()
	if sid == "" {
		t.Error("expected new session after re-enabling")
	}

	if rec.startCount() != 2 {
		t.Errorf("expected 2 start callbacks (original + re-enabled), got %d", rec.startCount())
	}
}

func TestGetSessionDuration(t *testing.T) {
	tracker, clk, _ := newTestTracker(30 * time.Second)

	// No session
	if d := tracker.GetSessionDuration(); d != 0 {
		t.Errorf("expected 0ms duration with no session, got %d", d)
	}

	tracker.RecordActivity()
	clk.Advance(5 * time.Second)

	d := tracker.GetSessionDuration()
	if d != 5000 {
		t.Errorf("expected 5000ms duration, got %d", d)
	}
}

func TestGetSessionDuration_NoSessionReturnsZero(t *testing.T) {
	tracker, _, _ := newTestTracker(30 * time.Second)

	if d := tracker.GetSessionDuration(); d != 0 {
		t.Errorf("expected 0 for no session, got %d", d)
	}
}

func TestNilCallbacks(t *testing.T) {
	// Tracker should work fine with nil callbacks
	tracker := NewTracker(30*time.Second, nil, nil)
	clk := newTestClock(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))
	tracker.setClockForTesting(clk.Now)

	sid := tracker.RecordActivity()
	if sid == "" {
		t.Error("expected session ID with nil callbacks")
	}

	clk.Advance(60 * time.Second)
	sid2 := tracker.RecordActivity()
	if sid2 == sid {
		t.Error("expected new session after timeout")
	}

	tracker.SetEnabled(false)
	// No panic expected
}

func TestConcurrentAccess(t *testing.T) {
	rec := newCallbackRecorder()
	tracker := NewTracker(30*time.Second, rec.onStart, rec.onEnd)

	var wg sync.WaitGroup
	const goroutines = 100

	// Track session IDs from all goroutines
	sessionIDs := make([]string, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			sessionIDs[idx] = tracker.RecordActivity()
		}(i)
	}
	wg.Wait()

	// All goroutines should get the same session ID (all within timeout)
	first := sessionIDs[0]
	if first == "" {
		t.Fatal("expected non-empty session ID")
	}
	for i, sid := range sessionIDs {
		if sid != first {
			t.Errorf("goroutine %d got session %q, expected %q", i, sid, first)
		}
	}

	// Exactly 1 start callback
	if rec.startCount() != 1 {
		t.Errorf("expected 1 onSessionStart callback from concurrent access, got %d", rec.startCount())
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	rec := newCallbackRecorder()
	tracker := NewTracker(30*time.Second, rec.onStart, rec.onEnd)

	var wg sync.WaitGroup
	const workers = 50

	// Mix of operations concurrently
	wg.Add(workers * 4)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			tracker.RecordActivity()
		}()
		go func() {
			defer wg.Done()
			tracker.CurrentSessionID()
		}()
		go func() {
			defer wg.Done()
			tracker.GetSessionDuration()
		}()
		go func() {
			defer wg.Done()
			tracker.AppDidEnterBackground()
		}()
	}
	wg.Wait()

	// Should not panic or deadlock
}

func TestMultipleSessionRotations(t *testing.T) {
	tracker, clk, rec := newTestTracker(10 * time.Second)

	// Create 5 sessions
	var sessionIDs []string
	for i := 0; i < 5; i++ {
		sid := tracker.RecordActivity()
		sessionIDs = append(sessionIDs, sid)
		clk.Advance(11 * time.Second) // expire each session
	}

	// Should have 5 starts and 4 ends (last session still active)
	if rec.startCount() != 5 {
		t.Errorf("expected 5 starts, got %d", rec.startCount())
	}
	if rec.endCount() != 4 {
		t.Errorf("expected 4 ends, got %d", rec.endCount())
	}

	// All session IDs should be unique
	seen := make(map[string]bool)
	for _, sid := range sessionIDs {
		if seen[sid] {
			t.Errorf("duplicate session ID: %s", sid)
		}
		seen[sid] = true
	}
}

func TestBackgroundWithoutSession(t *testing.T) {
	tracker, _, rec := newTestTracker(30 * time.Second)

	// Background/foreground without ever creating a session
	tracker.AppDidEnterBackground()
	tracker.AppWillEnterForeground()

	if rec.endCount() != 0 {
		t.Error("expected no end callbacks")
	}
	if rec.startCount() != 0 {
		t.Error("expected no start callbacks")
	}
}

func TestEndSessionDurationCalculation(t *testing.T) {
	tracker, clk, rec := newTestTracker(30 * time.Second)

	tracker.RecordActivity()

	// Activity over time
	clk.Advance(5 * time.Second)
	tracker.RecordActivity()
	clk.Advance(10 * time.Second)
	tracker.RecordActivity() // last activity at 15s from start

	// Disable to end session
	tracker.SetEnabled(false)

	if rec.endCount() != 1 {
		t.Fatalf("expected 1 end, got %d", rec.endCount())
	}

	end := rec.lastEnd()
	// Duration from session start to last activity = 15s
	if end.durationMs != 15000 {
		t.Errorf("expected 15000ms duration, got %dms", end.durationMs)
	}
}

func TestConcurrentRecordActivityWithTimeoutRace(t *testing.T) {
	// This test specifically targets the race where multiple goroutines
	// try to rotate a session simultaneously after timeout.
	rec := newCallbackRecorder()
	tracker := NewTracker(10*time.Millisecond, rec.onStart, rec.onEnd)

	var wg sync.WaitGroup
	var startCount atomic.Int64
	const rounds = 20

	for round := 0; round < rounds; round++ {
		// Let the session expire
		time.Sleep(15 * time.Millisecond)

		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func() {
				defer wg.Done()
				sid := tracker.RecordActivity()
				if sid != "" {
					startCount.Add(1)
				}
			}()
		}
		wg.Wait()
	}

	// All calls should have returned a valid session ID
	total := startCount.Load()
	if total != int64(rounds*10) {
		t.Errorf("expected %d successful RecordActivity calls, got %d", rounds*10, total)
	}
}

// --- Session Events Tests ---

func TestSessionStartEvent(t *testing.T) {
	props := SessionStartEvent("session-abc-123")

	if props["session_id"] != "session-abc-123" {
		t.Errorf("session_id = %v, want session-abc-123", props["session_id"])
	}
	if props["event_type"] != "session_start" {
		t.Errorf("event_type = %v, want session_start", props["event_type"])
	}
	if len(props) != 2 {
		t.Errorf("expected 2 properties, got %d", len(props))
	}
}

func TestSessionEndEvent(t *testing.T) {
	props := SessionEndEvent("session-abc-123", 45000)

	if props["session_id"] != "session-abc-123" {
		t.Errorf("session_id = %v, want session-abc-123", props["session_id"])
	}
	if props["event_type"] != "session_end" {
		t.Errorf("event_type = %v, want session_end", props["event_type"])
	}
	if props["duration_ms"] != int64(45000) {
		t.Errorf("duration_ms = %v, want 45000", props["duration_ms"])
	}
	if len(props) != 3 {
		t.Errorf("expected 3 properties, got %d", len(props))
	}
}

func TestSessionEndEvent_ZeroDuration(t *testing.T) {
	props := SessionEndEvent("sid", 0)

	if props["duration_ms"] != int64(0) {
		t.Errorf("duration_ms = %v, want 0", props["duration_ms"])
	}
}
