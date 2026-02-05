# Roadmap — Milestone 1 (v1.0)

**Project:** Causality — Self-Hosted Behavioral Analytics Platform
**Milestone:** v1.0 — Complete analytics platform with LLM differentiator
**Created:** 2026-02-05
**Phases:** 6
**Status:** Not started

## Overview

```
Phase 1: Pipeline Hardening, Observability & Go SDK
  │
  ├──→ Phase 2: gomobile SDK (iOS + Android)        ┐
  │                                                   ├── parallel
  ├──→ Phase 3: Analytics Web App (React SPA + Auth) ┘
  │         │
  │         └──→ Phase 4: LLM Text-to-SQL Analysis
  │
  ├──→ Phase 5: Web SDK (JavaScript/TypeScript)
  │
  └──→ Phase 6: templ + htmx Public Pages
```

Phases 2, 3, 5, and 6 all depend on Phase 1 but are otherwise independent. Phases 2+3 can run in parallel. Phase 4 depends on Phase 3 (needs the React app and Trino query patterns). Phases 5 and 6 can start anytime after Phase 1.

---

## Phase 1: Pipeline Hardening, Observability & Go SDK

**Goal:** Fix the foundation — zero data loss, authentication, observability, and a Go backend SDK. Every subsequent phase depends on a reliable, authenticated, observable pipeline.

**Depends on:** Nothing (first phase)

**Research needed:** No — standard stream processing and observability patterns

**Delivers:**
- Graceful shutdown with ACK-after-write (R1.1)
- Event deduplication with idempotency keys (R1.2)
- Parquet file compaction job (R1.3)
- API key authentication for event ingestion (R1.4)
- OpenTelemetry + Prometheus metrics on all services (R1.5)
- NATS pull consumer with worker pool (R1.6)
- Dead letter queue for failed events (R1.7)
- Rate limiting and request validation (R1.8)
- Go backend SDK with batching and retry (R1.9)
- ≥70% test coverage (R1.10)

**Success criteria:**
- [ ] `kill -TERM <warehouse-sink>` → zero event loss (ACK-after-write verified)
- [ ] Duplicate events (same idempotency key) stored only once
- [ ] Compaction reduces small file count by ≥10x
- [ ] Unauthenticated `/v1/events/ingest` returns 401
- [ ] Prometheus metrics at `/metrics` on all services
- [ ] Go SDK sends batched events with retry
- [ ] Test coverage ≥70%

**Plans:** TBD (use `/gsd:plan-phase 1`)

**Key risks:**
- Compaction strategy: measure file counts first, then decide compaction vs. Iceberg
- ACK-after-write requires careful NATS consumer refactor

---

## Phase 2: gomobile SDK (iOS + Android)

**Goal:** Ship a cross-platform mobile SDK from a single Go codebase. One implementation, two platforms.

**Depends on:** Phase 1 (needs API key auth, idempotency keys, stable pipeline)

**Research needed:** No deep research — but allocate 1-2 days for gomobile build environment setup and compatibility testing (Go 1.24 + NDK version)

**Delivers:**
- gomobile bind library — .xcframework (iOS) + .aar (Android) (R2.1)
- JSON bridge API: Init, Track, SetUser, Reset, Flush, GetDeviceId (R2.2)
- SQLite persistent event queue (R2.3)
- Size-based + time-based batch flushing (R2.4)
- Offline retry with exponential backoff (R2.5)
- Device context auto-collection (R2.6)
- Automatic session tracking (R2.7)
- Swift wrapper (SPM) + Kotlin wrapper (Maven) + example apps (R2.8)

**Success criteria:**
- [ ] .xcframework integrates into fresh Xcode project and sends events to server
- [ ] .aar integrates into fresh Android Studio project and sends events to server
- [ ] Events persist through app kill and send on next launch
- [ ] SDK flushes on app background
- [ ] Offline events sent when connectivity returns
- [ ] Device context auto-populated

**Plans:** TBD (use `/gsd:plan-phase 2`)

