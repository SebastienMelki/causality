---
phase: 02-gomobile-sdk
plan: 02
subsystem: mobile-sdk-storage
tags: [sqlite, persistence, queue, gomobile, mobile-sdk]
dependency-graph:
  requires: ["02-01"]
  provides: ["persistent-event-queue", "sqlite-storage-layer", "schema-migrations"]
  affects: ["02-04", "02-05", "02-06"]
tech-stack:
  added: ["modernc.org/sqlite"]
  patterns: ["FIFO queue with eviction", "WAL mode SQLite", "schema migration versioning"]
key-files:
  created:
    - sdk/mobile/internal/storage/db.go
    - sdk/mobile/internal/storage/migrations.go
    - sdk/mobile/internal/storage/queue.go
    - sdk/mobile/internal/storage/db_test.go
    - sdk/mobile/internal/storage/queue_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - id: sqlite-driver
    choice: "modernc.org/sqlite (pure Go)"
    reason: "CGO-free is mandatory for gomobile cross-compilation"
  - id: wal-mode
    choice: "WAL journal mode with 5s busy timeout"
    reason: "Concurrent read/write access from multiple SDK subsystems"
  - id: duplicate-handling
    choice: "INSERT OR IGNORE for duplicate idempotency keys"
    reason: "Silent dedup without error propagation; original event preserved"
  - id: eviction-strategy
    choice: "Delete oldest by created_at before insert when at capacity"
    reason: "Simple FIFO eviction; newest events have highest delivery value"
metrics:
  duration: "4m 39s"
  completed: "2026-02-06"
  tests: 30
  coverage: "82.3%"
---

# Phase 02 Plan 02: SQLite Persistent Event Queue Summary

**SQLite-backed FIFO event queue with automatic eviction using modernc.org/sqlite (pure Go, no CGO) for gomobile compatibility.**

## What Was Built

### Task 1: SQLite Database Layer (ab85e81)
- **DB struct** wrapping `*sql.DB` with WAL mode and 5s busy timeout
- **Schema migration system** with version tracking table and transactional safety
- **Initial migration (v1)**: events table with `AUTOINCREMENT`, `idempotency_key UNIQUE`, `created_at` index, `retry_count`/`last_retry_at` index
- Convenience wrappers: `Exec`, `Query`, `QueryRow`, `Begin`, `Inner`
- Migrations run automatically on `NewDB()` open

### Task 2: FIFO Event Queue with Tests (11d624d)
- **Queue struct** with configurable `maxSize` and DB dependency injection
- **Enqueue**: inserts event with `INSERT OR IGNORE` for idempotency, evicts oldest when at capacity
- **DequeueBatch**: returns up to N events in FIFO order (`created_at ASC, id ASC`)
- **Delete**: batch removal by IDs after successful delivery
- **MarkRetry**: increments retry count and updates `last_retry_at`
- **Count/Clear**: queue depth inspection and full reset
- **30 tests** covering: success paths, FIFO ordering, eviction, duplicate handling, persistence across reopen, error paths after close, edge cases (zero/negative limits, empty IDs, nil inner)

## Deviations from Plan

None -- plan executed exactly as written.

## Key Technical Details

| Aspect | Detail |
|--------|--------|
| SQLite driver | `modernc.org/sqlite` v1.44.3 (pure Go, zero CGO) |
| Journal mode | WAL (Write-Ahead Log) for concurrent access |
| Busy timeout | 5000ms to handle lock contention |
| Eviction | Oldest events deleted when `count >= maxSize` |
| Dedup | `UNIQUE` constraint on `idempotency_key` + `INSERT OR IGNORE` |
| Ordering | `ORDER BY created_at ASC, id ASC` for stable FIFO |
| Coverage | 82.3% (uncovered: internal error branches in migrations rollback) |

## Verification Results

- **No CGO**: `go list -f '{{.CgoFiles}}' ./sdk/mobile/...` returns `[]` for all packages
- **Tests**: 30/30 pass
- **Coverage**: 82.3% (exceeds 80% requirement)
- **Build**: `go build ./sdk/mobile/internal/storage/` succeeds
- **go mod tidy**: succeeds cleanly

## Next Phase Readiness

This plan provides the storage layer that subsequent plans depend on:
- **02-04** (HTTP transport): will use `DequeueBatch` + `Delete` for send-and-confirm pattern
- **02-05** (batch flush): will orchestrate queue reads with timer-based flushing
- **02-06** (offline/retry): will use `MarkRetry` for exponential backoff tracking
