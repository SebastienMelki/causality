package mobile

import (
	"encoding/json"
	"testing"
)

func TestParseEvent_ValidEvent(t *testing.T) {
	eventJSON := `{
		"type": "screen_view",
		"properties": {"screen_name": "Home", "screen_class": "HomeViewController"}
	}`

	event, err := parseEvent(eventJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Type != "screen_view" {
		t.Errorf("Type = %q, want %q", event.Type, "screen_view")
	}
	if event.Properties == nil {
		t.Fatal("Properties should not be nil")
	}

	// Verify properties can be unmarshaled
	var props map[string]interface{}
	if err := json.Unmarshal(event.Properties, &props); err != nil {
		t.Fatalf("failed to unmarshal properties: %v", err)
	}
	if props["screen_name"] != "Home" {
		t.Errorf("screen_name = %v, want %q", props["screen_name"], "Home")
	}
}

func TestParseEvent_TypedScreenView(t *testing.T) {
	sv := ScreenViewEvent{
		ScreenName:     "Dashboard",
		ScreenClass:    "DashboardActivity",
		PreviousScreen: "Login",
	}

	propsJSON, err := json.Marshal(sv)
	if err != nil {
		t.Fatalf("failed to marshal ScreenViewEvent: %v", err)
	}

	eventJSON := `{"type": "screen_view", "properties": ` + string(propsJSON) + `}`

	event, err := parseEvent(eventJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Type != "screen_view" {
		t.Errorf("Type = %q, want %q", event.Type, "screen_view")
	}

	// Verify we can deserialize back to the typed struct
	var parsed ScreenViewEvent
	if err := json.Unmarshal(event.Properties, &parsed); err != nil {
		t.Fatalf("failed to unmarshal to ScreenViewEvent: %v", err)
	}
	if parsed.ScreenName != "Dashboard" {
		t.Errorf("ScreenName = %q, want %q", parsed.ScreenName, "Dashboard")
	}
	if parsed.PreviousScreen != "Login" {
		t.Errorf("PreviousScreen = %q, want %q", parsed.PreviousScreen, "Login")
	}
}

func TestParseEvent_TypedPurchase(t *testing.T) {
	purchase := PurchaseCompleteEvent{
		OrderID:       "order-456",
		CartID:        "cart-789",
		ItemCount:     3,
		TotalCents:    15999,
		Currency:      "USD",
		PaymentMethod: "credit_card",
	}

	propsJSON, err := json.Marshal(purchase)
	if err != nil {
		t.Fatalf("failed to marshal PurchaseCompleteEvent: %v", err)
	}

	eventJSON := `{"type": "purchase_complete", "properties": ` + string(propsJSON) + `}`

	event, err := parseEvent(eventJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Type != "purchase_complete" {
		t.Errorf("Type = %q, want %q", event.Type, "purchase_complete")
	}

	var parsed PurchaseCompleteEvent
	if err := json.Unmarshal(event.Properties, &parsed); err != nil {
		t.Fatalf("failed to unmarshal to PurchaseCompleteEvent: %v", err)
	}
	if parsed.OrderID != "order-456" {
		t.Errorf("OrderID = %q, want %q", parsed.OrderID, "order-456")
	}
	if parsed.TotalCents != 15999 {
		t.Errorf("TotalCents = %d, want %d", parsed.TotalCents, 15999)
	}
	if parsed.Currency != "USD" {
		t.Errorf("Currency = %q, want %q", parsed.Currency, "USD")
	}
}

func TestParseEvent_MinimalEvent(t *testing.T) {
	eventJSON := `{"type": "custom"}`

	event, err := parseEvent(eventJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Type != "custom" {
		t.Errorf("Type = %q, want %q", event.Type, "custom")
	}
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "not json",
			input:   "this is not json",
			wantErr: "invalid event JSON",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: "event JSON is empty",
		},
		{
			name:    "missing type",
			input:   `{"properties": {"key": "value"}}`,
			wantErr: "event type is required",
		},
		{
			name:    "empty type",
			input:   `{"type": ""}`,
			wantErr: "event type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseEvent(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestParseUser_ValidUser(t *testing.T) {
	userJSON := `{
		"user_id": "user-123",
		"traits": {"name": "Alice", "plan": "premium", "email": "alice@example.com"}
	}`

	user, err := parseUser(userJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", user.UserID, "user-123")
	}
	if user.Traits["name"] != "Alice" {
		t.Errorf("Traits[name] = %q, want %q", user.Traits["name"], "Alice")
	}
	if user.Traits["plan"] != "premium" {
		t.Errorf("Traits[plan] = %q, want %q", user.Traits["plan"], "premium")
	}
}

func TestParseUser_WithAliases(t *testing.T) {
	userJSON := `{
		"user_id": "user-456",
		"aliases": ["email:bob@example.com", "phone:+1234567890"],
		"traits": {"name": "Bob"}
	}`

	user, err := parseUser(userJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.UserID != "user-456" {
		t.Errorf("UserID = %q, want %q", user.UserID, "user-456")
	}
	if len(user.Aliases) != 2 {
		t.Fatalf("Aliases length = %d, want 2", len(user.Aliases))
	}
	if user.Aliases[0] != "email:bob@example.com" {
		t.Errorf("Aliases[0] = %q, want %q", user.Aliases[0], "email:bob@example.com")
	}
	if user.Aliases[1] != "phone:+1234567890" {
		t.Errorf("Aliases[1] = %q, want %q", user.Aliases[1], "phone:+1234567890")
	}
}

func TestParseUser_InvalidJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "not json",
			input:   "not json",
			wantErr: "invalid user JSON",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: "user JSON is empty",
		},
		{
			name:    "missing user_id",
			input:   `{"traits": {"name": "Alice"}}`,
			wantErr: "user_id is required",
		},
		{
			name:    "empty user_id",
			input:   `{"user_id": ""}`,
			wantErr: "user_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUser(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestSerializeEvent_TypedEvent(t *testing.T) {
	sv := ScreenViewEvent{
		ScreenName:     "Settings",
		ScreenClass:    "SettingsFragment",
		PreviousScreen: "Home",
	}

	jsonStr, err := serializeEvent(sv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("serialized output is not valid JSON: %v", err)
	}

	if parsed["screen_name"] != "Settings" {
		t.Errorf("screen_name = %v, want %q", parsed["screen_name"], "Settings")
	}
	if parsed["screen_class"] != "SettingsFragment" {
		t.Errorf("screen_class = %v, want %q", parsed["screen_class"], "SettingsFragment")
	}
}

func TestSerializeEvent_PurchaseEvent(t *testing.T) {
	purchase := PurchaseCompleteEvent{
		OrderID:    "order-789",
		TotalCents: 9999,
		Currency:   "EUR",
	}

	jsonStr, err := serializeEvent(purchase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed PurchaseCompleteEvent
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}

	if parsed.OrderID != "order-789" {
		t.Errorf("OrderID = %q, want %q", parsed.OrderID, "order-789")
	}
	if parsed.TotalCents != 9999 {
		t.Errorf("TotalCents = %d, want %d", parsed.TotalCents, 9999)
	}
}

func TestMarshalEvent_CreatesWrappedEvent(t *testing.T) {
	sv := ScreenViewEvent{
		ScreenName: "Profile",
	}

	event, err := marshalEvent(EventTypeScreenView, sv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Type != EventTypeScreenView {
		t.Errorf("Type = %q, want %q", event.Type, EventTypeScreenView)
	}

	// Verify properties contain the typed event
	var parsed ScreenViewEvent
	if err := json.Unmarshal(event.Properties, &parsed); err != nil {
		t.Fatalf("failed to unmarshal properties: %v", err)
	}
	if parsed.ScreenName != "Profile" {
		t.Errorf("ScreenName = %q, want %q", parsed.ScreenName, "Profile")
	}
}

func TestIsValidEventType(t *testing.T) {
	validTypes := []string{
		EventTypeScreenView, EventTypeScreenExit, EventTypeButtonTap,
		EventTypeUserLogin, EventTypeUserLogout, EventTypeUserSignup,
		EventTypeProductView, EventTypeAddToCart, EventTypePurchaseComplete,
		EventTypeAppStart, EventTypeAppBackground, EventTypeAppForeground,
		EventTypeCustom,
	}

	for _, et := range validTypes {
		if !isValidEventType(et) {
			t.Errorf("isValidEventType(%q) = false, want true", et)
		}
	}

	invalidTypes := []string{"unknown", "invalid_type", "", "SCREEN_VIEW", "screenView"}
	for _, et := range invalidTypes {
		if isValidEventType(et) {
			t.Errorf("isValidEventType(%q) = true, want false", et)
		}
	}
}
