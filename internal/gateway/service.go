package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/SebastienMelki/causality/internal/nats"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// EventService implements the event ingestion business logic.
// This service is used by HTTP handlers (sebuf-generated or manual).
type EventService struct {
	publisher *nats.Publisher
	logger    *slog.Logger
}

// NewEventService creates a new event service.
func NewEventService(publisher *nats.Publisher, logger *slog.Logger) *EventService {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventService{
		publisher: publisher,
		logger:    logger.With("component", "event-service"),
	}
}

// IngestEvent handles single event ingestion.
func (s *EventService) IngestEvent(ctx context.Context, req *pb.IngestEventRequest) (*pb.IngestEventResponse, error) {
	if req.GetEvent() == nil {
		return nil, ErrEventRequired
	}

	// Enrich envelope with server-generated values
	s.enrichEnvelope(req.GetEvent())

	// Publish to NATS
	if err := s.publisher.PublishEvent(ctx, req.GetEvent()); err != nil {
		s.logger.Error("failed to publish event",
			"event_id", req.GetEvent().GetId(),
			"error", err,
		)
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	s.logger.Debug("event ingested",
		"event_id", req.GetEvent().GetId(),
		"app_id", req.GetEvent().GetAppId(),
	)

	return &pb.IngestEventResponse{
		EventId: req.GetEvent().GetId(),
		Status:  "accepted",
	}, nil
}

// IngestEventBatch handles batch event ingestion.
func (s *EventService) IngestEventBatch(ctx context.Context, req *pb.IngestEventBatchRequest) (*pb.IngestEventBatchResponse, error) {
	if len(req.GetEvents()) == 0 {
		return nil, ErrAtLeastOneEvent
	}

	results := make([]*pb.EventResult, len(req.GetEvents()))
	acceptedCount := int32(0)
	rejectedCount := int32(0)

	for i, event := range req.GetEvents() {
		result := &pb.EventResult{
			Index: int32(i), //nolint:gosec // Index is bounded by batch size which is well under int32 max.
		}

		// Validate and enrich
		if event == nil {
			result.Status = "rejected"
			result.Error = "event is nil"
			rejectedCount++
			results[i] = result
			continue
		}

		s.enrichEnvelope(event)

		// Publish to NATS
		if err := s.publisher.PublishEvent(ctx, event); err != nil {
			result.Status = "rejected"
			result.Error = err.Error()
			rejectedCount++
			s.logger.Warn("failed to publish event in batch",
				"index", i,
				"event_id", event.GetId(),
				"error", err,
			)
		} else {
			result.EventId = event.GetId()
			result.Status = "accepted"
			acceptedCount++
		}

		results[i] = result
	}

	s.logger.Info("batch ingestion complete",
		"total", len(req.GetEvents()),
		"accepted", acceptedCount,
		"rejected", rejectedCount,
	)

	return &pb.IngestEventBatchResponse{
		AcceptedCount: acceptedCount,
		RejectedCount: rejectedCount,
		Results:       results,
	}, nil
}

// enrichEnvelope adds server-generated values to the event envelope.
func (s *EventService) enrichEnvelope(event *pb.EventEnvelope) {
	// Generate UUID v7 if not provided (time-sortable)
	if event.GetId() == "" {
		event.Id = uuid.Must(uuid.NewV7()).String()
	}

	// Set timestamp if not provided
	if event.GetTimestampMs() == 0 {
		event.TimestampMs = time.Now().UnixMilli()
	}
}
