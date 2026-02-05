# Requirements — Milestone 1 (v1.0)

**Project:** Causality — Self-Hosted Behavioral Analytics Platform
**Milestone:** v1.0 — Complete analytics platform with LLM differentiator
**Defined:** 2026-02-05
**Source:** `.planning/research/SUMMARY.md`, `.planning/research/FEATURES.md`

## Milestone Goal

Ship a complete self-hosted behavioral analytics platform: SDKs collect events from mobile/web/backend apps, a hardened pipeline stores them reliably, a React web application provides dashboards and SQL access, and LLM-powered natural language queries let anyone analyze data without knowing SQL. All running locally via Docker Compose.

## Success Criteria (Milestone-Level)

- [ ] Events flow end-to-end: SDK → HTTP → NATS → Parquet → S3 → Trino → Dashboard
- [ ] gomobile SDK produces working .xcframework (iOS) and .aar (Android) artifacts
- [ ] Web SDK ships as npm package with automatic page view tracking
- [ ] Go backend SDK ships as Go module with batching
- [ ] Web application authenticates users and renders event segmentation charts
- [ ] SQL editor executes Trino queries and displays results
- [ ] Natural language query generates SQL, executes it, and explains results
- [ ] Public pages render via templ+htmx from the same Go gateway
- [ ] Zero data loss on graceful shutdown (events ACKed only after S3 write)
- [ ] API key authentication protects event ingestion endpoints
- [ ] Observability: Prometheus metrics exported from all services
- [ ] All services build and run via Docker Compose

---

## Phase 1: Pipeline Hardening, Observability & Go SDK

### Goal
Fix the foundation: zero data loss, authentication, observability, and a Go backend SDK. Every subsequent phase depends on this.

### Functional Requirements

#### R1.1 — Graceful Shutdown
- All services handle SIGTERM cleanly
- Warehouse sink flushes all in-flight batches to S3 before exiting
- Events are ACKed to NATS only after successful S3 upload (not before)
- Shutdown has a configurable timeout to prevent deadlock
- Docker Compose stop waits for flush (stop_grace_period ≥ 2x flush interval)

#### R1.2 — Event Deduplication
- SDK-generated idempotency key in every event (UUID, set by SDK)
- Server-side dedup using a sliding window (bloom filter or time-bounded set)
- Duplicate events are silently dropped (not errored)
- Dedup window is configurable (default: 10 minutes)

#### R1.3 — Parquet Compaction
- Background job merges small Parquet files into larger ones (target: 128–256 MB)
- Compaction runs on a schedule (e.g., hourly)
- Compaction updates Hive Metastore partition metadata
- Old small files are deleted after successful merge
- Compaction is idempotent (safe to re-run)

#### R1.4 — API Key Authentication
- API keys are per-app (tied to app_id)
- Keys are stored hashed in PostgreSQL
- Event ingestion endpoints require a valid API key (header: `X-API-Key`)
- Invalid/missing key returns 401
- Key management: create, revoke, list (admin API, no UI yet)

#### R1.5 — Observability (OpenTelemetry + Prometheus)
- All services export Prometheus metrics (request count, latency histogram, error rate)
- NATS consumer metrics: messages processed, batch size, flush latency, ACK latency
- Warehouse sink metrics: files written, file size, compaction runs
- Reaction engine metrics: rules evaluated, alerts fired, webhook delivery success/failure
- Structured JSON logging with correlation IDs (trace_id)
- Health endpoints (`/health`, `/ready`) on all services

#### R1.6 — Consumer Scaling
- NATS pull consumers with configurable worker pool
- Multiple warehouse-sink instances can run concurrently (no duplicate processing)
- Backpressure: consumer slows down if S3 upload is slow

#### R1.7 — Dead Letter Queue
- Events that fail processing after N retries are moved to a DLQ stream
- DLQ events are queryable (NATS subject or separate storage)
- Alert when DLQ depth exceeds threshold

#### R1.8 — Rate Limiting & Validation
- Per-API-key rate limiting (configurable, default: 1000 events/sec)
- Request body size limit (configurable, default: 5 MB)
- Event schema validation (required fields: app_id, event_type, timestamp)
- Oversized/malformed requests return 400 with descriptive error

