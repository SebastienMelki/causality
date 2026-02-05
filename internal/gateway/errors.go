package gateway

import "errors"

// Sentinel errors for the gateway package.
var (
	ErrEventRequired   = errors.New("event is required")
	ErrAtLeastOneEvent = errors.New("at least one event is required")

	// Validation errors
	ErrAppIDRequired    = errors.New("app_id is required")
	ErrEventTypeRequired = errors.New("event_type is required (payload must not be empty)")
	ErrTimestampRequired = errors.New("timestamp_ms is required and must be > 0")
	ErrBatchTooLarge     = errors.New("batch exceeds maximum event count")
)
