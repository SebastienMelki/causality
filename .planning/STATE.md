# Project State

## Current Position
- **Phase:** 1 of 6 — Pipeline Hardening, Observability & Go SDK
- **Plan:** 01-05 complete, 5 of 10 plans done
- **Wave:** 2 of 5 (wave 1 complete, wave 2 plans: 01-05 done)
- **Status:** In progress
- **Last activity:** 2026-02-05 — Completed 01-03-PLAN.md (Auth Module)

Progress: [█████░░░░░] 5/10 Phase 1 plans

## Accumulated Decisions
- Module pattern: hexagonal vertical slices (retcons pattern)
- GPG signing disabled: use `--no-gpg-sign` for all commits
- Model profile: quality (opus executors, sonnet verifier)
- OTel metric naming: dotted lowercase (e.g., `http.request.duration`) per OTel semantic conventions
- Single Metrics struct: all 16 instruments in one struct, created via `NewMetrics(meter)`
- Proto field 7 for idempotency_key (next available metadata slot before oneof at 10)
- Bloom filter double-check locking: RLock fast path + Lock re-check for concurrent safety
- Window/2 rotation interval for full sliding window coverage
- Empty idempotency keys pass through unchanged (backwards compatible)
- Two dedup adapter types: CheckDuplicate (single event) and FilterEvents (batch)
- ACK-after-write: NATS messages ACKed only after successful S3 upload, NAKed on failure
- Partition-level ACK/NAK: each partition handled independently in flush
- Poison messages use msg.Term() to prevent infinite redelivery
- Metrics as nil-safe constructor parameter on warehouse consumer
- Default WorkerCount=1 for backward compatibility
- DLQ uses core NATS subscription (not JetStream) for advisory subjects
- DLQ messages enriched with X-DLQ-* headers for traceability
- DLQ stream 30-day retention vs 7-day main stream for investigation time
- DLQ republish uses dlq.<original-subject> for subject-based routing
- API key hashing: SHA256 (not bcrypt) for high-entropy 256-bit random keys — fast lookup, brute-force infeasible
- Admin key endpoints unprotected until Phase 3 session auth + RBAC
- Partial PostgreSQL index on key_hash WHERE NOT revoked for optimized active key lookup
- Auth middleware skips /health, /ready, /metrics paths
- Context-injected app_id via auth.GetAppID(ctx) for downstream handler use

## Completed
- Project initialization
- Research phase
- Requirements definition
- Roadmap creation
- Phase 1 planning (10 plans, 5 waves)
- **01-01**: Observability foundation (OTel + Prometheus metrics + HTTP middleware + idempotency_key proto)
- **01-02**: NATS hardening (ACK-after-write consumer with trackedEvent, worker pool, graceful shutdown with timeout)
- **01-04**: Dedup module (sliding window bloom filter with bits-and-blooms/bloom/v3, gateway + consumer adapters)
- **01-05**: Dead letter queue (DLQ module with NATS advisory listener, CAUSALITY_DLQ stream, OTel depth metrics)
- **01-03**: Auth module (API key generation with SHA256 hashing, X-API-Key middleware, admin CRUD endpoints)

## Blockers
- None

## Session Continuity
- **Last session:** 2026-02-05T18:37:35Z
- **Stopped at:** Completed 01-03-PLAN.md
- **Resume file:** None
- **Next plans:** 01-06 (Gateway integration), 01-07 (Trino), 01-08 (Go SDK) — waves 3-4
