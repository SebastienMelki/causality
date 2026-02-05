package dedup

import (
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// CheckDuplicate returns true if the given idempotency key has already
// been seen within the dedup window, indicating the event should be
// dropped. This is intended for use at the gateway/event-service level
// when processing individual ingest requests.
//
// An empty key always returns false (events without idempotency keys
// are never deduplicated).
func (m *Module) CheckDuplicate(idempotencyKey string) bool {
	return m.svc.IsDuplicate(idempotencyKey)
}

// FilterEvents takes a slice of EventEnvelope messages and returns only
// those whose idempotency keys have not been seen before. Events with
// empty idempotency keys always pass through. This is intended for use
// at the NATS consumer level when processing batches of events.
//
// The returned slice preserves the original order of non-duplicate events.
func (m *Module) FilterEvents(events []*pb.EventEnvelope) []*pb.EventEnvelope {
	if len(events) == 0 {
		return events
	}

	filtered := make([]*pb.EventEnvelope, 0, len(events))
	for _, evt := range events {
		if !m.svc.IsDuplicate(evt.GetIdempotencyKey()) {
			filtered = append(filtered, evt)
		}
	}

	return filtered
}
