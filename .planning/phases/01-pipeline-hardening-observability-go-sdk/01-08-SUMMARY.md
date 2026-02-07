---
phase: 01-pipeline-hardening-observability-go-sdk
plan: 08
subsystem: observability
tags: [prometheus, otel, nats, dlq, worker-pool, reaction-engine, metrics]

# Dependency graph
requires:
  - phase: 01-01
    provides: observability module (OTel + Prometheus), Metrics struct with reaction instruments
  - phase: 01-02
    provides: worker pool pattern, ACK-after-write pattern, shutdown timeout pattern
  - phase: 01-05
    provides: DLQ module with advisory listener and CAUSALITY_DLQ stream
provides:
  - Reaction engine with Prometheus metrics at /metrics
  - Worker pool for parallel NATS message consumption
  - DLQ advisory listener for reaction engine consumer
  - Poison message termination via msg.Term()
  - Configurable shutdown timeout
affects: [01-09, 01-10]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Reaction engine observability wiring (same pattern as warehouse sink)"
    - "ConsumerConfig with WorkerCount/FetchBatchSize for reaction consumer"

key-files:
  created: []
  modified:
    - cmd/reaction-engine/main.go
    - internal/reaction/consumer.go
    - internal/reaction/config.go
    - docker-compose.yml

key-decisions:
  - "Reaction engine metrics on port 9091 (warehouse sink uses 9090)"
  - "Default 2 workers for reaction engine in Docker Compose (single-event processing vs batch)"
  - "Shutdown timeout 30s default for reaction (vs 60s for warehouse sink with batch flush)"

patterns-established:
  - "Observability wiring pattern: observability.New -> NewMetrics -> pass to consumer"
  - "DLQ wiring pattern: ensure DLQ stream -> dlq.New -> Start"

# Metrics
duration: 3min
completed: 2026-02-05
---

# Phase 01 Plan 08: Reaction Engine Observability Summary

**Reaction engine wired with OTel/Prometheus metrics, DLQ advisory listener, and worker pool for parallel consumption**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-05T18:42:55Z
- **Completed:** 2026-02-05T18:45:37Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Reaction consumer refactored with configurable worker pool (sync.WaitGroup pattern matching warehouse sink)
- Poison messages terminated via msg.Term() to prevent infinite redelivery
- Observability module, DLQ module, and Prometheus metrics endpoint wired into reaction engine main
- Docker Compose updated with metrics port 9091, stop_grace_period 1m, and new env vars

## Task Commits

Each task was committed atomically:

1. **Task 1: Add worker pool and metrics instrumentation to reaction consumer** - `293d172` (feat)
2. **Task 2: Wire observability, DLQ, and metrics endpoint into reaction engine main** - `96d1a2b` (feat)

## Files Created/Modified
- `internal/reaction/config.go` - Added ConsumerConfig (WorkerCount, FetchBatchSize) and ShutdownTimeout
- `internal/reaction/consumer.go` - Refactored with worker pool, poison message Term(), metrics instrumentation
- `cmd/reaction-engine/main.go` - Wired observability, DLQ, metrics HTTP server, proper shutdown ordering
- `docker-compose.yml` - Exposed port 9091, stop_grace_period, METRICS_ADDR/SHUTDOWN_TIMEOUT/CONSUMER_WORKER_COUNT

## Decisions Made
- Reaction engine metrics served on port 9091 to avoid conflict with warehouse sink on 9090
- Default 2 workers in Docker Compose for reaction engine (simpler per-event processing vs warehouse batching)
- Shutdown timeout defaults to 30s for reaction engine (no batch flush needed, unlike warehouse's 60s)
- Moved ACK into processMessage (reaction engine does per-message ACK, not deferred batch ACK like warehouse)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Reaction engine has full observability parity with warehouse sink
- Prometheus metrics endpoint ready for Grafana dashboards
- DLQ advisory listener captures max-delivery failures for investigation
- Ready for 01-09 (Go SDK) and 01-10 (integration testing)

---
*Phase: 01-pipeline-hardening-observability-go-sdk*
*Completed: 2026-02-05*
