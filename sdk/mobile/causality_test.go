package mobile

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const testTimeout = time.Second

// validConfigJSON returns a minimal valid config JSON string.
func validConfigJSON() string {
	return `{"api_key": "test-key", "endpoint": "https://api.example.com", "app_id": "test-app"}`
}

func TestInit_ValidConfig(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := Init(validConfigJSON())
	if result != "" {
		t.Fatalf("Init returned error: %s", result)
	}

	if !IsInitialized() {
		t.Error("IsInitialized() = false after Init")
	}
}

func TestInit_InvalidConfig(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := Init(`{"api_key": ""}`)
	if result == "" {
		t.Fatal("Init should return error for invalid config")
	}
	if !strings.Contains(result, "api_key is required") {
		t.Errorf("error = %q, want to contain 'api_key is required'", result)
	}

	if IsInitialized() {
		t.Error("IsInitialized() = true after failed Init")
	}
}

func TestInit_InvalidJSON(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := Init("not json")
	if result == "" {
		t.Fatal("Init should return error for invalid JSON")
	}
}

func TestInit_TriggersErrorCallback_OnFailure(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	cb := newMockCallback()
	RegisterErrorCallback(cb)

	Init(`{"api_key": ""}`)

	// Error callback should be notified for fatal config error
	if !cb.waitForCalls(1, testTimeout) {
		t.Fatal("callback not invoked for invalid config Init")
	}

	calls := cb.getCalls()
	if calls[0].Code != ErrCodeInvalidConfig {
		t.Errorf("Code = %q, want %q", calls[0].Code, ErrCodeInvalidConfig)
	}
}

func TestInit_GeneratesDeviceId(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	deviceID := GetDeviceId()
	if deviceID == "" {
		t.Error("GetDeviceId() returned empty after Init")
	}
	// Should be a UUID format
	if len(deviceID) != 36 {
		t.Errorf("DeviceId length = %d, want 36 (UUID format)", len(deviceID))
	}
}

func TestInit_DebugMode(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := Init(`{"api_key": "key", "endpoint": "https://api.example.com", "app_id": "app", "debug_mode": true}`)
	if result != "" {
		t.Fatalf("Init returned error: %s", result)
	}
}

func TestInit_CreatesComponents(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	inst := getInstance()
	if inst == nil {
		t.Fatal("instance is nil after Init")
	}
	if inst.db == nil {
		t.Error("db is nil")
	}
	if inst.queue == nil {
		t.Error("queue is nil")
	}
	if inst.idManager == nil {
		t.Error("idManager is nil")
	}
	if inst.identityManager == nil {
		t.Error("identityManager is nil")
	}
	if inst.sessionTracker == nil {
		t.Error("sessionTracker is nil (session tracking enabled by default)")
	}
	if inst.batcher == nil {
		t.Error("batcher is nil")
	}
	if inst.transportClient == nil {
		t.Error("transportClient is nil")
	}
}

func TestInit_SessionTrackingDisabled(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := Init(`{"api_key": "key", "endpoint": "https://api.example.com", "app_id": "app", "enable_session_tracking": false}`)
	if result != "" {
		t.Fatalf("Init returned error: %s", result)
	}

	inst := getInstance()
	if inst.sessionTracker != nil {
		t.Error("sessionTracker should be nil when session tracking disabled")
	}
}

func TestTrack_ValidEvent(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)
	if result != "" {
		t.Fatalf("Track returned error: %s", result)
	}
}

func TestTrack_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := Track(`{"type": "screen_view"}`)
	if result == "" {
		t.Fatal("Track should return error when not initialized")
	}
	if !strings.Contains(result, "not initialized") {
		t.Errorf("error = %q, want to contain 'not initialized'", result)
	}
}

func TestTrack_InvalidJSON(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := Track("invalid json")
	if result == "" {
		t.Fatal("Track should return error for invalid JSON")
	}
}

