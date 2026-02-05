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

// DedupChecker checks whether an idempotency key has been seen before.
// Implementations must be safe for concurrent use.
type DedupChecker interface {
	// IsDuplicate returns true if the given key was already seen within
	// the dedup window. An empty key always returns false.
	IsDuplicate(key string) bool
}

// EventService implements the event ingestion business logic.
// This service is used by HTTP handlers (sebuf-generated or manual).
type EventService struct {
	publisher      *nats.Publisher
	dedup          DedupChecker
	maxBatchEvents int
	logger         *slog.Logger
}

// NewEventService creates a new event service. The dedup parameter is optional;
// pass nil to disable deduplication. The maxBatchEvents parameter controls the
// maximum number of events in a batch (0 means no limit).
func NewEventService(publisher *nats.Publisher, dedup DedupChecker, maxBatchEvents int, logger *slog.Logger) *EventService {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventService{
		publisher:      publisher,
		dedup:          dedup,
		maxBatchEvents: maxBatchEvents,
		logger:         logger.With("component", "event-service"),
	}
}

// IngestEvent handles single event ingestion.
func (s *EventService) IngestEvent(ctx context.Context, req *pb.IngestEventRequest) (*pb.IngestEventResponse, error) {
	if req.GetEvent() == nil {
		return nil, ErrEventRequired
	}

	event := req.GetEvent()

	// Validate required fields
	if err := s.validateEvent(event); err != nil {
		return nil, err
	}

	// Enrich envelope with server-generated values
	s.enrichEnvelope(event)

	// Check for duplicate (after enrich so idempotency_key is set)
	if s.dedup != nil && s.dedup.IsDuplicate(event.GetIdempotencyKey()) {
		s.logger.Debug("duplicate event silently dropped",
			"event_id", event.GetId(),
			"idempotency_key", event.GetIdempotencyKey(),
		)
		// Return success to client (silently drop)
		return &pb.IngestEventResponse{
			EventId: event.GetId(),
			Status:  "accepted",
		}, nil
	}

	// Publish to NATS
	if err := s.publisher.PublishEvent(ctx, event); err != nil {
		s.logger.Error("failed to publish event",
			"event_id", event.GetId(),
			"error", err,
		)
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	s.logger.Debug("event ingested",
		"event_id", event.GetId(),
		"app_id", event.GetAppId(),
	)

	return &pb.IngestEventResponse{
		EventId: event.GetId(),
		Status:  "accepted",
	}, nil
}

// IngestEventBatch handles batch event ingestion.
func (s *EventService) IngestEventBatch(ctx context.Context, req *pb.IngestEventBatchRequest) (*pb.IngestEventBatchResponse, error) {
	if len(req.GetEvents()) == 0 {
		return nil, ErrAtLeastOneEvent
	}

	// Check batch size limit
	if s.maxBatchEvents > 0 && len(req.GetEvents()) > s.maxBatchEvents {
		return nil, ErrBatchTooLarge
	}

	results := make([]*pb.EventResult, len(req.GetEvents()))
	acceptedCount := int32(0)
	rejectedCount := int32(0)

	for i, event := range req.GetEvents() {
		result := &pb.EventResult{
			Index: int32(i), //nolint:gosec // Index is bounded by batch size which is well under int32 max.
		}

		// Validate: nil event
		if event == nil {
			result.Status = "rejected"
			result.Error = "event is nil"
			rejectedCount++
			results[i] = result
			continue
		}

		// Validate required fields; skip invalid events
		if err := s.validateEvent(event); err != nil {
			result.Status = "rejected"
			result.Error = err.Error()
			rejectedCount++
			results[i] = result
			continue
		}

		// Enrich
		s.enrichEnvelope(event)

		// Dedup check
		if s.dedup != nil && s.dedup.IsDuplicate(event.GetIdempotencyKey()) {
			// Silently drop duplicates but report as accepted
			result.EventId = event.GetId()
			result.Status = "accepted"
			acceptedCount++
			results[i] = result
			s.logger.Debug("duplicate event in batch silently dropped",
				"index", i,
				"idempotency_key", event.GetIdempotencyKey(),
			)
			continue
		}

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

// validateEvent checks that an event has all required fields.
func (s *EventService) validateEvent(event *pb.EventEnvelope) error {
	if event.GetAppId() == "" {
		return ErrAppIDRequired
	}
	if event.GetPayload() == nil {
		return ErrEventTypeRequired
	}
	if event.GetTimestampMs() <= 0 {
		return ErrTimestampRequired
	}
	return nil
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

	// Generate idempotency key if not provided
	if event.GetIdempotencyKey() == "" {
		event.IdempotencyKey = uuid.New().String()
	}
}
