package transport

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	causalityv1 "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
	"github.com/SebastienMelki/causality/sdk/mobile/internal/device"
)

// sdkEvent mirrors the SDK's Event type for JSON parsing within the transport layer.
type sdkEvent struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties,omitempty"`
	Metadata   sdkMetadata     `json:"metadata,omitempty"`
}

type sdkMetadata struct {
	SessionID      string `json:"session_id,omitempty"`
	DeviceID       string `json:"device_id"`
	UserID         string `json:"user_id,omitempty"`
	Timestamp      string `json:"timestamp"`
	IdempotencyKey string `json:"idempotency_key"`
	AppID          string `json:"app_id"`
}

// convertEvents parses SDK JSON event strings into protobuf EventEnvelopes.
// Device context is collected once per batch and attached to all envelopes.
func convertEvents(jsonEvents []string) ([]*causalityv1.EventEnvelope, error) {
	deviceCtx := buildDeviceContext()
	envelopes := make([]*causalityv1.EventEnvelope, 0, len(jsonEvents))

	for i, jsonStr := range jsonEvents {
		var evt sdkEvent
		if err := json.Unmarshal([]byte(jsonStr), &evt); err != nil {
			return nil, fmt.Errorf("event %d: unmarshal: %w", i, err)
		}

		env := &causalityv1.EventEnvelope{
			AppId:          evt.Metadata.AppID,
			DeviceId:       evt.Metadata.DeviceID,
			IdempotencyKey: evt.Metadata.IdempotencyKey,
			DeviceContext:  deviceCtx,
		}

		// Parse timestamp from RFC3339Nano to milliseconds since epoch
		if evt.Metadata.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, evt.Metadata.Timestamp); err == nil {
				env.TimestampMs = t.UnixMilli()
			}
		}

		if err := setPayload(env, evt.Type, evt.Properties); err != nil {
			return nil, fmt.Errorf("event %d (%s): %w", i, evt.Type, err)
		}

		envelopes = append(envelopes, env)
	}

	return envelopes, nil
}

// buildDeviceContext collects current device info and maps it to the protobuf type.
func buildDeviceContext() *causalityv1.DeviceContext {
	ctx := device.CollectContext()
	return &causalityv1.DeviceContext{
		Platform:     mapPlatform(ctx.Platform),
		OsVersion:    ctx.OSVersion,
		AppVersion:   ctx.AppVersion,
		BuildNumber:  ctx.AppBuildNumber,
		DeviceModel:  ctx.DeviceModel,
		Manufacturer: ctx.DeviceManufacturer,
		ScreenWidth:  int32(ctx.ScreenWidth),
		ScreenHeight: int32(ctx.ScreenHeight),
		Locale:       ctx.Locale,
		Timezone:     ctx.Timezone,
		NetworkType:  mapNetworkType(ctx.NetworkType),
		Carrier:      ctx.Carrier,
	}
}

func mapPlatform(p string) causalityv1.Platform {
	switch strings.ToLower(p) {
	case "ios":
		return causalityv1.Platform_PLATFORM_IOS
	case "android":
		return causalityv1.Platform_PLATFORM_ANDROID
	case "web":
		return causalityv1.Platform_PLATFORM_WEB
	default:
		return causalityv1.Platform_PLATFORM_UNSPECIFIED
	}
}

func mapNetworkType(nt string) causalityv1.NetworkType {
	switch strings.ToLower(nt) {
	case "wifi":
		return causalityv1.NetworkType_NETWORK_TYPE_WIFI
	case "cellular", "cellular_4g", "4g":
		return causalityv1.NetworkType_NETWORK_TYPE_CELLULAR_4G
	case "cellular_5g", "5g":
		return causalityv1.NetworkType_NETWORK_TYPE_CELLULAR_5G
	case "cellular_3g", "3g":
		return causalityv1.NetworkType_NETWORK_TYPE_CELLULAR_3G
	case "cellular_2g", "2g":
		return causalityv1.NetworkType_NETWORK_TYPE_CELLULAR_2G
	case "ethernet":
		return causalityv1.NetworkType_NETWORK_TYPE_ETHERNET
	case "offline":
		return causalityv1.NetworkType_NETWORK_TYPE_OFFLINE
	default:
		return causalityv1.NetworkType_NETWORK_TYPE_UNSPECIFIED
	}
}