func TestTrack_MissingType(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := Track(`{"properties": {"key": "value"}}`)
	if result == "" {
		t.Fatal("Track should return error for missing event type")
	}
}

func TestTrack_InjectsDeviceID(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	// Track an event
	result := Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)
	if result != "" {
		t.Fatalf("Track returned error: %s", result)
	}

	// Verify device_id was injected by checking the queue
	inst := getInstance()
	events, err := inst.queue.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("no events in queue after Track")
	}

	var event Event
	if err := json.Unmarshal([]byte(events[0].EventJSON), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Metadata.DeviceID == "" {
		t.Error("device_id not injected into event metadata")
	}
	if len(event.Metadata.DeviceID) != 36 {
		t.Errorf("device_id length = %d, want 36 (UUID format)", len(event.Metadata.DeviceID))
	}

	// Device ID should match GetDeviceId()
	if event.Metadata.DeviceID != GetDeviceId() {
		t.Errorf("device_id = %q, GetDeviceId() = %q, want equal", event.Metadata.DeviceID, GetDeviceId())
	}
}

func TestTrack_InjectsSessionID(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	// Session tracking enabled by default
	Init(validConfigJSON())

	// Track an event
	result := Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)
	if result != "" {
		t.Fatalf("Track returned error: %s", result)
	}

	// Verify session_id was injected
	inst := getInstance()
	events, err := inst.queue.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("no events in queue after Track")
	}

	var event Event
	if err := json.Unmarshal([]byte(events[0].EventJSON), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Metadata.SessionID == "" {
		t.Error("session_id not injected into event metadata")
	}
	if len(event.Metadata.SessionID) != 36 {
		t.Errorf("session_id length = %d, want 36 (UUID format)", len(event.Metadata.SessionID))
	}
}

func TestTrack_InjectsUserID(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	SetUser(`{"user_id": "user-42"}`)

	// Track an event
	result := Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)
	if result != "" {
		t.Fatalf("Track returned error: %s", result)
	}

	// Verify user_id was injected
	inst := getInstance()
	events, err := inst.queue.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("no events in queue after Track")
	}

	var event Event
	if err := json.Unmarshal([]byte(events[0].EventJSON), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Metadata.UserID != "user-42" {
		t.Errorf("user_id = %q, want %q", event.Metadata.UserID, "user-42")
	}
}

func TestTrack_InjectsIdempotencyKey(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)

	inst := getInstance()
	events, err := inst.queue.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("no events in queue")
	}

	var event Event
	if err := json.Unmarshal([]byte(events[0].EventJSON), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Metadata.IdempotencyKey == "" {
		t.Error("idempotency_key not injected")
	}
	if len(event.Metadata.IdempotencyKey) != 36 {
		t.Errorf("idempotency_key length = %d, want 36 (UUID)", len(event.Metadata.IdempotencyKey))
	}
}

func TestTrack_InjectsTimestamp(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	before := time.Now().UTC()
	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)
	after := time.Now().UTC()

	inst := getInstance()
	events, err := inst.queue.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}

	var event Event
	if err := json.Unmarshal([]byte(events[0].EventJSON), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	ts, err := time.Parse(time.RFC3339Nano, event.Metadata.Timestamp)
	if err != nil {
		t.Fatalf("failed to parse timestamp %q: %v", event.Metadata.Timestamp, err)
	}

	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v not between %v and %v", ts, before, after)
	}
}

func TestTrack_InjectsAppID(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)

	inst := getInstance()
	events, err := inst.queue.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}

	var event Event
	if err := json.Unmarshal([]byte(events[0].EventJSON), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Metadata.AppID != "test-app" {
		t.Errorf("app_id = %q, want %q", event.Metadata.AppID, "test-app")
	}
}

func TestTrack_NoUserID_WhenNotSet(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)

	inst := getInstance()
	events, err := inst.queue.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}

	var event Event
	if err := json.Unmarshal([]byte(events[0].EventJSON), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Metadata.UserID != "" {
		t.Errorf("user_id = %q, want empty when no user set", event.Metadata.UserID)
	}
}

