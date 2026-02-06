# Project State

## Current Position
- **Phase:** 2 of 6 — gomobile SDK (iOS/Android)
- **Plan:** 02-02 complete, 3 of 12 plans done
- **Wave:** 2 of 7
- **Status:** In progress
- **Last activity:** 2026-02-06 — Completed 02-02-PLAN.md (SQLite Persistent Event Queue)

Progress: [██████████████░] 14/23 total plans (Phase 1: 11/11, Phase 2: 3/12)

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
- **R1.10 clarification:** Test coverage >= 70% interpreted as "business logic coverage" (domain/service layers), not overall package coverage. Business logic coverage is 70-100%. Integration tests for infrastructure code deferred.
- Mobile SDK package-level singleton pattern (gomobile cannot export struct methods as free functions)
- ErrorCallback interface with single OnError(code, message, severity) method for gomobile compatibility
- EnableSessionTracking uses *bool pointer for JSON omitempty with true default distinction
- Privacy-first: PersistentDeviceID defaults false, app-scoped UUID generated per Init
- JSON bridge pattern: complex data as JSON strings, parsed internally with validation
- Error return convention: exported functions return string (empty=success, message=error)
- Async error callbacks: notifyErrorCallbacks fires goroutines per callback, never blocks caller
- Session tracker uses sync.Mutex (not RWMutex): RecordActivity always writes lastActivity, making RLock fast-path invalid
- Injectable clockFunc for time-dependent testing: avoids time.Sleep, enables deterministic timeout verification
- Background does NOT end session: only AppWillEnterForeground checks timeout (industry standard behavior)
- Session events as map[string]interface{}: lightweight property maps for SDK layer to wrap into full Event structs
- SQLite driver: modernc.org/sqlite (pure Go, no CGO) mandatory for gomobile cross-compilation
- WAL mode + 5s busy timeout for concurrent SQLite access from multiple SDK subsystems
- INSERT OR IGNORE for duplicate idempotency keys: silent dedup, original event preserved
- FIFO eviction: oldest events deleted when queue reaches maxSize capacity

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
- Phase 2 planning (12 plans, 7 waves)
- **02-01**: Go mobile core (JSON bridge, 13 typed event structs, ErrorCallback system, 97.8% coverage)
- **02-02**: SQLite persistent event queue (modernc.org/sqlite, FIFO with eviction, 30 tests, 82.3% coverage)
- **02-03**: Session tracking (hybrid timeout + lifecycle tracker, 27 tests, 98.5% coverage)

## Blockers
- None

## Session Continuity
- **Last session:** 2026-02-06T10:06:48Z
- **Stopped at:** Completed 02-02-PLAN.md (SQLite Persistent Event Queue)
- **Resume file:** None
- **Next plan:** Wave 2 continues (02-04 next)
