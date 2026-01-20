package nats

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// Publisher handles publishing events to NATS JetStream.
type Publisher struct {
	js         jetstream.JetStream
	streamName string
	logger     *slog.Logger
}

// NewPublisher creates a new event publisher.
func NewPublisher(js jetstream.JetStream, streamName string, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{
		js:         js,
		streamName: streamName,
		logger:     logger.With("component", "publisher"),
	}
}

// PublishEvent publishes a single event to the appropriate NATS subject.
func (p *Publisher) PublishEvent(ctx context.Context, event *pb.EventEnvelope) error {
	subject := p.deriveSubject(event)

	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	ack, err := p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("event published",
		"event_id", event.Id,
		"subject", subject,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// PublishEventBatch publishes multiple events to NATS.
// Returns the number of successfully published events and any error.
func (p *Publisher) PublishEventBatch(ctx context.Context, events []*pb.EventEnvelope) (int, error) {
	published := 0

	for _, event := range events {
		if err := p.PublishEvent(ctx, event); err != nil {
			p.logger.Error("failed to publish event in batch",
				"event_id", event.Id,
				"error", err,
			)
			// Continue with remaining events
			continue
		}
		published++
	}

	if published < len(events) {
		return published, fmt.Errorf("failed to publish %d of %d events", len(events)-published, len(events))
	}

	return published, nil
}

// PublishAsync publishes an event asynchronously and returns a future for the ack.
func (p *Publisher) PublishAsync(ctx context.Context, event *pb.EventEnvelope) (jetstream.PubAckFuture, error) {
	subject := p.deriveSubject(event)

	data, err := proto.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	future, err := p.js.PublishAsync(subject, data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish event async: %w", err)
	}

	return future, nil
}

// deriveSubject derives the NATS subject from the event envelope.
// Format: {kind}.{app_id}.{category}.{type}
// Example: events.myapp.screen.view
func (p *Publisher) deriveSubject(event *pb.EventEnvelope) string {
	category, eventType := p.getEventCategoryAndType(event)

	// Sanitize app_id for subject (replace dots with underscores)
	appID := strings.ReplaceAll(event.AppId, ".", "_")

	return fmt.Sprintf("events.%s.%s.%s", appID, category, eventType)
}

// getEventCategoryAndType returns the category and type of the event payload.
func (p *Publisher) getEventCategoryAndType(event *pb.EventEnvelope) (category, eventType string) {
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
			// Sanitize custom event name
			name := strings.ToLower(payload.CustomEvent.EventName)
			name = strings.ReplaceAll(name, " ", "_")
			name = strings.ReplaceAll(name, ".", "_")
			return "custom", name
		}
		return "custom", "unknown"

	default:
		// Fallback: use reflection to get type name
		if event.Payload != nil {
			t := reflect.TypeOf(event.Payload)
			return "unknown", strings.ToLower(t.Elem().Name())
		}
		return "unknown", "unknown"
	}
}

// DeriveSubjectForTest exposes subject derivation for testing.
func (p *Publisher) DeriveSubjectForTest(event *pb.EventEnvelope) string {
	return p.deriveSubject(event)
}