func TestTrack_NoSessionID_WhenDisabled(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(`{"api_key": "key", "endpoint": "https://api.example.com", "app_id": "app", "enable_session_tracking": false}`)
	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)

	inst := getInstance()
	events, err := inst.queue.DequeueBatch(1)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}

	var event Event
	if err := json.Unmarshal([]byte(events[0].EventJSON), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Metadata.SessionID != "" {
		t.Errorf("session_id = %q, want empty when session tracking disabled", event.Metadata.SessionID)
	}
}

func TestTrack_EventEnqueued(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)
	Track(`{"type": "button_tap", "properties": {"button_id": "btn1"}}`)

	inst := getInstance()
	count, err := inst.queue.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("queue count = %d, want 2", count)
	}
}

func TestTrackTyped_ValidType(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := TrackTyped("screen_view", `{"screen_name": "Dashboard"}`)
	if result != "" {
		t.Fatalf("TrackTyped returned error: %s", result)
	}
}

func TestTrackTyped_UnknownType(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := TrackTyped("unknown_type", `{"key": "value"}`)
	if result == "" {
		t.Fatal("TrackTyped should return error for unknown type")
	}
	if !strings.Contains(result, "unknown event type") {
		t.Errorf("error = %q, want to contain 'unknown event type'", result)
	}
}

func TestSetUser_Valid(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := SetUser(`{"user_id": "user-123", "traits": {"name": "Alice"}}`)
	if result != "" {
		t.Fatalf("SetUser returned error: %s", result)
	}

	if userID := GetUserId(); userID != "user-123" {
		t.Errorf("GetUserId() = %q, want %q", userID, "user-123")
	}
}

func TestSetUser_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := SetUser(`{"user_id": "user-123"}`)
	if result == "" {
		t.Fatal("SetUser should return error when not initialized")
	}
}

func TestSetUser_InvalidJSON(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := SetUser("not json")
	if result == "" {
		t.Fatal("SetUser should return error for invalid JSON")
	}
}

func TestSetUser_MissingUserID(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := SetUser(`{"traits": {"name": "Alice"}}`)
	if result == "" {
		t.Fatal("SetUser should return error for missing user_id")
	}
}

func TestSetUser_Persisted(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	SetUser(`{"user_id": "user-persist"}`)

	// Identity manager should have the user persisted
	inst := getInstance()
	user := inst.identityManager.GetUser()
	if user == nil {
		t.Fatal("identityManager.GetUser() returned nil")
	}
	if user.UserID != "user-persist" {
		t.Errorf("user.UserID = %q, want %q", user.UserID, "user-persist")
	}
}

func TestReset_ClearsUserId(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	SetUser(`{"user_id": "user-123"}`)

	if GetUserId() != "user-123" {
		t.Fatal("user not set before Reset")
	}

	result := Reset()
	if result != "" {
		t.Fatalf("Reset returned error: %s", result)
	}

	if GetUserId() != "" {
		t.Errorf("GetUserId() = %q after Reset, want empty", GetUserId())
	}
}

func TestReset_PreservesDeviceId(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	deviceID := GetDeviceId()

	Reset()

	if GetDeviceId() != deviceID {
		t.Errorf("DeviceId changed after Reset: %q -> %q", deviceID, GetDeviceId())
	}
}

func TestReset_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := Reset()
	if result == "" {
		t.Fatal("Reset should return error when not initialized")
	}
}

func TestResetAll_ClearsEverything(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	SetUser(`{"user_id": "user-123"}`)
	originalDeviceID := GetDeviceId()

	result := ResetAll()
	if result != "" {
		t.Fatalf("ResetAll returned error: %s", result)
	}

	if GetUserId() != "" {
		t.Errorf("GetUserId() = %q after ResetAll, want empty", GetUserId())
	}

	// Device ID should be regenerated (different)
	newDeviceID := GetDeviceId()
	if newDeviceID == originalDeviceID {
		t.Error("DeviceId should be regenerated after ResetAll")
	}
	if newDeviceID == "" {
		t.Error("DeviceId should not be empty after ResetAll")
	}
}

