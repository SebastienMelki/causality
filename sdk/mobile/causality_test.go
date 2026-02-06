package mobile

import (
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

	// Session tracking module not yet wired, so session ID should be empty
	if id := GetSessionId(); id != "" {
		t.Errorf("GetSessionId() = %q, want empty (no session module)", id)
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
