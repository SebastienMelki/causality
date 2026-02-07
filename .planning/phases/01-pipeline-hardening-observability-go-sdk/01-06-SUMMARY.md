---
phase: 01-pipeline-hardening-observability-go-sdk
plan: 06
subsystem: gateway
tags: [auth, rate-limit, dedup, validation, observability]
requires:
  - phase: 01
    plan: 01
    provides: Observability module and metrics
  - phase: 01
    plan: 03
    provides: Auth module and middleware
  - phase: 01
    plan: 04
    provides: Dedup module
provides:
  - Fully secured HTTP gateway with auth, rate limiting, dedup, validation
  - /metrics endpoint for Prometheus scraping
  - Admin key management routes at /api/admin/keys
affects: []
tech-stack:
  added: []
  patterns: [per-key rate limiting, body size limit, event validation]
key-files:
  created: []
  modified: [internal/gateway/server.go, internal/gateway/middleware.go, internal/gateway/service.go, internal/gateway/config.go, cmd/server/main.go, docker-compose.yml]
key-decisions:
  - "Per-key rate limiting using sync.Map with periodic cleanup"
  - "Body size limit via http.MaxBytesReader (default 5MB)"
  - "Event validation: app_id, payload (event_type), timestamp_ms required"
  - "Server generates idempotency_key (UUID) if not provided"
  - "Duplicate events silently dropped (return success to client)"
  - "Middleware order: RequestID → Logging → Recovery → HTTPMetrics → CORS → BodySize → Auth → RateLimit → ContentType"
  - "Admin routes mounted outside auth middleware for key management"
patterns-established:
  - "ServerOpts pattern: optional dependencies passed via struct to NewServer"
  - "DedupChecker interface for loose coupling between gateway and dedup module"
duration: 4min
completed: 2026-02-05
---

# Phase 1 Plan 06: Gateway Integration Summary

**Integrated auth middleware, per-key rate limiting, body size limits, event validation, dedup check, and observability into the HTTP gateway server. All Wave 2 modules now wired together.**

## Performance

- Duration: ~4 minutes
- 2 tasks, 2 commits
- Full project `go build ./...` and `go vet ./...` pass

## Accomplishments

1. **Per-Key Rate Limiting** (`internal/gateway/middleware.go`): Added `PerKeyRateLimit` middleware using `sync.Map` to maintain per-API-key token bucket rate limiters. Reads `app_id` from context (set by auth middleware). Default: 1000 requests/sec, 2000 burst. Includes periodic cleanup goroutine (every 5 minutes).

2. **Body Size Limiting** (`internal/gateway/middleware.go`): Added `BodySizeLimit` middleware using `http.MaxBytesReader`. Default: 5MB. Protects against oversized requests.

3. **Event Validation** (`internal/gateway/service.go`): Added validation for required fields: `app_id` must not be empty, `payload` (which becomes `event_type`) must not be empty, `timestamp_ms` must be > 0. Returns 400 with descriptive error messages.

4. **Dedup Integration** (`internal/gateway/service.go`): Added `DedupChecker` interface and wired into `EventService`. Events with duplicate `idempotency_key` are silently dropped (return success to client). Server generates UUID for `idempotency_key` if not provided by client.

5. **Server Wiring** (`internal/gateway/server.go`): Updated `NewServer` to accept `ServerOpts` with auth middleware, metrics handler, metrics, dedup checker, and admin route registrar. Middleware chain order: RequestID → Logging → Recovery → HTTPMetrics → CORS → BodySizeLimit → authMiddleware → PerKeyRateLimit → ContentType.

6. **Main Wiring** (`cmd/server/main.go`): Complete rewrite to wire all modules: observability, PostgreSQL database, auth module, dedup module, NATS, and HTTP server. Proper graceful shutdown sequence. Added `DatabaseConfig` for PostgreSQL connection.

7. **Docker Compose** (`docker-compose.yml`): Added PostgreSQL dependency, database environment variables, dedup/rate-limit config for `causality-server` service.

## Task Commits

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Add per-key rate limiting, body size limits, event validation, and dedup integration | `29ea9cd` | internal/gateway/middleware.go, service.go, config.go |
| 2 | Wire auth, dedup, observability, and new middleware into server and main | (combined) | internal/gateway/server.go, cmd/server/main.go, docker-compose.yml |

## Files Modified

- `internal/gateway/config.go` — Added MaxBodySize, MaxBatchEvents, PerKeyRPS, PerKeyBurst
- `internal/gateway/middleware.go` — Added PerKeyRateLimit, BodySizeLimit middleware
- `internal/gateway/service.go` — Added DedupChecker, validation, idempotency_key enrichment
- `internal/gateway/server.go` — Updated NewServer to accept ServerOpts, wired middleware chain
- `internal/gateway/errors.go` — Added validation error types
- `cmd/server/main.go` — Complete rewrite with all module wiring
- `docker-compose.yml` — Added database config and new env vars for causality-server

## Decisions Made

1. **Middleware order**: Auth comes before rate limiting so rate limits are per authenticated key, not per IP.

2. **ServerOpts pattern**: Instead of many constructor parameters, optional dependencies passed via struct.

3. **DedupChecker interface**: Loose coupling — gateway doesn't import dedup directly, just the interface.

4. **Silent duplicate drop**: Return success to client for duplicate events (idempotent behavior).

5. **Server-generated idempotency_key**: If client doesn't provide one, server generates UUID. Ensures all events can be deduplicated.

## Deviations from Plan

None — plan executed as written.

## Issues Encountered

None.

## Next Phase Readiness

Gateway is fully secured and observable:
- Unauthenticated `/v1/events/ingest` returns 401
- Per-key rate limiting enforced
- Request body > 5MB rejected
- Events missing required fields return 400
- Duplicate events silently dropped
- Prometheus metrics at `/metrics`
- Admin key routes at `/api/admin/keys`

---
*Phase: 01-pipeline-hardening-observability-go-sdk*
*Completed: 2026-02-05*
