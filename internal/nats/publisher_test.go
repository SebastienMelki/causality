package nats

import (
	"testing"

	"github.com/SebastienMelki/causality/internal/events"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

func TestDeriveSubject(t *testing.T) {
	publisher := &Publisher{
		streamName: "CAUSALITY_EVENTS",
	}

	tests := []struct {
		name     string
		event    *pb.EventEnvelope
		expected string
	}{
		{
			name: "screen view event",
			event: &pb.EventEnvelope{
				AppId:    "myapp",
				DeviceId: "device123",
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{
						ScreenName: "home",
					},
				},
			},
			expected: "events.myapp.screen.view",
		},
		{
			name: "user login event",
			event: &pb.EventEnvelope{
				AppId:    "testapp",
				DeviceId: "device456",
				Payload: &pb.EventEnvelope_UserLogin{
					UserLogin: &pb.UserLogin{
						UserId: "user123",
						Method: "email",
					},
				},
			},
			expected: "events.testapp.user.login",
		},
		{
			name: "button tap event",
			event: &pb.EventEnvelope{
				AppId:    "myapp",
				DeviceId: "device789",
				Payload: &pb.EventEnvelope_ButtonTap{
					ButtonTap: &pb.ButtonTap{
						ButtonId: "submit_btn",
					},
				},
			},
			expected: "events.myapp.interaction.button_tap",
		},
		{
			name: "purchase complete event",
			event: &pb.EventEnvelope{
				AppId:    "shop",
				DeviceId: "device000",
				Payload: &pb.EventEnvelope_PurchaseComplete{
					PurchaseComplete: &pb.PurchaseComplete{
						OrderId:    "order123",
						TotalCents: 9999,
					},
				},
			},
			expected: "events.shop.commerce.purchase_complete",
		},
		{
			name: "app start event",
			event: &pb.EventEnvelope{
				AppId:    "myapp",
				DeviceId: "device111",
				Payload: &pb.EventEnvelope_AppStart{
					AppStart: &pb.AppStart{
						IsColdStart: true,
					},
				},
			},
			expected: "events.myapp.system.app_start",
		},
		{
			name: "custom event",
			event: &pb.EventEnvelope{
				AppId:    "myapp",
				DeviceId: "device222",
				Payload: &pb.EventEnvelope_CustomEvent{
					CustomEvent: &pb.CustomEvent{
						EventName: "feature_flag_evaluated",
					},
				},
			},
			expected: "events.myapp.custom.feature_flag_evaluated",
		},
		{
			name: "app_id with dots gets sanitized",
			event: &pb.EventEnvelope{
				AppId:    "com.example.app",
				DeviceId: "device333",
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{
						ScreenName: "home",
					},
				},
			},
			expected: "events.com_example_app.screen.view",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := publisher.DeriveSubjectForTest(tt.event)
			if result != tt.expected {
				t.Errorf("DeriveSubject() = %q, want %q", result, tt.expected)
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
		{
			name: "screen view",
			event: &pb.EventEnvelope{
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{ScreenName: "home"},
				},
			},
			expectedCategory: "screen",
			expectedType:     "view",
		},
		{
			name: "screen exit",
			event: &pb.EventEnvelope{
				Payload: &pb.EventEnvelope_ScreenExit{
					ScreenExit: &pb.ScreenExit{ScreenName: "home"},
				},
			},
			expectedCategory: "screen",
			expectedType:     "exit",
		},
		{
			name: "swipe gesture",
			event: &pb.EventEnvelope{
				Payload: &pb.EventEnvelope_SwipeGesture{
					SwipeGesture: &pb.SwipeGesture{Direction: pb.SwipeDirection_SWIPE_DIRECTION_LEFT},
				},
			},
			expectedCategory: "interaction",
			expectedType:     "swipe",
		},
		{
			name: "add to cart",
			event: &pb.EventEnvelope{
				Payload: &pb.EventEnvelope_AddToCart{
					AddToCart: &pb.AddToCart{ProductId: "prod123"},
				},
			},
			expectedCategory: "commerce",
			expectedType:     "add_to_cart",
		},
		{
			name: "network change",
			event: &pb.EventEnvelope{
				Payload: &pb.EventEnvelope_NetworkChange{
					NetworkChange: &pb.NetworkChange{
						CurrentType: pb.NetworkType_NETWORK_TYPE_WIFI,
					},
				},
			},
			expectedCategory: "system",
			expectedType:     "network_change",
		},
		{
			name:             "nil payload",
			event:            &pb.EventEnvelope{},
			expectedCategory: "unknown",
			expectedType:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category, eventType := events.GetCategoryAndType(tt.event)
			if category != tt.expectedCategory {
				t.Errorf("category = %q, want %q", category, tt.expectedCategory)
			}
			if eventType != tt.expectedType {
				t.Errorf("eventType = %q, want %q", eventType, tt.expectedType)
			}
		})
	}
}