#### R1.9 — Go Backend SDK
- Go module (`github.com/SebastienMelki/causality/sdk/go`)
- Track(event) — enqueue event for sending
- Flush() — force send all queued events
- Close() — flush and shutdown cleanly
- Configurable batch size and flush interval
- Automatic retry with exponential backoff
- Sets idempotency key (UUID) on each event
- Collects server context (hostname, Go version, SDK version)

#### R1.10 — Test Coverage
- Achieve ≥70% test coverage across pipeline components
- Unit tests for all business logic (dedup, compaction, rate limiting, auth)
- Integration tests for HTTP → NATS → S3 flow
- Tests for graceful shutdown scenarios

### Success Criteria
- [ ] `kill -TERM <warehouse-sink-pid>` results in zero event loss
- [ ] Duplicate events (same idempotency key) are stored only once
- [ ] Compaction reduces file count by ≥10x for high-throughput partitions
- [ ] Unauthenticated requests to `/v1/events/ingest` return 401
- [ ] Prometheus metrics visible at `/metrics` on all services
- [ ] Go SDK sends batched events and retries on failure
- [ ] Test coverage ≥70%

---

## Phase 2: gomobile SDK (iOS + Android)

### Goal
Ship a cross-platform mobile SDK from a single Go codebase using gomobile. One implementation serves both iOS and Android.

### Functional Requirements

#### R2.1 — gomobile Library Build
- gomobile bind produces .xcframework for iOS and .aar for Android
- CI-reproducible build with pinned Go toolchain and NDK versions
- Build tested on both ARM64 (device) and x86_64 (simulator)

#### R2.2 — JSON Bridge API
- Exported API uses only gomobile-compatible types (string, int, bool, []byte)
- `Init(configJSON string) error` — initialize SDK with app config
- `Track(eventJSON string) error` — enqueue event
- `SetUser(userJSON string) error` — set user identity
- `Reset() error` — clear user identity (logout)
- `Flush() error` — force send queued events
- `GetDeviceId() string` — return device identifier
- All complex data passed as JSON strings (no maps/slices in exported API)

#### R2.3 — Persistent Event Queue
- Events stored in SQLite before network send
- Events survive app kill, crash, and reboot
- Queue processes in FIFO order
- Maximum queue size configurable (default: 10,000 events)
- Old events evicted when queue is full (oldest first)

#### R2.4 — Batch Flushing
- Size-based trigger (default: 30 events)
- Time-based trigger (default: 30 seconds)
- Flush on app background transition
- Manual flush via `Flush()`
- Non-blocking: Track() returns immediately, flush happens on background goroutine

#### R2.5 — Offline Support & Retry
- Events queue locally when offline
- Automatic retry when connectivity returns
- Exponential backoff with jitter (1s, 2s, 4s, 8s, ... max 5 min)
- Network state detection (reachability check before flush attempt)

#### R2.6 — Device Context Auto-Collection
- OS name and version
- Device model and manufacturer
- App version and build number
- Screen resolution
- Locale and timezone
- SDK version
- Unique device ID (generated on first launch, persisted)

#### R2.7 — Session Tracking
- Automatic session detection (new session after 30s in background)
- Session ID generated per session
- Session start/end events emitted automatically
- Session duration tracked

#### R2.8 — Native Wrapper Libraries
- Swift wrapper package (SPM-compatible) with idiomatic Swift API
- Kotlin wrapper (Maven-compatible) with idiomatic Kotlin API
- Wrappers handle JSON serialization, provide typed event builders
- Example apps (iOS SwiftUI + Android Compose) demonstrating integration

### Success Criteria
- [ ] `.xcframework` integrates into a fresh Xcode project and sends events
- [ ] `.aar` integrates into a fresh Android Studio project and sends events
- [ ] Events survive app kill and are sent on next launch
- [ ] SDK flushes events on app background
- [ ] Offline events are sent when connectivity returns
- [ ] Device context auto-populated in every event

---

## Phase 3: Analytics Web Application (React SPA + Auth)

### Goal
Build the analytics web application: authentication, event segmentation charts, SQL editor, dashboard builder, and alert management. This is the primary interface for non-SDK users.

### Functional Requirements

