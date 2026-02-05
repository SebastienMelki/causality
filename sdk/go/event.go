// Package causality provides a Go SDK for sending events to the Causality analytics platform.
package causality

import (
	"os"
	"runtime"
	"time"
)

// SDKVersion is the current version of the Go SDK.
const SDKVersion = "0.1.0"

// Event represents an analytics event to be tracked.
type Event struct {
	// EventType is the type of event (e.g., "screenView", "buttonTap", "customEvent")
	EventType string `json:"event_type"`

	// Timestamp is when the event occurred (RFC3339 format)
	Timestamp string `json:"timestamp,omitempty"`

	// UserID identifies the user who triggered the event
	UserID string `json:"user_id,omitempty"`

	// SessionID identifies the user's session
	SessionID string `json:"session_id,omitempty"`

	// AppID is the application identifier (set by SDK from config if not provided)
	AppID string `json:"app_id,omitempty"`

	// IdempotencyKey is a unique identifier for deduplication (set by SDK)
	IdempotencyKey string `json:"idempotency_key,omitempty"`

	// Metadata contains arbitrary key-value pairs for the event
	Metadata map[string]string `json:"metadata,omitempty"`

	// ServerContext contains server-side context (set by SDK)
	ServerContext *ServerContext `json:"server_context,omitempty"`
}

// ServerContext contains automatically collected server-side information.
type ServerContext struct {
	// Hostname is the server's hostname
	Hostname string `json:"hostname,omitempty"`

	// GoVersion is the Go runtime version
	GoVersion string `json:"go_version,omitempty"`

	// SDKVersion is the SDK version
	SDKVersion string `json:"sdk_version,omitempty"`

	// OS is the operating system
	OS string `json:"os,omitempty"`

	// Arch is the CPU architecture
	Arch string `json:"arch,omitempty"`
}

// collectServerContext gathers server-side context information.
func collectServerContext() *ServerContext {
	hostname, _ := os.Hostname()

	return &ServerContext{
		Hostname:   hostname,
		GoVersion:  runtime.Version(),
		SDKVersion: SDKVersion,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	}
}

// batchRequest represents the request body for the batch endpoint.
type batchRequest struct {
	Events []Event `json:"events"`
}

// now returns the current time in RFC3339 format.
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
