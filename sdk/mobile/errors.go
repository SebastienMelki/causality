package mobile

import (
	"encoding/json"
	"fmt"
	"log"
)

// ErrorSeverity indicates how critical an error is.
type ErrorSeverity int

const (
	// SeverityDebug is informational, logged in debug mode only.
	SeverityDebug ErrorSeverity = iota
	// SeverityWarning is non-critical, SDK continues operating.
	SeverityWarning
	// SeverityCritical is a serious issue, app should handle.
	SeverityCritical
	// SeverityFatal means SDK cannot operate, requires reinitialization.
	SeverityFatal
)

// String returns the human-readable name of the severity level.
func (s ErrorSeverity) String() string {
	switch s {
	case SeverityDebug:
		return "debug"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	case SeverityFatal:
		return "fatal"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Error codes for categorization.
const (
	ErrCodeNotInitialized = "NOT_INITIALIZED"
	ErrCodeInvalidConfig  = "INVALID_CONFIG"
	ErrCodeInvalidJSON    = "INVALID_JSON"
	ErrCodeInvalidEvent   = "INVALID_EVENT"
	ErrCodeNetworkError   = "NETWORK_ERROR"
	ErrCodeAuthFailed     = "AUTH_FAILED"
	ErrCodeDiskFull       = "DISK_FULL"
	ErrCodeDiskError      = "DISK_ERROR"
	ErrCodeQueueFull      = "QUEUE_FULL"
	ErrCodeServerError    = "SERVER_ERROR"
	ErrCodeRateLimited    = "RATE_LIMITED"
)

// SDKError represents a structured error with severity and code.
type SDKError struct {
	Code     string        `json:"code"`
	Message  string        `json:"message"`
	Severity ErrorSeverity `json:"severity"`
}

// Error implements the error interface.
func (e *SDKError) Error() string {
	return e.Message
}

// ToJSON serializes the error to a JSON string for native wrapper consumption.
func (e *SDKError) ToJSON() string {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf(`{"code":"%s","message":"%s","severity":%d}`, e.Code, e.Message, e.Severity)
	}
	return string(data)
}

// newDebugError creates a debug-level error (logged only in debug mode).
func newDebugError(code, message string) *SDKError {
	return &SDKError{Code: code, Message: message, Severity: SeverityDebug}
}

// newWarningError creates a warning-level error.
func newWarningError(code, message string) *SDKError {
	return &SDKError{Code: code, Message: message, Severity: SeverityWarning}
}

// newCriticalError creates a critical-level error.
func newCriticalError(code, message string) *SDKError {
	return &SDKError{Code: code, Message: message, Severity: SeverityCritical}
}

// newFatalError creates a fatal-level error.
func newFatalError(code, message string) *SDKError {
	return &SDKError{Code: code, Message: message, Severity: SeverityFatal}
}

// Convenience constructors for common error scenarios.
// These are used by the HTTP transport (Plan 02-05) and persistence layer (Plan 02-02).

// newAuthError creates an auth failure error (e.g., 401 from server).
func newAuthError(message string) *SDKError {
	return newCriticalError(ErrCodeAuthFailed, message)
}

// newServerError creates a server error (e.g., 5xx from server).
func newServerError(message string) *SDKError {
	return newCriticalError(ErrCodeServerError, message)
}

// newRateLimitError creates a rate limit error (e.g., 429 from server).
func newRateLimitError(message string) *SDKError {
	return newWarningError(ErrCodeRateLimited, message)
}

// newDiskFullError creates a disk full error.
func newDiskFullError(message string) *SDKError {
	return newCriticalError(ErrCodeDiskFull, message)
}

// newQueueFullError creates a queue full error.
func newQueueFullError(message string) *SDKError {
	return newWarningError(ErrCodeQueueFull, message)
}

// newNetworkError creates a network connectivity error.
func newNetworkError(message string) *SDKError {
	return newWarningError(ErrCodeNetworkError, message)
}

// wrapError returns empty string for nil, error message otherwise.
// Used by exported functions that return string instead of error.
func wrapError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// debugLog prints debug output to stderr, which appears in Xcode console and Logcat.
func debugLog(format string, args ...interface{}) {
	log.Printf("[Causality] "+format, args...)
}
