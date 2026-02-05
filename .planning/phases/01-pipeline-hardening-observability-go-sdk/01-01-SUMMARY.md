---
phase: 01-pipeline-hardening-observability-go-sdk
plan: 01
subsystem: observability
tags: [otel, prometheus, metrics, protobuf]
requires:
  - phase: none
    provides: first plan in phase
provides:
  - OTel MeterProvider with Prometheus exporter
  - Metric instrument definitions for all Phase 1 modules
  - HTTP metrics middleware
  - idempotency_key proto field
affects: [01-02, 01-03, 01-04, 01-05, 01-06, 01-07, 01-08, 01-09, 01-10]
tech-stack:
  added: [go.opentelemetry.io/otel, go.opentelemetry.io/otel/sdk/metric, go.opentelemetry.io/otel/exporters/prometheus, github.com/prometheus/client_golang]
  patterns: [OTel MeterProvider setup, Prometheus metrics export, HTTP metrics middleware]
key-files:
  created: [internal/observability/module.go, internal/observability/metrics.go, internal/observability/middleware.go]
  modified: [proto/causality/v1/events.proto, pkg/proto/causality/v1/events.pb.go, pkg/proto/causality/v1/service_http_binding.pb.go, go.mod, go.sum]
key-decisions:
  - "Used OTel metric naming convention (dotted lowercase) for all instruments"
  - "Defined all Phase 1 metric instruments upfront in a single Metrics struct for shared use"
  - "Used string status attribute in middleware (not int) for Prometheus label cardinality"
  - "Placed idempotency_key at field number 7 (next available metadata slot in EventEnvelope)"
patterns-established:
  - "Observability Module pattern: New() returns Module with MeterProvider, Meter(), MetricsHandler(), Shutdown()"
  - "Metrics struct pattern: all instruments created via NewMetrics(meter) for consistent initialization"
  - "HTTP middleware pattern: HTTPMetrics(metrics) returns func(http.Handler) http.Handler, compatible with gateway.Chain()"
duration: 2min
completed: 2026-02-05
---

# Phase 1 Plan 01: Observability Foundation Summary

**OTel MeterProvider with Prometheus exporter, 16 metric instruments across HTTP/NATS/S3/dedup/DLQ/reaction, HTTP middleware, and idempotency_key proto field on EventEnvelope.**

## Performance

- Duration: ~2 minutes
- 2 tasks, 2 commits
- Zero compilation errors, zero vet warnings
- Full project `go build ./...` and `go vet ./...` pass

## Accomplishments

1. **Observability Module** (`internal/observability/module.go`): Created the central OTel setup with `prometheus.New()` exporter feeding into `sdkmetric.MeterProvider`. Exposes `MetricsHandler()` for mounting `/metrics` endpoint and `Meter()` for instrument creation. Follows the hexagonal module pattern with `New()`, `Shutdown()`, and accessor methods.

2. **Metric Instruments** (`internal/observability/metrics.go`): Defined 16 metric instruments in a `Metrics` struct covering all Phase 1 service domains:
   - HTTP: request duration (histogram), total requests (counter), error requests (counter)
   - NATS: messages processed, batch size, flush latency, ACK latency
   - S3: files written, file size
   - Dedup: dropped events counter
   - DLQ: depth (up-down counter)
   - Reaction: rules evaluated, alerts fired, webhook success/failure

3. **HTTP Metrics Middleware** (`internal/observability/middleware.go`): Created middleware following the existing `internal/gateway/middleware.go` pattern (function returning `func(http.Handler) http.Handler`). Records duration, total count, and error count with method/path/status attributes. Uses a `statusResponseWriter` wrapper to capture response status codes.

4. **Idempotency Key Proto Field**: Added `string idempotency_key = 7` to `EventEnvelope` in `proto/causality/v1/events.proto`. Regenerated Go code with `buf generate`. The generated `GetIdempotencyKey()` accessor is available for dedup and SDK plans.

## Task Commits

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Create observability module with OTel Prometheus metrics | `811f405` | internal/observability/module.go, metrics.go, middleware.go, go.mod, go.sum |
| 2 | Add idempotency_key field to EventEnvelope proto | `519953e` | proto/causality/v1/events.proto, pkg/proto/causality/v1/events.pb.go |

## Files Created/Modified

### Created
- `internal/observability/module.go` - OTel Module with MeterProvider, Prometheus exporter, lifecycle
- `internal/observability/metrics.go` - Metrics struct with 16 instrument definitions
- `internal/observability/middleware.go` - HTTP metrics middleware

### Modified
- `proto/causality/v1/events.proto` - Added idempotency_key field (number 7)
- `pkg/proto/causality/v1/events.pb.go` - Regenerated with IdempotencyKey
- `pkg/proto/causality/v1/service_http_binding.pb.go` - Regenerated (sebuf plugin update)
- `go.mod` - Added OTel, Prometheus dependencies
- `go.sum` - Updated checksums

## Decisions Made

1. **OTel metric naming**: Used dotted lowercase names (e.g., `http.request.duration`, `nats.messages.processed`) following OpenTelemetry semantic conventions rather than underscore-separated Prometheus conventions. The Prometheus exporter handles the translation automatically.

2. **Single Metrics struct**: All 16 instruments are defined in one `Metrics` struct and created together via `NewMetrics()`. This ensures consistent initialization and makes it easy to pass the full instrument set to any component.

3. **Status as string attribute**: The HTTP middleware records status code as `attribute.String("status", strconv.Itoa(status))` rather than `attribute.Int` for cleaner Prometheus label representation.

4. **Proto field number 7**: Used field 7 for `idempotency_key` as it is the next available metadata field number after `device_context = 6`, keeping metadata fields contiguous before the oneof payload block starting at 10.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. All dependencies resolved cleanly, buf generate succeeded on first run, and the full project compiled without errors after both tasks.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

All subsequent plans in Phase 1 are unblocked:
- **01-02 (NATS hardening)**: Can use `NATSMessagesProcessed`, `NATSBatchSize`, `NATSFlushLatency`, `NATSAckLatency` metrics
- **01-03 (Dedup)**: Can use `DedupDropped` metric and `IdempotencyKey` proto field
- **01-04 (DLQ)**: Can use `DLQDepth` metric
- **01-05 (S3 hardening)**: Can use `S3FilesWritten`, `S3FileSize` metrics
- **01-06 (Reaction engine)**: Can use `RulesEvaluated`, `AlertsFired`, `WebhookSuccess`, `WebhookFailure` metrics
- **01-07 (Server wiring)**: Can wire `Module.MetricsHandler()` to `/metrics` endpoint and `HTTPMetrics` middleware into the handler chain

---
*Phase: 01-pipeline-hardening-observability-go-sdk*
*Completed: 2026-02-05*
