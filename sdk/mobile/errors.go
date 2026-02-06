package mobile

import "fmt"

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

// Error codes for categorization.
const (
	ErrCodeNotInitialized = "NOT_INITIALIZED"
	ErrCodeInvalidConfig  = "INVALID_CONFIG"
	ErrCodeInvalidJSON    = "INVALID_JSON"
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

// wrapError returns empty string for nil, error message otherwise.
// Used by exported functions that return string instead of error.
func wrapError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// debugLog is a simple logging helper for debug output.
// Uses fmt.Sprintf for formatting. Platform-specific logging
// is handled by native wrappers.
func debugLog(format string, args ...interface{}) {
	// In debug mode, format the message. Platform-specific native wrappers
	// (Swift os_log, Kotlin Logcat) will override this via callbacks.
	_ = fmt.Sprintf(format, args...)
}
