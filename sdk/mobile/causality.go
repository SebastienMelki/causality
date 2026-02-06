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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SebastienMelki/causality/sdk/mobile/internal/batch"
	"github.com/SebastienMelki/causality/sdk/mobile/internal/device"
	"github.com/SebastienMelki/causality/sdk/mobile/internal/identity"
	"github.com/SebastienMelki/causality/sdk/mobile/internal/session"
	"github.com/SebastienMelki/causality/sdk/mobile/internal/storage"
	"github.com/SebastienMelki/causality/sdk/mobile/internal/transport"
	"github.com/google/uuid"
)

// sdkInstance is the package-level singleton.
var (
	sdkMu    sync.RWMutex
	instance *sdk
)

// sdk holds the initialized SDK state with all wired components.
type sdk struct {
	config          *Config
	db              *storage.DB
	queue           *storage.Queue
	idManager       *device.IDManager
	identityManager *identity.IdentityManager
	sessionTracker  *session.Tracker
	batcher         *batch.Batcher
	transportClient *transport.Client
	debugMode       bool

	ctx    context.Context
	cancel context.CancelFunc

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

	// Determine data path for SQLite storage
	dataPath := cfg.DataPath
	if dataPath == "" {
		tmpDir, err := os.MkdirTemp("", "causality-sdk-*")
		if err != nil {
			sdkErr := &SDKError{
				Code:     ErrCodeDiskError,
				Message:  fmt.Sprintf("failed to create temp directory: %s", err.Error()),
				Severity: SeverityFatal,
			}
			notifyErrorCallbacks(sdkErr)
			return sdkErr.Error()
		}
		dataPath = tmpDir
	}

	// Open SQLite database
	dbPath := filepath.Join(dataPath, "causality.db")
	db, err := storage.NewDB(dbPath)
	if err != nil {
		sdkErr := &SDKError{
			Code:     ErrCodeDiskError,
			Message:  fmt.Sprintf("failed to open database: %s", err.Error()),
			Severity: SeverityFatal,
		}
		notifyErrorCallbacks(sdkErr)
		return sdkErr.Error()
	}

	// Create persistent event queue
	queue := storage.NewQueue(db, cfg.MaxQueueSize)

	// Create device ID manager
	idManager := device.NewIDManager(db, cfg.PersistentDeviceID)

	// Create identity manager and restore persisted identity
	identityMgr := identity.NewIdentityManager(db)
	if err := identityMgr.LoadFromDB(); err != nil {
		// Non-fatal: identity will start fresh
		if cfg.DebugMode {
			debugLog("Failed to load persisted identity: %s", err.Error())
		}
	}

	// Create session tracker if enabled
	var sessionTracker *session.Tracker
	if cfg.EnableSessionTracking != nil && *cfg.EnableSessionTracking {
		timeout := time.Duration(cfg.SessionTimeoutMs) * time.Millisecond
		sessionTracker = session.NewTracker(timeout, nil, nil)
	}

	// Create HTTP transport client
	transportClient := transport.NewClient(
		cfg.Endpoint,
		cfg.APIKey,
		30*time.Second, // HTTP request timeout
		nil,            // Use default retry strategy
	)

	// Create context for background operations
	ctx, cancel := context.WithCancel(context.Background())

	// Create batcher with flush loop
	flushInterval := time.Duration(cfg.FlushIntervalMs) * time.Millisecond
	batcher := batch.NewBatcher(queue, transportClient, cfg.BatchSize, flushInterval)
	batcher.StartFlushLoop(ctx)

	sdkMu.Lock()
	instance = &sdk{
		config:          cfg,
		db:              db,
		queue:           queue,
		idManager:       idManager,
		identityManager: identityMgr,
		sessionTracker:  sessionTracker,
		batcher:         batcher,
		transportClient: transportClient,
		debugMode:       cfg.DebugMode,
		ctx:             ctx,
		cancel:          cancel,
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
// Track automatically injects metadata into every event:
//   - device_id from the device ID manager
//   - session_id from the session tracker (if enabled)
//   - user_id from the identity manager (if set)
//   - idempotency_key (generated UUID)
//   - timestamp (UTC RFC3339Nano)
//   - app_id from config
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

	// Generate idempotency key
	idempotencyKey := uuid.New().String()

	// Inject metadata
	event.Metadata = EventMetadata{
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotencyKey,
		AppID:          inst.config.AppID,
	}

	// Inject device_id from ID manager
	event.Metadata.DeviceID = inst.idManager.GetOrCreateDeviceID()

	// Inject session_id from session tracker (if enabled)
	if inst.sessionTracker != nil {
		event.Metadata.SessionID = inst.sessionTracker.RecordActivity()
	}

	// Inject user_id from identity manager (if set)
	user := inst.identityManager.GetUser()
	if user != nil {
		event.Metadata.UserID = user.UserID
	}

	if inst.debugMode {
		debugLog("Track: type=%s, idempotency_key=%s, device_id=%s, session_id=%s",
			event.Type, event.Metadata.IdempotencyKey, event.Metadata.DeviceID, event.Metadata.SessionID)
	}

	// Serialize event with all metadata
	eventData, err := json.Marshal(event)
	if err != nil {
		sdkErr := &SDKError{
			Code:     ErrCodeInvalidJSON,
			Message:  fmt.Sprintf("failed to serialize event: %s", err.Error()),
			Severity: SeverityWarning,
		}
		logError(sdkErr, inst.debugMode)
		return sdkErr.Error()
	}

	// Enqueue via batcher
	if err := inst.batcher.Add(string(eventData), idempotencyKey); err != nil {
		sdkErr := &SDKError{
			Code:     ErrCodeDiskError,
			Message:  fmt.Sprintf("failed to enqueue event: %s", err.Error()),
			Severity: SeverityWarning,
		}
		logError(sdkErr, inst.debugMode)
		return sdkErr.Error()
	}

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

	// Convert bridge traits (map[string]string) to identity traits (map[string]interface{})
	var traits map[string]interface{}
	if user.Traits != nil {
		traits = make(map[string]interface{}, len(user.Traits))
		for k, v := range user.Traits {
			traits[k] = v
		}
	}

	if err := inst.identityManager.SetUser(user.UserID, traits, user.Aliases); err != nil {
		sdkErr := &SDKError{
			Code:     ErrCodeDiskError,
			Message:  fmt.Sprintf("failed to persist user identity: %s", err.Error()),
			Severity: SeverityWarning,
		}
		logError(sdkErr, inst.debugMode)
		return sdkErr.Error()
	}

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

	inst.identityManager.Reset()

	if inst.debugMode {
		debugLog("Reset: user identity cleared")
	}

	return ""
}

// ResetAll performs a full reset: clears user identity, regenerates device ID,
// clears the event queue, and ends the session.
// Use this for complete logout / privacy reset scenarios.
// Returns empty string on success, or an error message on failure.
func ResetAll() string {
	inst := getInstance()
	if inst == nil {
		return notInitializedError()
	}

	// Clear user identity
	inst.identityManager.Reset()

	// Regenerate device ID
	inst.idManager.RegenerateDeviceID()

	// Clear event queue
	if err := inst.queue.Clear(); err != nil {
		if inst.debugMode {
			debugLog("ResetAll: failed to clear queue: %s", err.Error())
		}
	}

	// End session (disable and re-enable to force session rotation)
	if inst.sessionTracker != nil {
		inst.sessionTracker.SetEnabled(false)
		inst.sessionTracker.SetEnabled(true)
	}

	if inst.debugMode {
		debugLog("ResetAll: user, device ID, queue, and session cleared")
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

	if err := inst.batcher.Flush(inst.ctx); err != nil {
		sdkErr := &SDKError{
			Code:     ErrCodeNetworkError,
			Message:  fmt.Sprintf("flush failed: %s", err.Error()),
			Severity: SeverityWarning,
		}
		logError(sdkErr, inst.debugMode)
		return sdkErr.Error()
	}

	return ""
}

// GetDeviceId returns the current device identifier.
// Returns empty string if SDK is not initialized.
func GetDeviceId() string {
	inst := getInstance()
	if inst == nil {
		return ""
	}

	return inst.idManager.GetOrCreateDeviceID()
}

// GetSessionId returns the current session identifier.
// Returns empty string if no session is active or SDK is not initialized.
func GetSessionId() string {
	inst := getInstance()
	if inst == nil {
		return ""
	}

	if inst.sessionTracker == nil {
		return ""
	}

	return inst.sessionTracker.CurrentSessionID()
}

// GetUserId returns the current user identifier.
// Returns empty string if no user is set or SDK is not initialized.
func GetUserId() string {
	inst := getInstance()
	if inst == nil {
		return ""
	}

	user := inst.identityManager.GetUser()
	if user == nil {
		return ""
	}
	return user.UserID
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

// AppDidEnterBackground notifies the SDK that the app went to background.
// This triggers a flush of queued events and records the background transition
// for session tracking.
// Returns empty string on success, or an error message on failure.
func AppDidEnterBackground() string {
	inst := getInstance()
	if inst == nil {
		return notInitializedError()
	}

	// Notify session tracker
	if inst.sessionTracker != nil {
		inst.sessionTracker.AppDidEnterBackground()
	}

	// Trigger a flush to send queued events while we can
	if err := inst.batcher.Flush(inst.ctx); err != nil {
		if inst.debugMode {
			debugLog("AppDidEnterBackground: flush failed: %s", err.Error())
		}
	}

	if inst.debugMode {
		debugLog("AppDidEnterBackground: recorded")
	}

	return ""
}

// AppWillEnterForeground notifies the SDK that the app is returning from background.
// If the background duration exceeded the session timeout, the current session ends
// and a new one will be started on the next Track call.
// Returns empty string on success, or an error message on failure.
func AppWillEnterForeground() string {
	inst := getInstance()
	if inst == nil {
		return notInitializedError()
	}

	// Notify session tracker
	if inst.sessionTracker != nil {
		inst.sessionTracker.AppWillEnterForeground()
	}

	if inst.debugMode {
		debugLog("AppWillEnterForeground: recorded")
	}

	return ""
}

// SetPlatformContext sets platform-specific device information.
// Called by native wrappers (Swift/Kotlin) during SDK initialization.
// All parameters use gomobile-compatible types.
func SetPlatformContext(platform, osVersion, model, manufacturer, appVersion, buildNumber string, screenW, screenH int, locale, timezone string) {
	device.SetPlatformContext(platform, osVersion, model, manufacturer, appVersion, buildNumber, screenW, screenH, locale, timezone)
}

// SetNetworkInfo updates the carrier and network type information.
// Called by native wrappers when network conditions change.
func SetNetworkInfo(carrier, networkType string) {
	device.SetNetworkInfo(carrier, networkType)
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
	inst := instance
	instance = nil
	sdkMu.Unlock()

	// Clean up components if they exist
	if inst != nil {
		if inst.cancel != nil {
			inst.cancel()
		}
		if inst.batcher != nil {
			inst.batcher.Stop()
		}
		if inst.db != nil {
			inst.db.Close()
		}
	}

	errorCallbacksMu.Lock()
	errorCallbacks = nil
	errorCallbacksMu.Unlock()
}