#### R3.1 — API Gateway Expansion
- Single Go binary serves: event ingestion API, analytics BFF API, React SPA (go:embed), templ pages
- Path-based routing: `/v1/events/*` (ingestion), `/api/*` (analytics BFF), `/app/*` (React SPA), `/*` (templ pages)
- CORS configuration for development (React dev server on different port)
- Request logging middleware with correlation IDs

#### R3.2 — Session Authentication
- Email/password login (bcrypt password hashing)
- JWT access tokens (short-lived, 15 min)
- Refresh tokens (long-lived, 7 days, stored in PostgreSQL, rotated on use)
- Secure cookie transport for web app (HttpOnly, Secure, SameSite=Strict)
- Logout invalidates refresh token

#### R3.3 — RBAC (Role-Based Access Control)
- Three roles: admin, editor, viewer
- Admin: full access (user management, API key management, rule management)
- Editor: create/edit dashboards, queries, alerts
- Viewer: read-only access to dashboards and queries
- Roles assigned per user in PostgreSQL
- Middleware enforces role checks on API endpoints

#### R3.4 — React Application Shell
- React 19 + Vite 6 + TypeScript
- TanStack Router for type-safe routing
- TanStack Query for server state management
- Tailwind CSS 4 + shadcn/ui component library
- Responsive layout with sidebar navigation
- Dark mode support
- Error boundaries and loading states

#### R3.5 — Event Segmentation / Trends View
- Select event type(s) to chart
- Time-series line/bar chart (Recharts)
- Configurable time granularity (hour, day, week, month)
- Date range picker (relative: last 7/14/30/90 days; absolute: date picker)
- Property filters (e.g., platform = "iOS", app_version = "2.1")
- Property breakdowns (e.g., "break down by platform" → separate lines)
- Queries executed against Trino via BFF API

#### R3.6 — SQL Editor
- CodeMirror 6 with SQL language support
- Schema browser sidebar (tables, columns, types from Trino INFORMATION_SCHEMA)
- Query execution with results table (paginated, sortable)
- Query history (last 50 queries per user, stored in PostgreSQL)
- Saved queries (name, description, SQL, creator)
- Auto-LIMIT protection (queries without LIMIT get LIMIT 1000 appended)
- Keyboard shortcuts (Cmd+Enter to run, Cmd+S to save)

#### R3.7 — Real-Time Event Stream
- Server-Sent Events (SSE) endpoint streaming recent events
- Live tail view in web app showing events as they arrive
- Filterable by event type, app_id
- Pause/resume stream

#### R3.8 — Dashboard Builder
- Create named dashboards
- Add event segmentation charts to a dashboard
- Arrange charts in a grid layout
- Global time range applies to all charts
- Auto-refresh option (configurable interval)
- Share dashboards (by URL, respects RBAC)

#### R3.9 — Alert Management UI
- CRUD interface for reaction engine rules
- Create threshold/rate/count alerts
- Configure webhook delivery targets
- Enable/disable alerts
- View alert history (last N triggered alerts with timestamp and context)
- Test alert (fire manually to verify webhook delivery)

### Success Criteria
- [ ] User can log in, see event segmentation chart, and filter by properties
- [ ] SQL editor executes queries against Trino and renders results
- [ ] Dashboard with multiple charts renders with shared time range
- [ ] Alert rules can be created, edited, and tested from the UI
- [ ] RBAC: viewer cannot create alerts; admin can manage users
- [ ] React SPA served from Go binary via go:embed

---

## Phase 4: LLM Text-to-SQL Analysis

### Goal
Implement LLM-powered natural language querying — the primary differentiator. Users ask questions in plain English, the system generates SQL, executes it, and explains the results.

### Functional Requirements

#### R4.1 — LLM Service
- Separate Go service (cmd/llm-service) with internal HTTP API
- Isolated from event ingestion path (prevents slow LLM calls from blocking)
- Claude API integration via anthropic-sdk-go
- Configurable model selection (default: Claude Sonnet for speed, Claude Opus for complex queries)
- Request timeout (configurable, default: 60s)
- Rate limiting (per-user, configurable)

#### R4.2 — Schema Introspection
- Query Trino INFORMATION_SCHEMA for table/column metadata
- Cache schema in memory with configurable TTL (default: 5 min)
- Schema includes: table names, column names, column types, sample values for enum-like columns
- Schema formatted as structured context for LLM prompt