func TestResetAll_ClearsQueue(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)
	Track(`{"type": "button_tap", "properties": {"button_id": "btn1"}}`)

	inst := getInstance()
	count, _ := inst.queue.Count()
	if count == 0 {
		t.Fatal("queue should have events before ResetAll")
	}

	ResetAll()

	count, _ = inst.queue.Count()
	if count != 0 {
		t.Errorf("queue count = %d after ResetAll, want 0", count)
	}
}

func TestResetAll_EndsSession(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	// Start a session by tracking
	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)
	sessionBefore := GetSessionId()
	if sessionBefore == "" {
		t.Fatal("session should be active after Track")
	}

	ResetAll()

	// Session should be ended
	sessionAfter := GetSessionId()
	if sessionAfter != "" {
		t.Errorf("session should be empty after ResetAll, got %q", sessionAfter)
	}
}

func TestResetAll_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := ResetAll()
	if result == "" {
		t.Fatal("ResetAll should return error when not initialized")
	}
}

func TestFlush_Initialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := Flush()
	// Flush may return error (transport will fail since no server) or empty (no events to flush)
	// With no events, it should succeed
	if result != "" {
		t.Fatalf("Flush returned error: %s", result)
	}
}

func TestFlush_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := Flush()
	if result == "" {
		t.Fatal("Flush should return error when not initialized")
	}
}

func TestGetDeviceId_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	if id := GetDeviceId(); id != "" {
		t.Errorf("GetDeviceId() = %q when not initialized, want empty", id)
	}
}

func TestGetSessionId_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	if id := GetSessionId(); id != "" {
		t.Errorf("GetSessionId() = %q when not initialized, want empty", id)
	}
}

func TestGetSessionId_EmptyByDefault(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	// Session tracker exists but no RecordActivity called yet, so no session
	if id := GetSessionId(); id != "" {
		t.Errorf("GetSessionId() = %q, want empty (no activity yet)", id)
	}
}

func TestGetSessionId_ActiveAfterTrack(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())
	Track(`{"type": "screen_view", "properties": {"screen_name": "Home"}}`)

	id := GetSessionId()
	if id == "" {
		t.Error("GetSessionId() should be non-empty after Track")
	}
	if len(id) != 36 {
		t.Errorf("session_id length = %d, want 36 (UUID)", len(id))
	}
}

func TestGetUserId_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	if id := GetUserId(); id != "" {
		t.Errorf("GetUserId() = %q when not initialized, want empty", id)
	}
}

func TestGetUserId_EmptyByDefault(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	if id := GetUserId(); id != "" {
		t.Errorf("GetUserId() = %q, want empty (no user set)", id)
	}
}

func TestIsInitialized_BeforeInit(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	if IsInitialized() {
		t.Error("IsInitialized() = true before Init")
	}
}

func TestSetDebugMode(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	// Should not panic
	SetDebugMode(true)
	SetDebugMode(false)
}

func TestSetDebugMode_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	// Should not panic
	SetDebugMode(true)
}

func TestTrack_WithDebugMode(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(`{"api_key": "key", "endpoint": "https://api.example.com", "app_id": "app", "debug_mode": true}`)

	result := Track(`{"type": "button_tap", "properties": {"button_id": "btn-1"}}`)
	if result != "" {
		t.Fatalf("Track returned error in debug mode: %s", result)
	}
}

func TestSetUser_WithDebugMode(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(`{"api_key": "key", "endpoint": "https://api.example.com", "app_id": "app", "debug_mode": true}`)

	result := SetUser(`{"user_id": "user-debug"}`)
	if result != "" {
		t.Fatalf("SetUser returned error in debug mode: %s", result)
	}
}

