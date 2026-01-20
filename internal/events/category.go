// Package events provides shared event categorization logic.
package events

import (
	"reflect"
	"strings"

	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// Event category constants.
const (
	CategoryUser        = "user"
	CategoryScreen      = "screen"
	CategoryInteraction = "interaction"
	CategoryCommerce    = "commerce"
	CategorySystem      = "system"
	CategoryCustom      = "custom"
	CategoryUnknown     = "unknown"

	TypeUnknown = "unknown"
)

// GetCategoryAndType extracts the category and type from an event payload.
func GetCategoryAndType(event *pb.EventEnvelope) (category, eventType string) {
	switch payload := event.GetPayload().(type) {
	// User events.
	case *pb.EventEnvelope_UserLogin:
		return CategoryUser, "login"
	case *pb.EventEnvelope_UserLogout:
		return CategoryUser, "logout"
	case *pb.EventEnvelope_UserSignup:
		return CategoryUser, "signup"
	case *pb.EventEnvelope_UserProfileUpdate:
		return CategoryUser, "profile_update"

	// Screen events.
	case *pb.EventEnvelope_ScreenView:
		return CategoryScreen, "view"
	case *pb.EventEnvelope_ScreenExit:
		return CategoryScreen, "exit"

	// Interaction events.
	case *pb.EventEnvelope_ButtonTap:
		return CategoryInteraction, "button_tap"
	case *pb.EventEnvelope_SwipeGesture:
		return CategoryInteraction, "swipe"
	case *pb.EventEnvelope_ScrollEvent:
		return CategoryInteraction, "scroll"
	case *pb.EventEnvelope_TextInput:
		return CategoryInteraction, "text_input"
	case *pb.EventEnvelope_LongPress:
		return CategoryInteraction, "long_press"
	case *pb.EventEnvelope_DoubleTap:
		return CategoryInteraction, "double_tap"

	// Commerce events.
	case *pb.EventEnvelope_ProductView:
		return CategoryCommerce, "product_view"
	case *pb.EventEnvelope_AddToCart:
		return CategoryCommerce, "add_to_cart"
	case *pb.EventEnvelope_RemoveFromCart:
		return CategoryCommerce, "remove_from_cart"
	case *pb.EventEnvelope_CheckoutStart:
		return CategoryCommerce, "checkout_start"
	case *pb.EventEnvelope_CheckoutStep:
		return CategoryCommerce, "checkout_step"
	case *pb.EventEnvelope_PurchaseComplete:
		return CategoryCommerce, "purchase_complete"
	case *pb.EventEnvelope_PurchaseFailed:
		return CategoryCommerce, "purchase_failed"

	// System events.
	case *pb.EventEnvelope_AppStart:
		return CategorySystem, "app_start"
	case *pb.EventEnvelope_AppBackground:
		return CategorySystem, "app_background"
	case *pb.EventEnvelope_AppForeground:
		return CategorySystem, "app_foreground"
	case *pb.EventEnvelope_AppCrash:
		return CategorySystem, "app_crash"
	case *pb.EventEnvelope_NetworkChange:
		return CategorySystem, "network_change"
	case *pb.EventEnvelope_PermissionRequest:
		return CategorySystem, "permission_request"
	case *pb.EventEnvelope_PermissionResult:
		return CategorySystem, "permission_result"
	case *pb.EventEnvelope_MemoryWarning:
		return CategorySystem, "memory_warning"
	case *pb.EventEnvelope_BatteryChange:
		return CategorySystem, "battery_change"

	// Custom events.
	case *pb.EventEnvelope_CustomEvent:
		if payload.CustomEvent != nil {
			return CategoryCustom, payload.CustomEvent.GetEventName()
		}
		return CategoryCustom, TypeUnknown

	default:
		if event.GetPayload() != nil {
			t := reflect.TypeOf(event.GetPayload())
			return CategoryUnknown, t.Elem().Name()
		}
		return CategoryUnknown, TypeUnknown
	}
}

// SanitizeSubjectName sanitizes a name for use in NATS subjects.
func SanitizeSubjectName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return name
}
