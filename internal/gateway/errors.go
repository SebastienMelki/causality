package gateway

import "errors"

// Sentinel errors for the gateway package.
var (
	ErrEventRequired      = errors.New("event is required")
	ErrAtLeastOneEvent    = errors.New("at least one event is required")
)
