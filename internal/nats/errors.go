package nats

import "errors"

// Sentinel errors for the nats package.
var (
	ErrNotConnected     = errors.New("NATS is not connected")
	ErrPartialPublish   = errors.New("failed to publish some events")
)