#### R4.3 — Prompt Construction
- System prompt with: role, schema context, Trino SQL dialect tips, output format instructions
- User message: natural language question
- Trino-specific dialect tips (DATE functions, UNNEST for arrays, APPROX_DISTINCT, etc.)
- Few-shot examples of common query patterns (event counts, time series, property breakdowns)
- Conversation history included for follow-up questions

#### R4.4 — SQL Generation & Validation
- LLM generates SELECT-only SQL
- Generated SQL validated against schema (all referenced tables/columns must exist)
- Non-SELECT statements rejected (DROP, DELETE, INSERT, UPDATE, ALTER, CREATE)
- Query executed via read-only Trino user (database-level safety net)
- Resource limits on Trino queries (max rows, max execution time)
- If query fails, error sent back to LLM for self-correction (one retry)

#### R4.5 — Result Explanation
- Query results sent back to LLM for plain-English explanation
- LLM highlights key findings, trends, and notable data points
- Chart type suggestion based on result shape (time series → line, categorical → bar, single value → metric card)
- Explanation displayed alongside the data table and chart

#### R4.6 — Conversational Follow-ups
- Conversation history maintained per session (PostgreSQL)
- Follow-up questions reference previous context ("Now break that down by platform")
- LLM modifies previous query based on follow-up intent
- Conversation can be saved/named for future reference

#### R4.7 — Natural Language UI
- "Ask AI" page in React app
- Text input with submit button
- Streaming response display (show SQL being generated, then results, then explanation)
- Generated SQL shown in expandable panel (users can copy/edit/re-run)
- Result visualization (table + suggested chart)
- Conversation thread with history
- Suggested questions based on available data

#### R4.8 — Audit & Logging
- All LLM requests logged (prompt, response, SQL generated, execution time, token usage)
- Audit trail for generated queries (who asked, what SQL was generated, what was returned)
- Token usage tracking for cost monitoring

### Success Criteria
- [ ] User types "How many events were received yesterday?" and gets correct answer with chart
- [ ] Generated SQL is valid Trino SQL referencing only existing tables/columns
- [ ] Non-SELECT SQL is never executed
- [ ] Follow-up question modifies previous query correctly
- [ ] Result explanation is coherent and matches the data
- [ ] All LLM interactions are logged with token usage

---

## Phase 5: Web SDK (JavaScript/TypeScript)

### Goal
Ship a browser SDK as an npm package for instrumenting web applications.

### Functional Requirements

#### R5.1 — TypeScript Package
- Published as npm package
- Dual build: ESM (import) + UMD (script tag / CDN)
- Zero runtime dependencies
- TypeScript types included
- Tree-shakeable exports

#### R5.2 — Event Tracking API
- `init(config)` — initialize with API key and endpoint
- `track(eventType, properties)` — track custom event
- `identify(userId, traits)` — set user identity
- `reset()` — clear identity (logout)
- `page(name, properties)` — track page view
- `flush()` — force send queued events

#### R5.3 — Automatic Tracking
- Automatic page view on route change (configurable, default: on)
- Page visibility tracking (tab hidden/visible)
- Referrer and UTM parameter capture
- Browser context: user agent, screen size, viewport, timezone, locale

#### R5.4 — Persistence & Offline
- IndexedDB for event queue (fallback: localStorage)
- Events survive page refresh and browser restart
- Automatic retry when online (navigator.onLine + online event)
- Queue size limit (configurable, default: 5000 events)

#### R5.5 — Batching & Transport
- Batch events (configurable: size=20, interval=10s)
- Use `navigator.sendBeacon` for page unload events
- `fetch` with keepalive for normal sends
- requestIdleCallback for non-urgent batching
- Retry with exponential backoff

#### R5.6 — Session Tracking
- Session ID generated per browser session
- New session after 30 min inactivity (configurable)
- Session stored in sessionStorage
- Session start/end events

### Success Criteria
- [ ] `npm install @causality/web-sdk` works
- [ ] Script tag inclusion works for non-bundled sites
- [ ] Events survive page refresh and are sent on next load
- [ ] Automatic page views tracked on SPA route changes
- [ ] Events batch and send efficiently (not one request per event)

---

## Phase 6: templ + htmx Public Pages

### Goal
Build server-rendered public-facing pages (landing, docs, pricing) using templ templates and htmx for interactivity, served from the same Go gateway.

