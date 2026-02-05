---
phase: 01-pipeline-hardening-observability-go-sdk
plan: 03
subsystem: auth
tags: [api-keys, sha256, middleware, hexagonal, postgresql]

# Dependency graph
requires:
  - phase: 01-01
    provides: "Observability module and metrics patterns"
provides:
  - "Auth module with API key generation, validation, and revocation"
  - "HTTP middleware for X-API-Key header validation"
  - "Admin CRUD endpoints for key management"
  - "PostgreSQL migration for api_keys table"
affects: [01-06, 01-07, 01-08, phase-3]

# Tech tracking
tech-stack:
  added: []
  patterns: ["hexagonal auth module with ports/adapters", "SHA256 key hashing for high-entropy secrets", "context-injected app_id for downstream handlers"]

key-files:
  created:
    - internal/auth/module.go
    - internal/auth/ports.go
    - internal/auth/adapters.go
    - internal/auth/internal/domain/apikey.go
    - internal/auth/internal/service/key_service.go
    - internal/auth/internal/repo/key_repo.go
    - internal/auth/internal/handler/key_handler.go
    - internal/auth/migrations/001_create_api_keys.up.sql
    - internal/auth/migrations/001_create_api_keys.down.sql
  modified: []

key-decisions:
  - "SHA256 over bcrypt for API key hashing: keys are 256-bit random, brute-force infeasible, SHA256 enables fast constant-time lookup"
  - "Admin endpoints unprotected now, TODO for Phase 3 session auth + RBAC"
  - "UUID v7 for key IDs (time-sortable, consistent with event ID pattern)"
  - "Partial index on key_hash WHERE NOT revoked for fast active-key lookups"

patterns-established:
  - "Hexagonal auth module: ports.go (interfaces), adapters.go (HTTP middleware), internal/ (domain, service, repo, handler)"
  - "Admin route registration via Module.RegisterAdminRoutes(mux) pattern"
  - "Context-injected app_id via auth.GetAppID(ctx) for downstream use"

# Metrics
duration: 5min
completed: 2026-02-05
---

# Phase 1 Plan 3: Auth Module Summary

**API key auth module with SHA256 hashing, X-API-Key middleware, and admin CRUD endpoints following hexagonal pattern**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-05T18:32:55Z
- **Completed:** 2026-02-05T18:37:35Z
- **Tasks:** 3
- **Files created:** 9

## Accomplishments
- Complete auth vertical slice: domain, service, repository, handler, middleware, migrations
- API keys generated as 64-char hex strings (32 random bytes), stored as SHA256 hashes
- HTTP middleware validates X-API-Key header with skip for health/ready/metrics
- Admin endpoints: POST/DELETE/GET at /api/admin/keys for key lifecycle management
- List endpoint never exposes key hashes, create returns plaintext once only

## Task Commits

Each task was committed atomically:

1. **Task 1: Create auth domain, service, and repository** - `4c1e577` (feat) - committed by parallel 01-04 execution
2. **Task 2: Create auth module facade and HTTP middleware adapter** - `3104173` (feat)
3. **Task 3: Create admin API key management HTTP endpoints** - `4347a67` (feat)

## Files Created/Modified
- `internal/auth/module.go` - Module facade: New(), CreateKey, RevokeKey, ListKeys, AuthMiddleware, RegisterAdminRoutes
- `internal/auth/ports.go` - KeyStore interface and AppIDContextKey constant
- `internal/auth/adapters.go` - HTTP auth middleware with path skipping, GetAppID helper
- `internal/auth/internal/domain/apikey.go` - APIKey type, GenerateKey, HashKey (SHA256), ValidateKeyFormat
- `internal/auth/internal/service/key_service.go` - KeyService business logic: create, validate, revoke, list
- `internal/auth/internal/repo/key_repo.go` - PostgreSQL KeyRepository implementing KeyStore
- `internal/auth/internal/handler/key_handler.go` - Admin HTTP handlers: create, revoke, list keys
- `internal/auth/migrations/001_create_api_keys.up.sql` - Creates api_keys table with indexes
- `internal/auth/migrations/001_create_api_keys.down.sql` - Drops api_keys table

## Decisions Made
- **SHA256 over bcrypt:** API keys are 256-bit random strings (high entropy), making brute-force infeasible. SHA256 allows fast O(1) lookup vs bcrypt's intentional slowness. This is the standard approach for API key storage.
- **Admin endpoints unprotected:** Marked with TODO(phase-3) for session auth + RBAC. Acceptable for development; must be secured before production.
- **UUID v7 for key IDs:** Time-sortable UUIDs consistent with event ID generation pattern already established.
- **Partial PostgreSQL index:** `idx_api_keys_key_hash WHERE NOT revoked` optimizes the hot path (validating active keys) without indexing revoked keys.

## Deviations from Plan

### Auto-fixed Issues

**1. [Parallel Execution] Task 1 files committed by parallel 01-04 agent**
- **Found during:** Task 1
- **Issue:** A parallel execution of plan 01-04 (dedup module) also created and committed the auth domain, service, repo, ports, and migration files in commit `4c1e577`. The files were identical to what this plan specified.
- **Fix:** Verified committed files match plan spec exactly. Continued with Tasks 2 and 3 which were not yet created.
- **Impact:** Task 1 commit attributed to 01-04 instead of 01-03, but files are correct and complete.

---

**Total deviations:** 1 (parallel execution overlap)
**Impact on plan:** No code impact. All files correct and complete. Only commit attribution differs for Task 1.

## Issues Encountered
None - all tasks compiled and passed verification on first attempt.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Auth module ready for gateway integration (plan 01-06 or later)
- Middleware can be added to server.go middleware chain
- Admin routes can be registered on the ServeMux
- Database migration must be run before first use (`001_create_api_keys.up.sql`)
- Phase 3 must add session auth protection to admin endpoints

---
*Phase: 01-pipeline-hardening-observability-go-sdk*
*Completed: 2026-02-05*
