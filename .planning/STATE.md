# Project State

## Current Position
- **Phase:** 1 of 6 — Pipeline Hardening, Observability & Go SDK (COMPLETE)
- **Plan:** 01-11 complete, 11 of 11 plans done (gap closure)
- **Wave:** 5 of 5 (all waves complete)
- **Status:** Phase complete with gap closure
- **Last activity:** 2026-02-06 — Completed 01-11-PLAN.md (Test Coverage Gap Closure)

Progress: [███████████] 11/11 Phase 1 plans (including gap closure)

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
- Reaction engine metrics on port 9091 (warehouse sink uses 9090)
- Reaction engine shutdown timeout 30s default (no batch flush, unlike warehouse's 60s)
- Reaction engine per-message ACK (not deferred batch ACK like warehouse)
- Go SDK JSON over HTTP (not protobuf) for simplicity and minimal dependencies
- Go SDK full jitter exponential backoff: random delay between 0 and exponential delay
- Go SDK ServerContext collected once at client creation, reused for all events
- Mock interfaces for test isolation (mockJetStreamMsg, mockKeyStore, mockDedupChecker)
- Noop OTel meter for metrics testing without real collection
- Table-driven tests for validation logic
- EventPublisher interface for gateway service testability via dependency injection
- testableConsumer wrapper for workerLoop testing without full JetStream mocking
- Infrastructure-dependent code (S3, JetStream) requires integration tests for full coverage

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
- **01-06**: Gateway integration (auth middleware, per-key rate limiting, body size limits, dedup, validation, admin routes)
- **01-07**: Parquet compaction (merge small files to 128MB, cold partition filtering, hourly scheduler)
- **01-08**: Reaction engine observability (OTel/Prometheus metrics, DLQ advisory listener, worker pool, poison message Term())
- **01-09**: Go SDK (Track/Flush/Close, UUID idempotency keys, batching, exponential backoff retry)
- **01-10**: Test coverage (unit tests for auth, dedup, gateway, warehouse, compaction, SDK — 88-100% on business logic)
- **01-11**: Test coverage gap closure (gateway service.go 97.2%, warehouse consumer workerLoop 90.9%, compaction pure logic 100%)

## Blockers
- None

## Session Continuity
- **Last session:** 2026-02-06T22:45:00Z
- **Stopped at:** Completed 01-11-PLAN.md (Test Coverage Gap Closure), Phase 1 complete with gap closure
- **Resume file:** None
- **Next phase:** Phase 2 — gomobile SDK (iOS/Android)
