package mobile

import "encoding/json"

// SDKVersion is the current version of the mobile SDK.
const SDKVersion = "0.1.0"

// EventMetadata contains fields automatically injected by the SDK.
// Developers do not set these directly; the SDK populates them on Track().
type EventMetadata struct {
	SessionID      string `json:"session_id,omitempty"`
	DeviceID       string `json:"device_id"`
	UserID         string `json:"user_id,omitempty"`
	Timestamp      string `json:"timestamp"`
	IdempotencyKey string `json:"idempotency_key"`
	AppID          string `json:"app_id"`
}

// Event wraps any event type with metadata for the JSON bridge.
// The Type field determines which typed struct is in Properties.
type Event struct {
	// Type is the event type identifier (e.g., "screen_view", "button_tap", "custom").
	Type string `json:"type"`

	// Properties is the serialized typed event data (e.g., ScreenViewEvent as JSON).
	Properties json.RawMessage `json:"properties,omitempty"`

	// Metadata is injected by the SDK (session_id, device_id, user_id, timestamp, etc.).
	Metadata EventMetadata `json:"metadata,omitempty"`
}

// ScreenViewEvent represents a screen/page view.
// Proto equivalent: causality.v1.ScreenView
type ScreenViewEvent struct {
	ScreenName     string `json:"screen_name"`
	ScreenClass    string `json:"screen_class,omitempty"`
	PreviousScreen string `json:"previous_screen,omitempty"`
}

// ScreenExitEvent represents leaving a screen with duration.
// Proto equivalent: causality.v1.ScreenExit
type ScreenExitEvent struct {
	ScreenName string `json:"screen_name"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	NextScreen string `json:"next_screen,omitempty"`
}

// ButtonTapEvent represents a button/UI interaction.
// Proto equivalent: causality.v1.ButtonTap
type ButtonTapEvent struct {
	ButtonID   string `json:"button_id"`
	ButtonText string `json:"button_text,omitempty"`
	ScreenName string `json:"screen_name,omitempty"`
}

// UserLoginEvent represents a user login.
// Proto equivalent: causality.v1.UserLogin
type UserLoginEvent struct {
	UserID    string `json:"user_id"`
	Method    string `json:"method,omitempty"`
	IsNewUser bool   `json:"is_new_user,omitempty"`
}

// UserLogoutEvent represents a user logout.
// Proto equivalent: causality.v1.UserLogout
type UserLogoutEvent struct {
	UserID string `json:"user_id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// UserSignupEvent represents a new user registration.
// Proto equivalent: causality.v1.UserSignup
type UserSignupEvent struct {
	UserID         string `json:"user_id"`
	Method         string `json:"method,omitempty"`
	ReferralSource string `json:"referral_source,omitempty"`
}

// ProductViewEvent represents viewing a product.
// Proto equivalent: causality.v1.ProductView
type ProductViewEvent struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name,omitempty"`
	Category    string `json:"category,omitempty"`
	PriceCents  int64  `json:"price_cents,omitempty"`
	Currency    string `json:"currency,omitempty"`
	Source      string `json:"source,omitempty"`
}

// AddToCartEvent represents adding an item to cart.
// Proto equivalent: causality.v1.AddToCart
type AddToCartEvent struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name,omitempty"`
	Quantity    int    `json:"quantity,omitempty"`
	PriceCents  int64  `json:"price_cents,omitempty"`
	Currency    string `json:"currency,omitempty"`
	CartID      string `json:"cart_id,omitempty"`
}

// PurchaseCompleteEvent represents a completed purchase.
// Proto equivalent: causality.v1.PurchaseComplete
type PurchaseCompleteEvent struct {
	OrderID       string `json:"order_id"`
	CartID        string `json:"cart_id,omitempty"`
	ItemCount     int    `json:"item_count,omitempty"`
	TotalCents    int64  `json:"total_cents,omitempty"`
	Currency      string `json:"currency,omitempty"`
	PaymentMethod string `json:"payment_method,omitempty"`
}

// AppStartEvent represents the app launch.
// Proto equivalent: causality.v1.AppStart
type AppStartEvent struct {
	IsColdStart      bool   `json:"is_cold_start,omitempty"`
	LaunchDurationMs int64  `json:"launch_duration_ms,omitempty"`
	LaunchSource     string `json:"launch_source,omitempty"`
	DeeplinkURL      string `json:"deeplink_url,omitempty"`
}

// AppBackgroundEvent represents the app going to background.
// Proto equivalent: causality.v1.AppBackground
type AppBackgroundEvent struct {
	ForegroundDurationMs int64  `json:"foreground_duration_ms,omitempty"`
	CurrentScreen        string `json:"current_screen,omitempty"`
}

// AppForegroundEvent represents the app coming to foreground.
// Proto equivalent: causality.v1.AppForeground
type AppForegroundEvent struct {
	BackgroundDurationMs int64  `json:"background_duration_ms,omitempty"`
	ResumeScreen         string `json:"resume_screen,omitempty"`
}

// CustomEvent represents a user-defined event with arbitrary properties.
// Proto equivalent: causality.v1.CustomEvent
type CustomEvent struct {
	EventName string `json:"event_name"`
	// Properties are passed as a flat JSON string via the bridge.
	// Native wrappers provide typed builders.
}

// Known event type constants for validation.
const (
	EventTypeScreenView       = "screen_view"
	EventTypeScreenExit       = "screen_exit"
	EventTypeButtonTap        = "button_tap"
	EventTypeUserLogin        = "user_login"
	EventTypeUserLogout       = "user_logout"
	EventTypeUserSignup       = "user_signup"
	EventTypeProductView      = "product_view"
	EventTypeAddToCart         = "add_to_cart"
	EventTypePurchaseComplete = "purchase_complete"
	EventTypeAppStart         = "app_start"
	EventTypeAppBackground    = "app_background"
	EventTypeAppForeground    = "app_foreground"
	EventTypeCustom           = "custom"
)

// validEventTypes maps known event types for validation.
var validEventTypes = map[string]bool{
	EventTypeScreenView:       true,
	EventTypeScreenExit:       true,
	EventTypeButtonTap:        true,
	EventTypeUserLogin:        true,
	EventTypeUserLogout:       true,
	EventTypeUserSignup:       true,
	EventTypeProductView:      true,
	EventTypeAddToCart:         true,
	EventTypePurchaseComplete: true,
	EventTypeAppStart:         true,
	EventTypeAppBackground:    true,
	EventTypeAppForeground:    true,
	EventTypeCustom:           true,
}

// isValidEventType checks if the event type is known.
func isValidEventType(eventType string) bool {
	return validEventTypes[eventType]
}