// setPayload unmarshals event properties and sets the correct oneof payload on the envelope.
func setPayload(env *causalityv1.EventEnvelope, eventType string, props json.RawMessage) error {
	switch eventType {
	case "screen_view":
		var p causalityv1.ScreenView
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_ScreenView{ScreenView: &p}

	case "screen_exit":
		var p causalityv1.ScreenExit
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_ScreenExit{ScreenExit: &p}

	case "button_tap":
		var p causalityv1.ButtonTap
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_ButtonTap{ButtonTap: &p}

	case "user_login":
		var p causalityv1.UserLogin
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_UserLogin{UserLogin: &p}

	case "user_logout":
		var p causalityv1.UserLogout
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_UserLogout{UserLogout: &p}

	case "user_signup":
		var p causalityv1.UserSignup
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_UserSignup{UserSignup: &p}

	case "product_view":
		var p causalityv1.ProductView
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_ProductView{ProductView: &p}

	case "add_to_cart":
		var p causalityv1.AddToCart
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_AddToCart{AddToCart: &p}

	case "purchase_complete":
		var p causalityv1.PurchaseComplete
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_PurchaseComplete{PurchaseComplete: &p}

	case "app_start":
		var p causalityv1.AppStart
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_AppStart{AppStart: &p}

	case "app_background":
		var p causalityv1.AppBackground
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_AppBackground{AppBackground: &p}

	case "app_foreground":
		var p causalityv1.AppForeground
		if err := unmarshalProps(props, &p); err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_AppForeground{AppForeground: &p}

	case "custom":
		ce, err := convertCustomEvent(props)
		if err != nil {
			return err
		}
		env.Payload = &causalityv1.EventEnvelope_CustomEvent{CustomEvent: ce}

	default:
		// Unknown event types fall through as custom events with the type as event_name
		ce := &causalityv1.CustomEvent{EventName: eventType}
		if len(props) > 0 {
			ce.StringParams = make(map[string]string)
			ce.StringParams["_raw_properties"] = string(props)
		}
		env.Payload = &causalityv1.EventEnvelope_CustomEvent{CustomEvent: ce}
	}

	return nil
}

// unmarshalProps unmarshals JSON properties into a proto struct using standard JSON tags.
// Proto-generated types have matching snake_case JSON tags, so encoding/json works correctly.
func unmarshalProps(props json.RawMessage, dst interface{}) error {
	if len(props) == 0 {
		return nil
	}
	return json.Unmarshal(props, dst)
}

// convertCustomEvent handles custom events by extracting event_name and categorizing
// remaining properties into typed parameter maps.
func convertCustomEvent(props json.RawMessage) (*causalityv1.CustomEvent, error) {
	ce := &causalityv1.CustomEvent{}

	if len(props) == 0 {
		return ce, nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(props, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal custom event: %w", err)
	}

	if name, ok := raw["event_name"].(string); ok {
		ce.EventName = name
		delete(raw, "event_name")
	}

	if len(raw) == 0 {
		return ce, nil
	}

	ce.StringParams = make(map[string]string)
	ce.IntParams = make(map[string]int64)
	ce.FloatParams = make(map[string]float64)
	ce.BoolParams = make(map[string]bool)

	for k, v := range raw {
		switch val := v.(type) {
		case string:
			ce.StringParams[k] = val
		case float64:
			// JSON numbers are always float64; detect integers
			if val == float64(int64(val)) {
				ce.IntParams[k] = int64(val)
			} else {
				ce.FloatParams[k] = val
			}
		case bool:
			ce.BoolParams[k] = val
		default:
			// Nested objects/arrays → serialize back to JSON string
			b, _ := json.Marshal(v)
			ce.StringParams[k] = string(b)
		}
	}

	return ce, nil
}
