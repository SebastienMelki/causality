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
	if req.Event == nil {
		return nil, fmt.Errorf("event is required")
	}

	// Enrich envelope with server-generated values
	s.enrichEnvelope(req.Event)

	// Publish to NATS
	if err := s.publisher.PublishEvent(ctx, req.Event); err != nil {
		s.logger.Error("failed to publish event",
			"event_id", req.Event.Id,
			"error", err,
		)
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	s.logger.Debug("event ingested",
		"event_id", req.Event.Id,
		"app_id", req.Event.AppId,
	)

	return &pb.IngestEventResponse{
		EventId: req.Event.Id,
		Status:  "accepted",
	}, nil
}

// IngestEventBatch handles batch event ingestion.
func (s *EventService) IngestEventBatch(ctx context.Context, req *pb.IngestEventBatchRequest) (*pb.IngestEventBatchResponse, error) {
	if len(req.Events) == 0 {
		return nil, fmt.Errorf("at least one event is required")
	}

	results := make([]*pb.EventResult, len(req.Events))
	acceptedCount := int32(0)
	rejectedCount := int32(0)

	for i, event := range req.Events {
		result := &pb.EventResult{
			Index: int32(i),
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
				"event_id", event.Id,
				"error", err,
			)
		} else {
			result.EventId = event.Id
			result.Status = "accepted"
			acceptedCount++
		}

		results[i] = result
	}

	s.logger.Info("batch ingestion complete",
		"total", len(req.Events),
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
	if event.Id == "" {
		event.Id = uuid.Must(uuid.NewV7()).String()
	}

	// Set timestamp if not provided
	if event.TimestampMs == 0 {
		event.TimestampMs = time.Now().UnixMilli()
	}
}
