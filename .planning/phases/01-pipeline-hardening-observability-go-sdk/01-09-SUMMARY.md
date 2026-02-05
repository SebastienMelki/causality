---
phase: 01-pipeline-hardening-observability-go-sdk
plan: 09
subsystem: sdk
tags: [go-sdk, http-client, batching, retry, uuid, idempotency]

# Dependency graph
requires:
  - phase: 01-06
    provides: Gateway with batch endpoint accepting X-API-Key auth
provides:
  - Go SDK module at github.com/SebastienMelki/causality/sdk/go
  - Track(event) async event enqueue with idempotency key
  - Flush() synchronous batch send
  - Close() graceful shutdown with final flush
  - HTTP transport with exponential backoff retry
affects: [01-10, gomobile-sdk, web-sdk]

# Tech tracking
tech-stack:
  added: [github.com/google/uuid]
  patterns: [sdk-client-pattern, batch-flush-loop, exponential-backoff-jitter]

key-files:
  created:
    - sdk/go/causality.go
    - sdk/go/config.go
    - sdk/go/transport.go
    - sdk/go/batch.go
    - sdk/go/event.go
    - sdk/go/go.mod
    - sdk/go/go.sum
  modified: []

key-decisions:
  - "Minimal dependencies: only uuid for idempotency keys, stdlib for HTTP"
  - "JSON over HTTP instead of protobuf for SDK simplicity"
  - "Full jitter exponential backoff for retry (base 100ms, max 10s)"
  - "No retry on 4xx client errors, only 5xx server errors"
  - "ServerContext collected once at client creation, reused for all events"

patterns-established:
  - "SDK client pattern: New(cfg) returns *Client with Track/Flush/Close"
  - "Background flush loop: goroutine + ticker + context cancellation"
  - "Size and time triggered batch flush"
  - "sync.Once for idempotent Close()"

# Metrics
duration: 3min
completed: 2026-02-05
---

# Phase 01 Plan 09: Go SDK Summary

**Go backend SDK with Track/Flush/Close, UUID idempotency keys, batching, and exponential backoff retry**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-05T21:13:20Z
- **Completed:** 2026-02-05T21:16:09Z
- **Tasks:** 2
- **Files created:** 7

## Accomplishments
- Created standalone Go module at github.com/SebastienMelki/causality/sdk/go
- Implemented Track(event) for async event enqueue with automatic UUID idempotency keys
- Implemented Flush() for synchronous batch send and Close() for graceful shutdown
- HTTP transport with exponential backoff + jitter retry on 5xx errors
- Configurable batch size (default 50), flush interval (default 30s), max retries (default 3)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Go SDK module with event types, config, and transport** - `beeb892` (feat)
2. **Task 2: Create Go SDK client with Track, Flush, Close and batch loop** - `a30b1b8` (feat)

## Files Created/Modified
- `sdk/go/go.mod` - Go module definition with uuid dependency
- `sdk/go/go.sum` - Dependency checksums
- `sdk/go/event.go` - Event and ServerContext structs with JSON tags
- `sdk/go/config.go` - Config struct with validation and defaults
- `sdk/go/transport.go` - HTTP transport with retry and exponential backoff
- `sdk/go/batch.go` - Batcher with size/time-triggered flush loop
- `sdk/go/causality.go` - Client with New, Track, Flush, Close

## Decisions Made
- **JSON over HTTP**: SDK sends JSON to /v1/events/batch instead of protobuf. Simplifies SDK, no proto dependency, matches existing batch endpoint.
- **Minimal dependencies**: Only github.com/google/uuid for idempotency keys. HTTP client uses stdlib.
- **Full jitter backoff**: Random delay between 0 and exponential delay for better distribution under contention.
- **ServerContext once**: Collected at client creation, reused for all events to avoid repeated syscalls.
- **No 4xx retry**: Client errors indicate bad request, retrying won't help.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Go SDK ready for integration testing in 01-10
- Provides reference implementation for gomobile and web SDK patterns
- API contract: Track(event) with idempotency_key, Flush(), Close()

---
*Phase: 01-pipeline-hardening-observability-go-sdk*
*Completed: 2026-02-05*
