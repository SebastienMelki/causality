---
phase: 01-pipeline-hardening-observability-go-sdk
plan: 07
subsystem: compaction
tags: [parquet, s3, compaction, scheduler]
requires:
  - phase: 01
    plan: 01
    provides: Observability module and metrics
  - phase: 01
    plan: 02
    provides: Warehouse consumer and S3 client
provides:
  - Parquet file compaction (merges small files into 128-256MB targets)
  - Scheduled compaction runner (hourly by default)
  - Cold partition filtering (never compact current hour)
affects: []
tech-stack:
  added: []
  patterns: [scheduled job, stateless idempotent design, row group merging]
key-files:
  created: [internal/compaction/module.go, internal/compaction/internal/service/compaction_service.go, internal/compaction/internal/service/scheduler.go]
  modified: [cmd/warehouse-sink/main.go, internal/observability/metrics.go, docker-compose.yml]
key-decisions:
  - "Stateless idempotent design: S3 file layout IS the state"
  - "Cold partition filtering: never compact current hour's partition"
  - "Delete originals ONLY after successful upload of merged file"
  - "Target size 128MB (configurable), minimum 2 files to trigger compaction"
  - "Hourly schedule by default (configurable)"
patterns-established:
  - "Compaction module pattern: service + scheduler + facade"
  - "S3Interface for testable S3 operations"
duration: 3min
completed: 2026-02-05
---

# Phase 1 Plan 07: Parquet Compaction Summary

**Created the Parquet compaction module that merges small files into larger ones (target 128-256 MB) on a scheduled basis. Reduces Trino query overhead from scanning many small files.**

## Performance

- Duration: ~3 minutes
- 2 tasks, commits combined
- Full project `go build ./...` and `go vet ./...` pass

## Accomplishments

1. **CompactionService** (`internal/compaction/internal/service/compaction_service.go`): Core compaction logic with:
   - `listColdPartitions()` — Lists S3 prefixes, filters to partitions older than current hour
   - `CompactPartition(ctx, partition)` — Downloads small files, merges row groups, uploads compacted file, deletes originals
   - `CompactAll(ctx)` — Lists all cold partitions, compacts each, records metrics
   - Uses `parquet.MergeRowGroups` for efficient row group merging
   - Stateless idempotent design: no database state tracking, S3 file layout IS the state

2. **Scheduler** (`internal/compaction/internal/service/scheduler.go`): Scheduled compaction runner with:
   - Configurable ticker interval (default: 1 hour)
   - Start/Stop lifecycle
   - `RunNow(ctx)` for manual trigger
   - Logs duration of each compaction run

3. **Compaction Module** (`internal/compaction/module.go`): Facade with:
   - Config: Schedule, TargetSize, MinFiles, Enabled, BucketName, BucketRegion
   - `New()`, `Start(ctx)`, `Stop()`, `RunNow(ctx)` methods
   - Passes S3 client interface for testability

4. **Compaction Metrics** (`internal/observability/metrics.go`): Added new metrics:
   - `compaction.runs` — Counter for completed compaction runs
   - `compaction.files_compacted` — Counter for files merged
   - `compaction.partitions_skipped` — Counter for partitions with no small files
   - `compaction.duration` — Histogram for compaction duration

5. **Warehouse Sink Wiring** (`cmd/warehouse-sink/main.go`): Integrated compaction module:
   - Creates compaction module after S3 client
   - Starts compaction scheduler
   - Stops compaction before consumer on shutdown (proper ordering)
   - Added Compaction config to main Config struct

## Task Commits

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Create compaction service with Parquet merge logic | `17c6fbe` | internal/compaction/internal/service/compaction_service.go, module.go, scheduler.go |
| 2 | Wire compaction into warehouse-sink | (included above) | cmd/warehouse-sink/main.go |

## Files Created

- `internal/compaction/module.go` — Compaction module facade
- `internal/compaction/internal/service/compaction_service.go` — Core compaction logic
- `internal/compaction/internal/service/scheduler.go` — Scheduled runner

## Files Modified

- `internal/observability/metrics.go` — Added compaction metrics
- `cmd/warehouse-sink/main.go` — Wired compaction module
- `docker-compose.yml` — Added compaction environment variables

## Decisions Made

1. **Stateless idempotent design**: No database state tracking. Each run lists files, filters to small ones, compacts if needed. Re-running on an already-compacted partition is a harmless no-op.

2. **Cold partition filtering**: Never compact current hour's partition (the hot partition being actively written to). Avoids race conditions with warehouse consumer.

3. **Delete after upload**: Original files are only deleted after successful upload of merged file. On failure, originals are left intact (no data loss).

4. **Target size 128MB**: Balances Trino query performance (fewer files to scan) with memory usage during compaction.

5. **S3Interface**: Uses interface instead of concrete S3 client for testability.

## Deviations from Plan

None — plan executed as written.

## Issues Encountered

None. Parquet-go library's `MergeRowGroups` API worked as expected.

## Next Phase Readiness

Compaction is fully operational:
- Merges small Parquet files into 128MB+ targets
- Only cold partitions compacted (current hour excluded)
- Runs hourly by default (configurable)
- Idempotent: safe to re-run
- Original files deleted only after successful merge
- CompactionRuns metric tracks execution
- Integrated into warehouse-sink with proper shutdown ordering

---
*Phase: 01-pipeline-hardening-observability-go-sdk*
*Completed: 2026-02-05*
