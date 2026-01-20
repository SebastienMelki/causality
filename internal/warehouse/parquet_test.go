package warehouse

import (
	"testing"
	"time"

	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

func TestEventRowFromProto(t *testing.T) {
	timestamp := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	timestampMs := timestamp.UnixMilli()

	tests := []struct {
		name     string
		event    *pb.EventEnvelope
		year     int
		month    int
		day      int
		hour     int
		wantRow  EventRow
	}{
		{
			name: "screen view event",
			event: &pb.EventEnvelope{
				Id:            "evt-123",
				AppId:         "testapp",
				DeviceId:      "dev-456",
				TimestampMs:   timestampMs,
				CorrelationId: "corr-789",
				DeviceContext: &pb.DeviceContext{
					Platform:     pb.Platform_PLATFORM_IOS,
					OsVersion:    "17.0",
					AppVersion:   "1.2.3",
					DeviceModel:  "iPhone 15 Pro",
					Manufacturer: "Apple",
					ScreenWidth:  390,
					ScreenHeight: 844,
					Locale:       "en_US",
					Timezone:     "America/New_York",
					NetworkType:  pb.NetworkType_NETWORK_TYPE_WIFI,
				},
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{
						ScreenName:     "home",
						PreviousScreen: "splash",
					},
				},
			},
			year:  2024,
			month: 6,
			day:   15,
			hour:  14,
			wantRow: EventRow{
				ID:            "evt-123",
				AppID:         "testapp",
				DeviceID:      "dev-456",
				TimestampMS:   timestampMs,
				CorrelationID: "corr-789",
				EventCategory: "screen",
				EventType:     "view",
				Platform:      "PLATFORM_IOS",
				OSVersion:     "17.0",
				AppVersion:    "1.2.3",
				DeviceModel:   "iPhone 15 Pro",
				Manufacturer:  "Apple",
				ScreenWidth:   390,
				ScreenHeight:  844,
				Locale:        "en_US",
				Timezone:      "America/New_York",
				NetworkType:   "NETWORK_TYPE_WIFI",
				Year:          2024,
				Month:         6,
				Day:           15,
				Hour:          14,
			},
		},
		{
			name: "minimal event",
			event: &pb.EventEnvelope{
				Id:          "evt-minimal",
				AppId:       "app",
				DeviceId:    "dev",
				TimestampMs: timestampMs,
				Payload: &pb.EventEnvelope_UserLogin{
					UserLogin: &pb.UserLogin{
						UserId: "user1",
						Method: "email",
					},
				},
			},
			year:  2024,
			month: 6,
			day:   15,
			hour:  14,
			wantRow: EventRow{
				ID:            "evt-minimal",
				AppID:         "app",
				DeviceID:      "dev",
				TimestampMS:   timestampMs,
				EventCategory: "user",
				EventType:     "login",
				Year:          2024,
				Month:         6,
				Day:           15,
				Hour:          14,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := EventRowFromProto(tt.event, tt.year, tt.month, tt.day, tt.hour)

			// Check key fields
			if row.ID != tt.wantRow.ID {
				t.Errorf("ID = %q, want %q", row.ID, tt.wantRow.ID)
			}
			if row.AppID != tt.wantRow.AppID {
				t.Errorf("AppID = %q, want %q", row.AppID, tt.wantRow.AppID)
			}
			if row.DeviceID != tt.wantRow.DeviceID {
				t.Errorf("DeviceID = %q, want %q", row.DeviceID, tt.wantRow.DeviceID)
			}
			if row.TimestampMS != tt.wantRow.TimestampMS {
				t.Errorf("TimestampMS = %d, want %d", row.TimestampMS, tt.wantRow.TimestampMS)
			}
			if row.EventCategory != tt.wantRow.EventCategory {
				t.Errorf("EventCategory = %q, want %q", row.EventCategory, tt.wantRow.EventCategory)
			}
			if row.EventType != tt.wantRow.EventType {
				t.Errorf("EventType = %q, want %q", row.EventType, tt.wantRow.EventType)
			}
			if row.Year != tt.wantRow.Year {
				t.Errorf("Year = %d, want %d", row.Year, tt.wantRow.Year)
			}
			if row.Month != tt.wantRow.Month {
				t.Errorf("Month = %d, want %d", row.Month, tt.wantRow.Month)
			}
			if row.Day != tt.wantRow.Day {
				t.Errorf("Day = %d, want %d", row.Day, tt.wantRow.Day)
			}
			if row.Hour != tt.wantRow.Hour {
				t.Errorf("Hour = %d, want %d", row.Hour, tt.wantRow.Hour)
			}

			// Check device context if present
			if tt.event.DeviceContext != nil {
				if row.Platform != tt.wantRow.Platform {
					t.Errorf("Platform = %q, want %q", row.Platform, tt.wantRow.Platform)
				}
				if row.OSVersion != tt.wantRow.OSVersion {
					t.Errorf("OSVersion = %q, want %q", row.OSVersion, tt.wantRow.OSVersion)
				}
			}
		})
	}
}

