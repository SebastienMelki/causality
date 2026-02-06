// Package session provides hybrid session tracking that combines timeout-based
// and lifecycle-based session detection for mobile analytics.
//
// A session represents a period of user engagement. The hybrid approach:
//   - Timeout: session expires after configurable inactivity period (default 30s)
//   - Lifecycle: background/foreground transitions trigger session boundaries
//
// This is the industry standard approach used by major analytics SDKs.
package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultTimeout is the default session inactivity timeout.
// A new session starts when the user returns after this period.
const DefaultTimeout = 30 * time.Second

// clockFunc is a function that returns the current time.
// Default is time.Now; tests inject a controllable clock.
type clockFunc func() time.Time

// OnSessionStart is called when a new session begins.
type OnSessionStart func(sessionID string)

// OnSessionEnd is called when a session ends.
// durationMs is the session duration in milliseconds (from start to last activity).
type OnSessionEnd func(sessionID string, durationMs int64)

// Tracker manages session lifecycle using hybrid timeout + lifecycle detection.
// It is safe for concurrent use by multiple goroutines.
type Tracker struct {
	mu sync.Mutex

	sessionID      string
	sessionStart   time.Time
	lastActivity   time.Time
	backgroundedAt time.Time

	timeout time.Duration
	enabled bool

	onSessionStart OnSessionStart
	onSessionEnd   OnSessionEnd

	clock clockFunc
}

// NewTracker creates a session tracker with the given timeout and callbacks.
// If timeout is zero, DefaultTimeout is used.
// Callbacks may be nil if not needed.
func NewTracker(timeout time.Duration, onStart OnSessionStart, onEnd OnSessionEnd) *Tracker {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Tracker{
		timeout:        timeout,
		enabled:        true,
		onSessionStart: onStart,
		onSessionEnd:   onEnd,
		clock:          time.Now,
	}
}

// RecordActivity records user activity and returns the current session ID.
// If no session is active or the current session has expired, a new session
// is started (ending the old one if it existed).
//
// This is the primary method called on every tracked event.
func (t *Tracker) RecordActivity() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled {
		return ""
	}

	now := t.clock()

	// If session exists and not expired, update activity and return
	if t.sessionID != "" && !t.isExpired(now) {
		t.lastActivity = now
		return t.sessionID
	}

	// Session expired or doesn't exist -- rotate
	if t.sessionID != "" {
		t.endSessionLocked()
	}

	return t.startSessionLocked(now)
}

// AppDidEnterBackground records the time the app went to background.
// It does NOT end the session immediately -- the user might return quickly.
func (t *Tracker) AppDidEnterBackground() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled {
		return
	}

	t.backgroundedAt = t.clock()
}

// AppWillEnterForeground handles the app returning from background.
// If the background duration exceeds the timeout, the old session is ended.
// The next RecordActivity call will start a new session.
func (t *Tracker) AppWillEnterForeground() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled {
		return
	}

	// No session to expire
	if t.sessionID == "" {
		return
	}

	// If backgroundedAt is zero, treat as no background event recorded
	if t.backgroundedAt.IsZero() {
		return
	}

	now := t.clock()
	backgroundDuration := now.Sub(t.backgroundedAt)

	if backgroundDuration > t.timeout {
		t.endSessionLocked()
	}

	// Reset backgroundedAt
	t.backgroundedAt = time.Time{}
}

// CurrentSessionID returns the active session ID, or empty string if none.
func (t *Tracker) CurrentSessionID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessionID
}

// SetEnabled enables or disables session tracking.
// Disabling ends the current session if one is active.
func (t *Tracker) SetEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !enabled && t.sessionID != "" {
		t.endSessionLocked()
	}
	t.enabled = enabled
}

// GetSessionDuration returns the current session duration in milliseconds.
// Returns 0 if no session is active.
func (t *Tracker) GetSessionDuration() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.sessionID == "" {
		return 0
	}

	return t.clock().Sub(t.sessionStart).Milliseconds()
}

// isExpired checks if the session has timed out based on last activity.
// Must be called with mu held.
func (t *Tracker) isExpired(now time.Time) bool {
	return now.Sub(t.lastActivity) > t.timeout
}

// startSessionLocked creates a new session. Must be called with mu held.
func (t *Tracker) startSessionLocked(now time.Time) string {
	t.sessionID = uuid.New().String()
	t.sessionStart = now
	t.lastActivity = now
	t.backgroundedAt = time.Time{}

	if t.onSessionStart != nil {
		t.onSessionStart(t.sessionID)
	}

	return t.sessionID
}

// endSessionLocked ends the current session. Must be called with mu held.
func (t *Tracker) endSessionLocked() {
	if t.sessionID == "" {
		return
	}

	durationMs := t.lastActivity.Sub(t.sessionStart).Milliseconds()

	if t.onSessionEnd != nil {
		t.onSessionEnd(t.sessionID, durationMs)
	}

	t.sessionID = ""
	t.sessionStart = time.Time{}
	t.lastActivity = time.Time{}
}

// setClockForTesting replaces the clock function for deterministic tests.
// This is not exported and not available outside the package.
func (t *Tracker) setClockForTesting(clock clockFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clock = clock
}
