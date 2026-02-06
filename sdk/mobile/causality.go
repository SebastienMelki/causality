// Package mobile provides the Go core for the Causality mobile SDK.
//
// This package is designed to be compiled with gomobile bind to produce
// .xcframework (iOS) and .aar (Android) libraries. All exported functions
// use only gomobile-compatible types: string, int, int32, int64, float32,
// float64, bool, and error.
//
// Complex data (config, events, user identity) passes as JSON strings
// through the bridge layer. Native wrappers (Swift/Kotlin) provide
// idiomatic typed APIs that serialize to JSON internally.
package mobile

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// sdkInstance is the package-level singleton.
var (
	sdkMu    sync.RWMutex
	instance *sdk
)

// sdk holds the initialized SDK state.
type sdk struct {
	config    *Config
	deviceID  string
	sessionID string
	userID    string
	debugMode bool

	mu sync.RWMutex
}

// Init initializes the SDK with a JSON configuration string.
// Returns empty string on success, or an error message on failure.
// Must be called before any other SDK function.
//
// Example config JSON:
//
//	{"api_key": "key123", "endpoint": "https://analytics.example.com", "app_id": "my-app"}
func Init(configJSON string) string {
	cfg, err := parseConfig(configJSON)
	if err != nil {
		sdkErr := &SDKError{
			Code:     ErrCodeInvalidConfig,
			Message:  err.Error(),
			Severity: SeverityFatal,
		}
		notifyErrorCallbacks(sdkErr)
		return sdkErr.Error()
	}

	// Generate device ID
	deviceID := uuid.New().String()

	sdkMu.Lock()
	instance = &sdk{
		config:    cfg,
		deviceID:  deviceID,
		debugMode: cfg.DebugMode,
	}
	sdkMu.Unlock()

	if cfg.DebugMode {
		debugLog("SDK initialized for app %s at %s", cfg.AppID, cfg.Endpoint)
	}

	return ""
}

// Track enqueues an event for asynchronous batch sending.
// The eventJSON string should be a serialized Event with type and properties.
// Returns empty string on success, or an error message on failure.
//
// Example event JSON:
//
//	{"type": "screen_view", "properties": {"screen_name": "Home"}}
func Track(eventJSON string) string {
	inst := getInstance()
	if inst == nil {
		return notInitializedError()
	}

	event, err := parseEvent(eventJSON)
	if err != nil {
		sdkErr := &SDKError{
			Code:     ErrCodeInvalidJSON,
			Message:  fmt.Sprintf("invalid event: %s", err.Error()),
			Severity: SeverityWarning,
		}
		logError(sdkErr, inst.debugMode)
		return sdkErr.Error()
	}

	// Inject metadata
	inst.mu.RLock()
	event.Metadata = EventMetadata{
		SessionID:      inst.sessionID,
		DeviceID:       inst.deviceID,
		UserID:         inst.userID,
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: uuid.New().String(),
		AppID:          inst.config.AppID,
	}
	inst.mu.RUnlock()

	if inst.debugMode {
		debugLog("Track: type=%s, idempotency_key=%s", event.Type, event.Metadata.IdempotencyKey)
	}

	// Event is prepared for queueing.
	// The actual queue (SQLite persistence) will be added in Plan 02-02.
	_ = event

	return ""
}

// TrackTyped tracks a typed event, validating the event type against known types.
// eventType is the event type constant (e.g., "screen_view").
// eventJSON is the serialized typed event properties.
// Returns empty string on success, or an error message on failure.
//
// Example:
//
//	TrackTyped("screen_view", `{"screen_name": "Home", "screen_class": "HomeViewController"}`)
func TrackTyped(eventType string, eventJSON string) string {
	if !isValidEventType(eventType) {
		return fmt.Sprintf("unknown event type: %s", eventType)
	}

	// Build full event JSON with type wrapper
	fullJSON := fmt.Sprintf(`{"type":%q,"properties":%s}`, eventType, eventJSON)
	return Track(fullJSON)
}

