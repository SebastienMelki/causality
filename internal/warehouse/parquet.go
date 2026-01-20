package warehouse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress"

	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// EventRow is the flattened structure for Parquet storage.
// This schema is optimized for analytics queries via Hive/Athena.
type EventRow struct {
	// Event envelope fields
	ID            string `parquet:"id,snappy"`
	AppID         string `parquet:"app_id,snappy,dict"`
	DeviceID      string `parquet:"device_id,snappy"`
	TimestampMS   int64  `parquet:"timestamp_ms"`
	CorrelationID string `parquet:"correlation_id,snappy,optional"`

	// Event type information
	EventCategory string `parquet:"event_category,snappy,dict"`
	EventType     string `parquet:"event_type,snappy,dict"`

	// Device context fields
	Platform     string `parquet:"platform,snappy,dict,optional"`
	OSVersion    string `parquet:"os_version,snappy,optional"`
	AppVersion   string `parquet:"app_version,snappy,optional"`
	BuildNumber  string `parquet:"build_number,snappy,optional"`
	DeviceModel  string `parquet:"device_model,snappy,optional"`
	Manufacturer string `parquet:"manufacturer,snappy,optional"`
	ScreenWidth  int32  `parquet:"screen_width,optional"`
	ScreenHeight int32  `parquet:"screen_height,optional"`
	Locale       string `parquet:"locale,snappy,optional"`
	Timezone     string `parquet:"timezone,snappy,optional"`
	NetworkType  string `parquet:"network_type,snappy,dict,optional"`
	Carrier      string `parquet:"carrier,snappy,optional"`
	IsJailbroken bool   `parquet:"is_jailbroken,optional"`
	IsEmulator   bool   `parquet:"is_emulator,optional"`
	SDKVersion   string `parquet:"sdk_version,snappy,optional"`

	// Payload as JSON (with type discriminator for querying)
	PayloadJSON string `parquet:"payload_json,snappy"`

	// Partition columns (for Hive partitioning)
	Year  int `parquet:"year,dict"`
	Month int `parquet:"month,dict"`
	Day   int `parquet:"day,dict"`
	Hour  int `parquet:"hour,dict"`
}

// EventRowFromProto converts a protobuf EventEnvelope to an EventRow.
func EventRowFromProto(event *pb.EventEnvelope, year, month, day, hour int) EventRow {
	row := EventRow{
		ID:            event.Id,
		AppID:         event.AppId,
		DeviceID:      event.DeviceId,
		TimestampMS:   event.TimestampMs,
		CorrelationID: event.CorrelationId,
		Year:          year,
		Month:         month,
		Day:           day,
		Hour:          hour,
	}

	// Extract event category and type
	row.EventCategory, row.EventType = getEventCategoryAndType(event)

	// Extract device context
	if ctx := event.DeviceContext; ctx != nil {
		row.Platform = ctx.Platform.String()
		row.OSVersion = ctx.OsVersion
		row.AppVersion = ctx.AppVersion
		row.BuildNumber = ctx.BuildNumber
		row.DeviceModel = ctx.DeviceModel
		row.Manufacturer = ctx.Manufacturer
		row.ScreenWidth = ctx.ScreenWidth
		row.ScreenHeight = ctx.ScreenHeight
		row.Locale = ctx.Locale
		row.Timezone = ctx.Timezone
		row.NetworkType = ctx.NetworkType.String()
		row.Carrier = ctx.Carrier
		row.IsJailbroken = ctx.IsJailbroken
		row.IsEmulator = ctx.IsEmulator
		row.SDKVersion = ctx.SdkVersion
	}

	// Serialize payload to JSON
	row.PayloadJSON = serializePayload(event)

	return row
}