func TestReset_WithDebugMode(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(`{"api_key": "key", "endpoint": "https://api.example.com", "app_id": "app", "debug_mode": true}`)

	result := Reset()
	if result != "" {
		t.Fatalf("Reset returned error in debug mode: %s", result)
	}
}

func TestResetAll_WithDebugMode(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(`{"api_key": "key", "endpoint": "https://api.example.com", "app_id": "app", "debug_mode": true}`)

	result := ResetAll()
	if result != "" {
		t.Fatalf("ResetAll returned error in debug mode: %s", result)
	}
}

func TestFlush_WithDebugMode(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(`{"api_key": "key", "endpoint": "https://api.example.com", "app_id": "app", "debug_mode": true}`)

	result := Flush()
	if result != "" {
		t.Fatalf("Flush returned error in debug mode: %s", result)
	}
}

// --- Lifecycle Hook Tests ---

func TestAppDidEnterBackground_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := AppDidEnterBackground()
	if result == "" {
		t.Fatal("AppDidEnterBackground should return error when not initialized")
	}
}

func TestAppDidEnterBackground_Initialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := AppDidEnterBackground()
	if result != "" {
		t.Fatalf("AppDidEnterBackground returned error: %s", result)
	}
}

func TestAppWillEnterForeground_NotInitialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	result := AppWillEnterForeground()
	if result == "" {
		t.Fatal("AppWillEnterForeground should return error when not initialized")
	}
}

func TestAppWillEnterForeground_Initialized(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	result := AppWillEnterForeground()
	if result != "" {
		t.Fatalf("AppWillEnterForeground returned error: %s", result)
	}
}

func TestSetPlatformContext_DoesNotPanic(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	// SetPlatformContext can be called before or after Init
	SetPlatformContext("ios", "17.2", "iPhone 15", "Apple", "2.1.0", "142", 1179, 2556, "en_US", "America/New_York")
}

func TestSetNetworkInfo_DoesNotPanic(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	SetNetworkInfo("AT&T", "cellular")
}

func TestDeviceId_ConsistentAcrossCalls(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	id1 := GetDeviceId()
	id2 := GetDeviceId()
	if id1 != id2 {
		t.Errorf("GetDeviceId() returned different values: %q vs %q", id1, id2)
	}
}

func TestTrack_ConsistentDeviceIdAcrossEvents(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	Track(`{"type": "screen_view", "properties": {"screen_name": "A"}}`)
	Track(`{"type": "screen_view", "properties": {"screen_name": "B"}}`)

	inst := getInstance()
	events, err := inst.queue.DequeueBatch(2)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	var event1, event2 Event
	json.Unmarshal([]byte(events[0].EventJSON), &event1)
	json.Unmarshal([]byte(events[1].EventJSON), &event2)

	if event1.Metadata.DeviceID != event2.Metadata.DeviceID {
		t.Errorf("device_id differs across events: %q vs %q", event1.Metadata.DeviceID, event2.Metadata.DeviceID)
	}
}

func TestTrack_UniqueIdempotencyKeys(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	Init(validConfigJSON())

	Track(`{"type": "screen_view", "properties": {"screen_name": "A"}}`)
	Track(`{"type": "screen_view", "properties": {"screen_name": "B"}}`)

	inst := getInstance()
	events, err := inst.queue.DequeueBatch(2)
	if err != nil {
		t.Fatalf("DequeueBatch failed: %v", err)
	}

	var event1, event2 Event
	json.Unmarshal([]byte(events[0].EventJSON), &event1)
	json.Unmarshal([]byte(events[1].EventJSON), &event2)

	if event1.Metadata.IdempotencyKey == event2.Metadata.IdempotencyKey {
		t.Errorf("idempotency_key should be unique, both = %q", event1.Metadata.IdempotencyKey)
	}
}
