package warehouse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress"

	"github.com/SebastienMelki/causality/internal/events"
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
		ID:            event.GetId(),
		AppID:         event.GetAppId(),
		DeviceID:      event.GetDeviceId(),
		TimestampMS:   event.GetTimestampMs(),
		CorrelationID: event.GetCorrelationId(),
		Year:          year,
		Month:         month,
		Day:           day,
		Hour:          hour,
	}

	// Extract event category and type
	row.EventCategory, row.EventType = events.GetCategoryAndType(event)

	// Extract device context
	if ctx := event.GetDeviceContext(); ctx != nil {
		row.Platform = ctx.GetPlatform().String()
		row.OSVersion = ctx.GetOsVersion()
		row.AppVersion = ctx.GetAppVersion()
		row.BuildNumber = ctx.GetBuildNumber()
		row.DeviceModel = ctx.GetDeviceModel()
		row.Manufacturer = ctx.GetManufacturer()
		row.ScreenWidth = ctx.GetScreenWidth()
		row.ScreenHeight = ctx.GetScreenHeight()
		row.Locale = ctx.GetLocale()
		row.Timezone = ctx.GetTimezone()
		row.NetworkType = ctx.GetNetworkType().String()
		row.Carrier = ctx.GetCarrier()
		row.IsJailbroken = ctx.GetIsJailbroken()
		row.IsEmulator = ctx.GetIsEmulator()
		row.SDKVersion = ctx.GetSdkVersion()
	}

	// Serialize payload to JSON
	row.PayloadJSON = serializePayload(event)

	return row
}

// serializePayload serializes the event payload to JSON.
func serializePayload(event *pb.EventEnvelope) string {
	if event.GetPayload() == nil {
		return "{}"
	}

	// Get the payload value using reflection
	payloadValue := reflect.ValueOf(event.GetPayload())
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
		return nil, ErrNoRowsToWrite
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