// getEventCategoryAndType extracts category and type from event payload.
func getEventCategoryAndType(event *pb.EventEnvelope) (category, eventType string) {
	switch payload := event.Payload.(type) {
	// User events
	case *pb.EventEnvelope_UserLogin:
		return "user", "login"
	case *pb.EventEnvelope_UserLogout:
		return "user", "logout"
	case *pb.EventEnvelope_UserSignup:
		return "user", "signup"
	case *pb.EventEnvelope_UserProfileUpdate:
		return "user", "profile_update"

	// Screen events
	case *pb.EventEnvelope_ScreenView:
		return "screen", "view"
	case *pb.EventEnvelope_ScreenExit:
		return "screen", "exit"

	// Interaction events
	case *pb.EventEnvelope_ButtonTap:
		return "interaction", "button_tap"
	case *pb.EventEnvelope_SwipeGesture:
		return "interaction", "swipe"
	case *pb.EventEnvelope_ScrollEvent:
		return "interaction", "scroll"
	case *pb.EventEnvelope_TextInput:
		return "interaction", "text_input"
	case *pb.EventEnvelope_LongPress:
		return "interaction", "long_press"
	case *pb.EventEnvelope_DoubleTap:
		return "interaction", "double_tap"

	// Commerce events
	case *pb.EventEnvelope_ProductView:
		return "commerce", "product_view"
	case *pb.EventEnvelope_AddToCart:
		return "commerce", "add_to_cart"
	case *pb.EventEnvelope_RemoveFromCart:
		return "commerce", "remove_from_cart"
	case *pb.EventEnvelope_CheckoutStart:
		return "commerce", "checkout_start"
	case *pb.EventEnvelope_CheckoutStep:
		return "commerce", "checkout_step"
	case *pb.EventEnvelope_PurchaseComplete:
		return "commerce", "purchase_complete"
	case *pb.EventEnvelope_PurchaseFailed:
		return "commerce", "purchase_failed"

	// System events
	case *pb.EventEnvelope_AppStart:
		return "system", "app_start"
	case *pb.EventEnvelope_AppBackground:
		return "system", "app_background"
	case *pb.EventEnvelope_AppForeground:
		return "system", "app_foreground"
	case *pb.EventEnvelope_AppCrash:
		return "system", "app_crash"
	case *pb.EventEnvelope_NetworkChange:
		return "system", "network_change"
	case *pb.EventEnvelope_PermissionRequest:
		return "system", "permission_request"
	case *pb.EventEnvelope_PermissionResult:
		return "system", "permission_result"
	case *pb.EventEnvelope_MemoryWarning:
		return "system", "memory_warning"
	case *pb.EventEnvelope_BatteryChange:
		return "system", "battery_change"

	// Custom events
	case *pb.EventEnvelope_CustomEvent:
		if payload.CustomEvent != nil {
			return "custom", payload.CustomEvent.EventName
		}
		return "custom", "unknown"

	default:
		if event.Payload != nil {
			t := reflect.TypeOf(event.Payload)
			return "unknown", t.Elem().Name()
		}
		return "unknown", "unknown"
	}
}

// serializePayload serializes the event payload to JSON.
func serializePayload(event *pb.EventEnvelope) string {
	if event.Payload == nil {
		return "{}"
	}

	// Get the payload value using reflection
	payloadValue := reflect.ValueOf(event.Payload)
	if payloadValue.Kind() == reflect.Ptr && !payloadValue.IsNil() {
		payloadValue = payloadValue.Elem()
		if payloadValue.NumField() > 0 {
			actualPayload := payloadValue.Field(0).Interface()
			data, err := json.Marshal(actualPayload)
			if err == nil {
				return string(data)
			}
		}
	}

	return "{}"
}

// ParquetWriter handles writing events to Parquet format.
type ParquetWriter struct {
	config ParquetConfig
}

// NewParquetWriter creates a new Parquet writer.
func NewParquetWriter(cfg ParquetConfig) *ParquetWriter {
	return &ParquetWriter{
		config: cfg,
	}
}

// Write writes a batch of event rows to Parquet format and returns the bytes.
func (w *ParquetWriter) Write(rows []EventRow) ([]byte, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("no rows to write")
	}

	var buf bytes.Buffer

	// Get compression codec
	codec := w.getCompressionCodec()

	// Create Parquet writer
	writer := parquet.NewGenericWriter[EventRow](&buf,
		parquet.Compression(codec),
		parquet.CreatedBy("causality-warehouse-sink", "1.0.0", ""),
	)

	// Write rows
	if _, err := writer.Write(rows); err != nil {
		return nil, fmt.Errorf("failed to write rows: %w", err)
	}

	// Close writer to flush
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	return buf.Bytes(), nil
}

// getCompressionCodec returns the compression codec based on config.
func (w *ParquetWriter) getCompressionCodec() compress.Codec {
	switch w.config.Compression {
	case "snappy":
		return &parquet.Snappy
	case "gzip":
		return &parquet.Gzip
	case "zstd":
		return &parquet.Zstd
	case "none":
		return &parquet.Uncompressed
	default:
		return &parquet.Snappy
	}
}