// SetUser sets the user identity for subsequent events.
// The userJSON string should contain user_id and optional traits/aliases.
// Returns empty string on success, or an error message on failure.
//
// Example:
//
//	SetUser(`{"user_id": "user-123", "traits": {"name": "Alice", "plan": "premium"}}`)
func SetUser(userJSON string) string {
	inst := getInstance()
	if inst == nil {
		return notInitializedError()
	}

	user, err := parseUser(userJSON)
	if err != nil {
		sdkErr := &SDKError{
			Code:     ErrCodeInvalidJSON,
			Message:  fmt.Sprintf("invalid user: %s", err.Error()),
			Severity: SeverityWarning,
		}
		logError(sdkErr, inst.debugMode)
		return sdkErr.Error()
	}

	inst.mu.Lock()
	inst.userID = user.UserID
	inst.mu.Unlock()

	if inst.debugMode {
		debugLog("SetUser: user_id=%s", user.UserID)
	}

	return ""
}

// Reset clears the current user identity but preserves the device ID and session.
// This is a "soft reset" for user logout scenarios.
// Returns empty string on success, or an error message on failure.
func Reset() string {
	inst := getInstance()
	if inst == nil {
		return notInitializedError()
	}

	inst.mu.Lock()
	inst.userID = ""
	inst.mu.Unlock()

	if inst.debugMode {
		debugLog("Reset: user identity cleared")
	}

	return ""
}

// ResetAll performs a full reset: clears user identity and regenerates device ID.
// Use this for complete logout / privacy reset scenarios.
// Returns empty string on success, or an error message on failure.
func ResetAll() string {
	inst := getInstance()
	if inst == nil {
		return notInitializedError()
	}

	inst.mu.Lock()
	inst.userID = ""
	inst.deviceID = uuid.New().String()
	inst.sessionID = ""
	inst.mu.Unlock()

	if inst.debugMode {
		debugLog("ResetAll: user, device ID, and session cleared")
	}

	return ""
}

// Flush forces an immediate flush of all queued events.
// Returns empty string on success, or an error message on failure.
func Flush() string {
	inst := getInstance()
	if inst == nil {
		return notInitializedError()
	}

	if inst.debugMode {
		debugLog("Flush: force flush requested")
	}

	// Actual flush implementation will be added in Plan 02-05.
	return ""
}

// GetDeviceId returns the current device identifier.
// Returns empty string if SDK is not initialized.
func GetDeviceId() string {
	inst := getInstance()
	if inst == nil {
		return ""
	}

	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.deviceID
}

// GetSessionId returns the current session identifier.
// Returns empty string if no session is active or SDK is not initialized.
func GetSessionId() string {
	inst := getInstance()
	if inst == nil {
		return ""
	}

	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.sessionID
}

// GetUserId returns the current user identifier.
// Returns empty string if no user is set or SDK is not initialized.
func GetUserId() string {
	inst := getInstance()
	if inst == nil {
		return ""
	}

	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.userID
}

// IsInitialized returns true if the SDK has been initialized.
func IsInitialized() bool {
	return getInstance() != nil
}

// SetDebugMode toggles debug logging at runtime.
func SetDebugMode(enabled bool) {
	inst := getInstance()
	if inst == nil {
		return
	}

	inst.mu.Lock()
	inst.debugMode = enabled
	inst.mu.Unlock()
}

// getInstance returns the SDK singleton, or nil if not initialized.
func getInstance() *sdk {
	sdkMu.RLock()
	defer sdkMu.RUnlock()
	return instance
}

// notInitializedError returns and notifies about the not-initialized error.
func notInitializedError() string {
	sdkErr := &SDKError{
		Code:     ErrCodeNotInitialized,
		Message:  "SDK not initialized: call Init() first",
		Severity: SeverityFatal,
	}
	notifyErrorCallbacks(sdkErr)
	return sdkErr.Error()
}

// resetForTesting resets the SDK state for unit tests.
// This is not exported and not available via gomobile.
func resetForTesting() {
	sdkMu.Lock()
	instance = nil
	sdkMu.Unlock()

	errorCallbacksMu.Lock()
	errorCallbacks = nil
	errorCallbacksMu.Unlock()
}