func TestGetEventCategoryAndType(t *testing.T) {
	tests := []struct {
		name             string
		event            *pb.EventEnvelope
		expectedCategory string
		expectedType     string
	}{
		// User events
		{
			name:             "user login",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_UserLogin{UserLogin: &pb.UserLogin{}}},
			expectedCategory: "user",
			expectedType:     "login",
		},
		{
			name:             "user logout",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_UserLogout{UserLogout: &pb.UserLogout{}}},
			expectedCategory: "user",
			expectedType:     "logout",
		},
		{
			name:             "user signup",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_UserSignup{UserSignup: &pb.UserSignup{}}},
			expectedCategory: "user",
			expectedType:     "signup",
		},
		{
			name:             "user profile update",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_UserProfileUpdate{UserProfileUpdate: &pb.UserProfileUpdate{}}},
			expectedCategory: "user",
			expectedType:     "profile_update",
		},

		// Screen events
		{
			name:             "screen view",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_ScreenView{ScreenView: &pb.ScreenView{}}},
			expectedCategory: "screen",
			expectedType:     "view",
		},
		{
			name:             "screen exit",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_ScreenExit{ScreenExit: &pb.ScreenExit{}}},
			expectedCategory: "screen",
			expectedType:     "exit",
		},

		// Interaction events
		{
			name:             "button tap",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_ButtonTap{ButtonTap: &pb.ButtonTap{}}},
			expectedCategory: "interaction",
			expectedType:     "button_tap",
		},
		{
			name:             "swipe gesture",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_SwipeGesture{SwipeGesture: &pb.SwipeGesture{}}},
			expectedCategory: "interaction",
			expectedType:     "swipe",
		},
		{
			name:             "scroll event",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_ScrollEvent{ScrollEvent: &pb.ScrollEvent{}}},
			expectedCategory: "interaction",
			expectedType:     "scroll",
		},

		// Commerce events
		{
			name:             "product view",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_ProductView{ProductView: &pb.ProductView{}}},
			expectedCategory: "commerce",
			expectedType:     "product_view",
		},
		{
			name:             "add to cart",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_AddToCart{AddToCart: &pb.AddToCart{}}},
			expectedCategory: "commerce",
			expectedType:     "add_to_cart",
		},
		{
			name:             "purchase complete",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_PurchaseComplete{PurchaseComplete: &pb.PurchaseComplete{}}},
			expectedCategory: "commerce",
			expectedType:     "purchase_complete",
		},

		// System events
		{
			name:             "app start",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_AppStart{AppStart: &pb.AppStart{}}},
			expectedCategory: "system",
			expectedType:     "app_start",
		},
		{
			name:             "app crash",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_AppCrash{AppCrash: &pb.AppCrash{}}},
			expectedCategory: "system",
			expectedType:     "app_crash",
		},

		// Custom events
		{
			name:             "custom event",
			event:            &pb.EventEnvelope{Payload: &pb.EventEnvelope_CustomEvent{CustomEvent: &pb.CustomEvent{EventName: "my_event"}}},
			expectedCategory: "custom",
			expectedType:     "my_event",
		},

		// Edge cases
		{
			name:             "nil payload",
			event:            &pb.EventEnvelope{},
			expectedCategory: "unknown",
			expectedType:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category, eventType := getEventCategoryAndType(tt.event)
			if category != tt.expectedCategory {
				t.Errorf("category = %q, want %q", category, tt.expectedCategory)
			}
			if eventType != tt.expectedType {
				t.Errorf("eventType = %q, want %q", eventType, tt.expectedType)
			}
		})
	}
}

func TestParquetWriter_Write(t *testing.T) {
	writer := NewParquetWriter(ParquetConfig{
		Compression:  "snappy",
		RowGroupSize: 1000,
	})

	rows := []EventRow{
		{
			ID:            "evt-1",
			AppID:         "testapp",
			DeviceID:      "dev-1",
			TimestampMS:   time.Now().UnixMilli(),
			EventCategory: "screen",
			EventType:     "view",
			Year:          2024,
			Month:         6,
			Day:           15,
			Hour:          14,
		},
		{
			ID:            "evt-2",
			AppID:         "testapp",
			DeviceID:      "dev-2",
			TimestampMS:   time.Now().UnixMilli(),
			EventCategory: "user",
			EventType:     "login",
			Year:          2024,
			Month:         6,
			Day:           15,
			Hour:          14,
		},
	}

	// Note: This test requires the parquet-go library to be available
	// In the actual test, we would verify the bytes are valid Parquet
	data, err := writer.Write(rows)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Write() returned empty data")
	}

	// Check for Parquet magic bytes (PAR1)
	if len(data) >= 4 {
		magic := string(data[:4])
		if magic != "PAR1" {
			t.Errorf("Invalid Parquet magic bytes: got %q, want PAR1", magic)
		}
	}
}

func TestParquetWriter_WriteEmpty(t *testing.T) {
	writer := NewParquetWriter(ParquetConfig{
		Compression: "snappy",
	})

	_, err := writer.Write([]EventRow{})
	if err == nil {
		t.Error("Write() with empty rows should return error")
	}
}

func TestParquetWriter_Compression(t *testing.T) {
	tests := []struct {
		compression string
	}{
		{"snappy"},
		{"gzip"},
		{"zstd"},
		{"none"},
	}

	rows := []EventRow{
		{
			ID:            "evt-1",
			AppID:         "testapp",
			DeviceID:      "dev-1",
			TimestampMS:   time.Now().UnixMilli(),
			EventCategory: "screen",
			EventType:     "view",
			Year:          2024,
			Month:         6,
			Day:           15,
			Hour:          14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.compression, func(t *testing.T) {
			writer := NewParquetWriter(ParquetConfig{
				Compression: tt.compression,
			})

			data, err := writer.Write(rows)
			if err != nil {
				t.Fatalf("Write() with compression %q error = %v", tt.compression, err)
			}

			if len(data) == 0 {
				t.Error("Write() returned empty data")
			}
		})
	}
}
