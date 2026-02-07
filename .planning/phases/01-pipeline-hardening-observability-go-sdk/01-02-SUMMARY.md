---
phase: 01-pipeline-hardening-observability-go-sdk
plan: 02
subsystem: infra
tags: [nats, jetstream, s3, parquet, ack-after-write, worker-pool, graceful-shutdown, prometheus]

# Dependency graph
requires:
  - phase: 01-01
    provides: "OTel observability module with Metrics struct and Prometheus exporter"
provides:
  - "ACK-after-write consumer ensuring no data loss on crash"
  - "trackedEvent pattern for deferred NATS ACK/NAK"
  - "Configurable worker pool for parallel message consumption"
  - "Graceful shutdown with timeout protection and final flush"
  - "Prometheus metrics endpoint on warehouse-sink :9090"
  - "Poison message termination via msg.Term()"
affects: [01-03, 01-04, 01-05, 01-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "trackedEvent: retain NATS msg reference through batch-flush cycle for deferred ACK"
    - "partition-level ACK/NAK: ACK on successful S3 write, NAK on failure per partition"
    - "worker pool: configurable goroutine count with sync.WaitGroup coordination"
    - "shutdown timeout: context.WithTimeout on Stop to prevent deadlock"

key-files:
  created: []
  modified:
    - "internal/warehouse/consumer.go"
    - "internal/warehouse/config.go"
    - "cmd/warehouse-sink/main.go"
    - "docker-compose.yml"

key-decisions:
  - "Metrics parameter added to NewConsumer constructor (nil-safe throughout)"
  - "ACK/NAK happens per-partition: successful partitions ACK while failed ones NAK independently"
  - "Poison messages use msg.Term() not msg.Nak() to prevent infinite redelivery"
  - "ShutdownTimeout on Config (not BatchConfig) since it applies to the whole consumer lifecycle"
  - "Worker pool default is 1 for backward compatibility"

patterns-established:
  - "trackedEvent: pair domain object with NATS msg for deferred acknowledgment"
  - "Partition-isolated failure: one partition failing does not affect others in the same batch"
  - "Nil-safe metrics: all metric recording guarded by if c.metrics != nil"

# Metrics
duration: 5min
completed: 2026-02-05
---

# Phase 01 Plan 02: NATS Hardening Summary

**ACK-after-write consumer with trackedEvent pattern, configurable worker pool, and graceful shutdown with timeout protection**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-05T18:31:56Z
- **Completed:** 2026-02-05T18:36:33Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Eliminated data loss risk: messages are now ACKed only after successful S3 upload
- Failed S3 writes trigger NAK for automatic NATS redelivery
- Poison messages (unmarshal failures) are terminated to prevent infinite retry loops
- Worker pool enables parallel message consumption with configurable goroutine count
- Graceful shutdown flushes all in-flight batches with timeout protection
- Prometheus metrics endpoint exposed on warehouse-sink for operational visibility

## Task Commits

Each task was committed atomically:

1. **Task 1: Refactor warehouse consumer for ACK-after-write and worker pool** - `da2b4a1` (feat)
2. **Task 2: Wire observability and shutdown timeout into warehouse-sink main** - changes captured in `df3ea7c` (parallel agent commit)

**Plan metadata:** (see below)

## Files Created/Modified
- `internal/warehouse/consumer.go` - Refactored consumer with trackedEvent, worker pool, ACK-after-write flush
- `internal/warehouse/config.go` - Added WorkerCount, FetchBatchSize, ShutdownTimeout config fields
- `cmd/warehouse-sink/main.go` - Wired observability module, metrics server, shutdown timeout
- `docker-compose.yml` - Added stop_grace_period, metrics port, env vars for warehouse-sink

## Decisions Made
- **Metrics as constructor parameter:** Added `*observability.Metrics` to `NewConsumer` with nil-safe guards throughout, allowing the consumer to function without metrics if needed
- **Partition-level ACK/NAK:** Rather than ACK/NAK the entire batch together, each partition is handled independently -- a failed S3 write for one app_id/time partition does not prevent other partitions from being acknowledged
- **msg.Term() for poison messages:** Unmarshal failures use `Term()` (terminal acknowledgment) instead of `Nak()` to prevent NATS from redelivering messages that can never be processed
- **ShutdownTimeout at Config level:** Placed on `warehouse.Config` rather than `BatchConfig` since it governs the entire shutdown lifecycle, not just batching behavior
- **Default WorkerCount=1:** Conservative default maintains backward compatibility; operators can scale up via WORKER_COUNT env var

## Deviations from Plan

### Parallel Execution Overlap

Task 2 changes to `cmd/warehouse-sink/main.go` and `docker-compose.yml` were written to disk by this agent but committed by a parallel plan executor (01-05) in commit `df3ea7c`. This is because multiple plan agents ran concurrently and the 01-05 agent staged all modified files. The content is correct and complete.

---

**Total deviations:** 0 auto-fixed issues. 1 parallel execution overlap (no functional impact).
**Impact on plan:** No scope changes. All planned functionality delivered.

## Issues Encountered
None - plan executed as written. The parallel agent overlap was benign since the exact same changes were authored by this agent.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- ACK-after-write consumer is ready for integration with dedup (01-03) and DLQ (01-05)
- Metrics wiring establishes the pattern for instrumenting other services
- Worker pool is operational but defaults to 1; can be tuned after load testing
- No blockers for subsequent plans

---
*Phase: 01-pipeline-hardening-observability-go-sdk*
*Completed: 2026-02-05*