**Key risks:**
- gomobile type restrictions: must design JSON bridge API carefully (irreversible)
- Go 1.24 + NDK compatibility: known gomobile issues with stack splitting (#68760)
- Build reproducibility: pin exact toolchain versions

---

## Phase 3: Analytics Web Application (React SPA + Auth)

**Goal:** Build the analytics web application — authentication, event segmentation charts, SQL editor, dashboards, and alert management. The primary user interface.

**Depends on:** Phase 1 (needs API key auth, observable pipeline, Trino querying)

**Research needed:** No — standard React + Go patterns. CodeMirror SQL autocomplete may need schema integration during implementation.

**Delivers:**
- API Gateway expansion with path-based routing (R3.1)
- JWT session auth with refresh tokens (R3.2)
- RBAC: admin, editor, viewer (R3.3)
- React 19 + Vite 6 + TanStack Router/Query + shadcn/ui app shell (R3.4)
- Event segmentation / trends charts (Recharts) (R3.5)
- SQL editor (CodeMirror 6) with schema browser and query history (R3.6)
- Real-time event stream via SSE (R3.7)
- Dashboard builder (save, compose, share) (R3.8)
- Alert management UI for reaction engine (R3.9)

**Success criteria:**
- [ ] User logs in, sees event segmentation chart, filters by properties
- [ ] SQL editor runs Trino queries and renders paginated results
- [ ] Dashboard with multiple charts shares global time range
- [ ] Alerts created/edited/tested from UI
- [ ] RBAC enforced: viewer cannot create alerts
- [ ] React SPA served from Go binary via go:embed

**Plans:** TBD (use `/gsd:plan-phase 3`)

**Key risks:**
- Large phase scope — may need to split into sub-plans (auth, shell, charts, SQL editor, dashboards, alerts)
- go:embed + React dev workflow needs careful setup (proxy for development, embed for production)
- Trino query latency for dashboards — may need query result caching

---

## Phase 4: LLM Text-to-SQL Analysis

**Goal:** Implement the primary differentiator — natural language queries that generate SQL, execute it, and explain results.

**Depends on:** Phase 3 (needs React app, Trino query execution patterns, analytics BFF API)

**Research needed:** YES — text-to-SQL prompting strategies for Trino, schema representation, few-shot examples, self-correction patterns. Use `/gsd:research-phase 4` before planning.

**Delivers:**
- LLM service (cmd/llm-service) with internal HTTP API (R4.1)
- Schema introspection and caching from Trino (R4.2)
- Prompt construction with schema context and Trino dialect tips (R4.3)
- SQL generation, validation, and safety checks (R4.4)
- Result explanation in plain English (R4.5)
- Conversational follow-ups with context (R4.6)
- "Ask AI" page in React app with streaming response (R4.7)
- LLM audit logging with token usage tracking (R4.8)

**Success criteria:**
- [ ] "How many events were received yesterday?" returns correct answer with chart
- [ ] Generated SQL references only existing tables/columns
- [ ] Non-SELECT SQL is never executed (DROP, DELETE, etc. blocked)
- [ ] Follow-up question correctly modifies previous query
- [ ] Result explanation matches the data
- [ ] All LLM interactions logged with token usage

**Plans:** TBD (use `/gsd:plan-phase 4`)

**Key risks:**
- Text-to-SQL accuracy: 67-86% on benchmarks, needs iterative prompt tuning
- SQL injection via prompt injection: read-only Trino user + query parsing mitigate
- LLM latency (1-60s): must not block other services (hence separate process)
- LLM cost at scale: monitor token usage, consider caching common queries

---

## Phase 5: Web SDK (JavaScript/TypeScript)

**Goal:** Ship a browser SDK as an npm package for web application instrumentation.

**Depends on:** Phase 1 (needs API key auth, stable pipeline)

**Research needed:** No — standard browser SDK patterns

**Delivers:**
- TypeScript npm package with ESM + UMD builds (R5.1)
- Event tracking API: init, track, identify, reset, page, flush (R5.2)
- Automatic page view and browser context tracking (R5.3)
- IndexedDB persistence with offline retry (R5.4)
- Batch transport with sendBeacon + fetch (R5.5)
- Session tracking with inactivity timeout (R5.6)

**Success criteria:**
- [ ] `npm install @causality/web-sdk` works
- [ ] Script tag inclusion works for non-bundled sites
- [ ] Events survive page refresh
- [ ] Automatic page views on SPA route changes
- [ ] Efficient batching (not one request per event)

**Plans:** TBD (use `/gsd:plan-phase 5`)

**Key risks:**
- Browser compatibility: need to test across Chrome, Firefox, Safari
- IndexedDB quirks in Safari (storage limits, private browsing)
- sendBeacon payload size limit (64KB)

---

## Phase 6: templ + htmx Public Pages

**Goal:** Build server-rendered public-facing pages using templ and htmx, served from the same Go gateway.

**Depends on:** Phase 1 (needs gateway), Phase 3 (routing coexistence with React SPA)

**Research needed:** No — standard server-rendered patterns

**Delivers:**
- templ templates for landing, features, docs, pricing pages (R6.1)
- htmx interactivity for dynamic sections (R6.2)
- Routing coexistence: templ owns `/`, React owns `/app/*` (R6.3)
- Shared Tailwind design system (R6.4)

**Success criteria:**
- [ ] Landing page renders at `/` with <100ms TTFB
- [ ] `/app/` routes load React SPA
- [ ] Consistent design language across templ and React pages
- [ ] Lighthouse SEO ≥90

**Plans:** TBD (use `/gsd:plan-phase 6`)

**Key risks:**
- Tailwind CSS sharing between templ and React needs coordinated build
- URL namespace conflicts between templ, React, and API routes

---

## Phase Execution Order

**Recommended sequence:**

1. **Phase 1** (Pipeline Hardening) — must be first, everything depends on it
2. **Phase 2 + Phase 3** in parallel — SDK feeds data, web app queries it
3. **Phase 4** (LLM) — after Phase 3 delivers the React app and Trino patterns
4. **Phase 5** (Web SDK) — anytime after Phase 1, but lower priority than 2-4
5. **Phase 6** (Public Pages) — last, since it's marketing/docs, not core analytics

**Parallelization opportunities:**
- Phase 2 ∥ Phase 3 (independent after Phase 1)
- Phase 5 can overlap with Phase 4 (independent)
- Phase 6 can overlap with Phase 4 or Phase 5

---

## Future Milestones

### v2.0 — Advanced Analytics & Production Deployment
- Phase 7: Advanced Analytics (funnels, retention, user paths, identity resolution)
- Phase 8: Kubernetes Deployment (Helm 4, HPA, StatefulSets)
- Phase 9: Agentic LLM (multi-step analysis, tool use, autonomous investigation)

---
*Created: 2026-02-05*
*Next action: `/gsd:plan-phase 1` to create the first phase execution plan*
