---
phase: 01-pipeline-hardening-observability-go-sdk
plan: 05
subsystem: streaming
tags: [nats, jetstream, dlq, dead-letter-queue, advisory, observability]

# Dependency graph
requires:
  - phase: 01-01
    provides: "Observability foundation with DLQDepth metric instrument"
provides:
  - "DLQ module with advisory listener for MaxDeliver events"
  - "NATS stream manager EnsureDLQStream method"
  - "DLQ depth metric tracking via observability.Metrics"
  - "DLQ count query for monitoring"
affects: [01-06, 01-07, 01-10]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "NATS advisory subscription for consumer lifecycle events"
    - "Core NATS subscription (not JetStream) for system events"
    - "Message republish with metadata headers for DLQ traceability"

key-files:
  created:
    - "internal/dlq/module.go"
    - "internal/dlq/internal/service/dlq_service.go"
  modified:
    - "internal/nats/config.go"
    - "internal/nats/stream.go"

key-decisions:
  - "Use core NATS subscription (not JetStream consumer) for advisory subjects since advisories are system events"
  - "Preserve original message metadata in X-DLQ-* headers for debugging and potential reprocessing"
  - "DLQ stream uses 30-day retention (720h) vs 7-day main stream to give time for investigation"
  - "Republish to dlq.<original-subject> to maintain subject-based routing in DLQ"

patterns-established:
  - "Advisory listener pattern: subscribe to $JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.<stream>.<consumer>"
  - "DLQ message enrichment: X-DLQ-Original-Subject, X-DLQ-Original-Stream, X-DLQ-Original-Consumer, X-DLQ-Original-Sequence, X-DLQ-Deliveries headers"
  - "Module facade delegates to internal service (hexagonal pattern)"

# Metrics
duration: 3min
completed: 2026-02-05
---

# Phase 01 Plan 05: Dead Letter Queue Summary

**DLQ module with NATS advisory listener capturing MaxDeliver-exceeded messages to CAUSALITY_DLQ stream with 30-day retention and OTel depth metrics**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-05T18:34:40Z
- **Completed:** 2026-02-05T18:37:17Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Extended NATS stream manager with EnsureDLQStream method using same create-or-update pattern as main stream
- Created DLQ module following hexagonal pattern with advisory listener for MaxDeliver events
- Failed messages are fetched by sequence, enriched with debugging headers, and republished to dlq.> subjects
- DLQ depth tracked via observability.Metrics.DLQDepth counter with consumer and subject attributes

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend NATS stream manager with DLQ stream and update consumer MaxDeliver** - `df3ea7c` (feat)
2. **Task 2: Create DLQ module with advisory listener** - `4137f29` (feat)

## Files Created/Modified
- `internal/nats/config.go` - Added DLQStreamName and DLQMaxAge to StreamConfig
- `internal/nats/stream.go` - Added EnsureDLQStream method for DLQ stream creation
- `internal/dlq/module.go` - DLQ module facade with Start/Stop/GetDLQCount API
- `internal/dlq/internal/service/dlq_service.go` - Advisory listener, message fetch-and-republish, metrics tracking

## Decisions Made
- Used core NATS subscription (not JetStream consumer) for advisory subjects since advisories are system-level events on core NATS, not JetStream messages
- Preserved original message metadata in X-DLQ-* headers (subject, stream, consumer, sequence, delivery count) for debugging and potential reprocessing
- DLQ stream configured with 30-day retention (vs 7-day main stream) to give operators time to investigate failed messages
- Republish uses dlq.<original-subject> pattern to maintain subject-based routing within the DLQ stream

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- DLQ module ready for integration into main.go wiring (future plan)
- EnsureDLQStream can be called alongside EnsureStream during service startup
- Consumer MaxDeliver already configured (5 for warehouse-sink, 3 for analysis-engine and alerting)
- DLQ count queryable for health check endpoints and monitoring dashboards

---
*Phase: 01-pipeline-hardening-observability-go-sdk*
*Completed: 2026-02-05*