### Functional Requirements

#### R6.1 — templ Templates
- templ v0.3.x for type-safe Go HTML templates
- Pages: landing page, features overview, documentation, pricing
- Responsive design (Tailwind CSS, shared with React app)
- SEO-friendly server-rendered HTML
- Static asset serving (CSS, images, fonts)

#### R6.2 — htmx Interactivity
- htmx 2.0 for dynamic page sections without full-page reload
- Contact form with server-side validation
- Documentation search/filter
- Smooth page transitions

#### R6.3 — Routing Coexistence
- templ pages own: `/`, `/features`, `/docs`, `/pricing`, `/contact`
- React SPA owns: `/app/*`
- API owns: `/v1/*`, `/api/*`
- Clear URL namespace with no conflicts
- SPA fallback handler for `/app/*` routes (returns index.html for client-side routing)

#### R6.4 — Content Management
- Content stored as Go constants or embedded files (no CMS)
- Easy to update without rebuilding (or accept rebuild for changes)
- Shared navigation/footer components across all templ pages

### Success Criteria
- [ ] Landing page renders at `/` with sub-100ms TTFB
- [ ] `/app/` routes load React SPA
- [ ] `/api/` routes return JSON
- [ ] All pages are responsive and share consistent design language
- [ ] templ pages pass Lighthouse SEO audit ≥90

---

## Out of Scope (v1.0)

These are explicitly deferred to future milestones:

| Feature | Reason | Milestone |
|---------|--------|-----------|
| Funnel analysis | Complex multi-step SQL + specialized UI. Build after event segmentation proves the pattern. | v2.0 |
| Retention analysis | Cohort calculation complexity. Needs identity resolution first. | v2.0 |
| User paths / flows | Sankey diagram rendering is specialized. High effort, medium value. | v2.0 |
| Identity resolution | Schema changes for user-device mapping. Required for retention. | v2.0 |
| Agentic LLM | Requires stable text-to-SQL foundation. Multi-step reasoning is exploratory. | v2.0 |
| Kubernetes deployment | Docker Compose is sufficient for internal team use. K8s adds premature complexity. | v2.0+ |
| Helm charts | Depends on K8s. | v2.0+ |
| Slack/email notifications | Webhook delivery works. Slack integration is nice-to-have. | v1.1 |
| Session replay | Anti-feature. Different product category. | Never |
| Feature flags | Anti-feature. Use dedicated tools. | Never |
| A/B testing | Anti-feature. Different domain. | Never |
| Auto-capture | Anti-feature. Explicit instrumentation via protobuf is better. | Never |
| Multi-tenant SaaS | Internal tool first. | TBD |

---

## Module Architecture

All new backend code follows the hexagonal module pattern (vertical slices):

```
internal/
  <module>/
    module.go          # Facade — exports Module struct, New(), wiring
    ports.go           # Interfaces (ports) consumed by this module
    adapters.go        # Implementations exported to other modules
    internal/
      domain/          # Domain types, value objects
      service/         # Business logic
      repo/            # Repository implementations (PostgreSQL)
    migrations/        # SQL migrations for this module's tables
```

Each module is a self-contained vertical slice. Dependencies between modules flow through ports (interfaces). Manual wiring in `main.go`.

---

## Cross-Cutting Concerns

### Security
- All passwords bcrypt-hashed (cost ≥12)
- JWT tokens with proper expiry and refresh rotation
- API keys hashed in database (never stored in plaintext)
- Read-only Trino user for LLM-generated queries
- CORS configured per environment
- Request body size limits on all endpoints
- Rate limiting on auth and LLM endpoints
- HMAC-signed webhooks (already exists)

### Observability
- Prometheus metrics on all services (`/metrics`)
- Structured JSON logging (zerolog or slog)
- Correlation IDs (trace_id) across service boundaries
- Health/readiness probes on all services

### Testing
- ≥70% test coverage target for new code
- Unit tests for business logic
- Integration tests for API endpoints
- E2E tests for critical flows (event ingestion → query)

### Infrastructure
- Docker Compose for local development
- Multi-stage Dockerfiles for all services
- Make targets for all common operations
- CI pipeline (build, test, lint) — runner TBD

---
*Defined: 2026-02-05*
*Source: Research phase completed 2026-02-05*
