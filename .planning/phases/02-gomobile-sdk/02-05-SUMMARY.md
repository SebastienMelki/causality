---
phase: 02-gomobile-sdk
plan: 05
subsystem: mobile-sdk-transport
tags: [batching, http-transport, retry, exponential-backoff, jitter]
depends_on:
  requires: ["02-02"]
  provides: ["event-batching", "http-transport", "retry-strategy"]
  affects: ["02-06", "02-07", "02-08"]
tech_stack:
  added: []
  patterns: ["interface-based dependency injection", "dual-trigger batching", "exponential backoff with jitter"]
key_files:
  created:
    - sdk/mobile/internal/transport/retry.go
    - sdk/mobile/internal/transport/client.go
    - sdk/mobile/internal/batch/batcher.go
    - sdk/mobile/internal/batch/flush.go
    - sdk/mobile/internal/transport/retry_test.go
    - sdk/mobile/internal/transport/client_test.go
    - sdk/mobile/internal/batch/batcher_test.go
  modified: []
decisions:
  - id: batch-interface-abstraction
    decision: "EventQueue and EventSender interfaces abstract storage.Queue and transport.Client for testable dependency injection"
    rationale: "Enables unit testing with mocks without real SQLite or HTTP servers"
  - id: non-blocking-add
    decision: "Add() is non-blocking; flush triggered via buffered channel signal"
    rationale: "Mobile SDK must never block the calling thread on event tracking"
  - id: final-flush-on-stop
    decision: "Stop() performs a final flush before exiting the flush loop"
    rationale: "Ensures queued events are not lost when the SDK shuts down"
  - id: retry-after-header
    decision: "Retry-After header (seconds or HTTP-date) is respected, using the larger of header vs strategy delay"
    rationale: "Server-specified backoff should be honored but never reduce below the client's own backoff"
metrics:
  duration: "7m 9s"
  completed: "2026-02-06"
  tests: 43
  coverage_batch: "90.1%"
  coverage_transport: "87.4%"
---

# Phase 02 Plan 05: Event Batching and HTTP Transport Summary

Event batching with dual triggers (count + time) and HTTP transport with exponential backoff retry, using stdlib net/http only.

## What Was Built

### Transport Layer (`sdk/mobile/internal/transport/`)

**retry.go** - `RetryStrategy` interface and `ExponentialBackoff` implementation:
- `NextDelay(attempt)` calculates delay with exponential growth, max cap, and configurable jitter
- `DefaultRetry`: 1s base, 5m max, 10 retries, 20% jitter
- Jitter prevents thundering herd by randomizing delay +/- configured percentage

**client.go** - HTTP transport client:
- `NewClient(endpoint, apiKey, timeout, retry)` creates HTTP client with User-Agent header
- `SendBatch(ctx, events)` POSTs JSON array to `/v1/events/batch` with X-API-Key auth
- Retries on 5xx and 429 (Too Many Requests) with configurable strategy
- Immediate error return on 4xx (except 429) -- no retry for bad requests
- Respects `Retry-After` header (seconds or HTTP-date format)
- Context-based cancellation for all network operations
- `sleepWithContext()` enables clean cancellation during retry waits

### Batch Layer (`sdk/mobile/internal/batch/`)

**batcher.go** - Event batcher with dual triggers:
- `EventQueue` interface abstracts `storage.Queue` for testability
- `EventSender` interface abstracts `transport.Client` for testability
- `Add(eventJSON, idempotencyKey)` enqueues to persistent storage, non-blocking
- Count trigger: when `pendingCount >= batchSize`, signals flush channel
- `Flush(ctx)` dequeues events, sends via transport, deletes on success
- On send failure: calls `MarkRetry(id)` for each event, events stay in queue
- `SetOnError(fn)` for optional error callback

**flush.go** - Background flush loop:
- `StartFlushLoop(ctx)` launches goroutine with `time.NewTicker`
- Dual select: ticker (time trigger) + flushCh (count trigger) + stopCh + ctx.Done()
- `Stop()` closes stopCh, performs final flush, waits for goroutine exit

## Tests

**43 tests total, all pass with `-race` flag:**

### Transport Tests (28 tests)
- `TestExponentialBackoff_*` (7): delay increase, max cap, max retries stop, jitter variation, zero jitter determinism
- `TestSendBatch_*` (12): success, empty batch, 5xx retry, 429 retry, 4xx no-retry, 403 no-retry, all retries exhausted, context cancellation, 502/504 retry, response parsing
- `TestRetryDelay_*` (6): no header, seconds header, smaller-than-strategy header, HTTP-date header, invalid value, exhausted retries
- `TestIsRetryableStatus` (1): table-driven status code classification
- `TestNewClient` (1): constructor validation

### Batch Tests (15 tests)
- `TestNewBatcher_*` (2): minimum enforcement, valid values
- `TestAdd_*` (3): enqueue, enqueue error, batch-size flush trigger
- `TestFlush_*` (4): send and delete, empty queue, failed events kept, dequeue error
- `TestFlushLoop_*` (1): periodic flush verification
- `TestStop_*` (2): wait for completion, final flush error callback
- `TestSetOnError` (1): error callback configuration
- `TestFlush_BatchSizeLimitsDequeue` (1): dequeue respects batch size
- `TestContextCancellation_StopsFlushLoop` (1): clean goroutine exit

## Key Design Decisions

1. **Interface-based DI**: `EventQueue` and `EventSender` interfaces enable full unit testing with mocks -- no SQLite or HTTP server required in tests
2. **Non-blocking Add**: `flushCh` is buffered (capacity 1), so Add never blocks the caller even if a flush is already pending
3. **Final flush on Stop**: Stop() closes stopCh which triggers one last flush in the select loop before goroutine exits
4. **Retry-After respects server**: Uses `max(headerDelay, strategyDelay)` so server-specified backoff is honored but never weakens client backoff

## Deviations from Plan

None -- plan executed exactly as written.

## Verification Results

```
go build ./sdk/mobile/internal/batch/        -- OK
go build ./sdk/mobile/internal/transport/     -- OK
go vet ./sdk/mobile/internal/batch/ ./sdk/mobile/internal/transport/ -- OK
go test -race -count=1: 43/43 pass, 0 failures, 0 races
batch coverage:     90.1%
transport coverage: 87.4%
```

## Next Phase Readiness

This plan provides the batching and transport layer needed by:
- **02-06** (SDK integration): Wire batcher into the main SDK singleton
- **02-07** (lifecycle management): Use batcher's Stop/Flush for app lifecycle events
- **02-08** (gomobile binding): Export Flush/Stop through the gomobile bridge
