package mobile

import (
	"sync"
)

// ErrorCallback is invoked when critical errors occur in the SDK.
// This interface is gomobile-compatible (single method with basic types).
//
// Parameters:
//   - code: Error code (e.g., "AUTH_FAILED", "DISK_FULL")
//   - message: Human-readable error message
//   - severity: 0=debug, 1=warning, 2=critical, 3=fatal
//
// This allows the app to:
//   - Show user-facing messages for auth failures
//   - Handle disk full by prompting user to free space
//   - Log to crash reporting services (Crashlytics, Sentry)
type ErrorCallback interface {
	OnError(code string, message string, severity int)
}

var (
	errorCallbacksMu sync.RWMutex
	errorCallbacks   []ErrorCallback
)

// RegisterErrorCallback adds a callback for critical error notifications.
// Native wrappers call this with platform-specific callback implementations.
// Multiple callbacks can be registered; all will be notified.
func RegisterErrorCallback(callback ErrorCallback) {
	if callback == nil {
		return
	}
	errorCallbacksMu.Lock()
	defer errorCallbacksMu.Unlock()
	errorCallbacks = append(errorCallbacks, callback)
}

// UnregisterErrorCallbacks clears all registered callbacks.
func UnregisterErrorCallbacks() {
	errorCallbacksMu.Lock()
	defer errorCallbacksMu.Unlock()
	errorCallbacks = nil
}

// notifyErrorCallbacks dispatches an error to all registered callbacks.
// Only called for Warning+ severity (not Debug).
// Callbacks are invoked asynchronously to avoid blocking the caller.
func notifyErrorCallbacks(err *SDKError) {
	if err == nil || err.Severity < SeverityWarning {
		return
	}

	errorCallbacksMu.RLock()
	callbacks := make([]ErrorCallback, len(errorCallbacks))
	copy(callbacks, errorCallbacks)
	errorCallbacksMu.RUnlock()

	for _, cb := range callbacks {
		// Fire and forget - don't block on callbacks
		go cb.OnError(err.Code, err.Message, int(err.Severity))
	}
}

// logError logs errors based on severity and debug mode,
// and notifies callbacks for critical+ errors.
func logError(err *SDKError, debugMode bool) {
	if err == nil {
		return
	}

	// Debug severity only logged in debug mode
	if err.Severity == SeverityDebug && !debugMode {
		return
	}

	// Log the error
	switch err.Severity {
	case SeverityDebug:
		debugLog("DEBUG: %s - %s", err.Code, err.Message)
	case SeverityWarning:
		debugLog("WARN: %s - %s", err.Code, err.Message)
	case SeverityCritical, SeverityFatal:
		debugLog("ERROR: %s - %s", err.Code, err.Message)
		notifyErrorCallbacks(err)
	}
}
