# Project Research Summary

**Project:** Causality -- Self-Hosted Behavioral Analytics Platform
**Domain:** Behavioral analytics (event collection, storage, querying, dashboards, LLM-powered analysis)
**Researched:** 2026-02-05
**Confidence:** HIGH

## Executive Summary

Causality is a self-hosted behavioral analytics platform building on an already-working event pipeline (Go services, NATS JetStream, Parquet/S3, Trino). The existing infrastructure handles event ingestion, streaming, storage, and alerting but lacks SDKs, authentication, dashboards, and differentiated analysis capabilities. The research identifies a clear path to production: harden the pipeline first (deduplication, graceful shutdown, compaction, observability), then build the user-facing layers (SDKs, web application with React analytics dashboards, LLM-powered text-to-SQL analysis).

The recommended approach leverages the team's Go expertise across the entire stack: gomobile for cross-platform mobile SDKs (iOS + Android from one codebase), Go gateway serving both React SPA and templ+htmx public pages, separate LLM service for text-to-SQL, and Kubernetes deployment with Helm. The primary differentiator is self-hosted LLM-powered natural language analysis -- a capability that Amplitude (Spark AI) and PostHog (Max AI) only offer in their cloud products. Causality fills the gap for teams requiring data sovereignty.

The critical risks are well-documented: small Parquet files destroying query performance (requires compaction), LLM schema hallucination causing wrong results (requires schema injection and validation), data loss on pod shutdown (requires ACK-after-write and proper signal handling), gomobile type restrictions forcing API compromises (requires JSON bridge pattern), and SQL injection via prompt manipulation (requires read-only Trino user and query parsing). Each has established mitigation strategies that must be designed in from the start, not retrofitted.

## Key Findings

### Recommended Stack

The existing Go/NATS/Parquet/Trino pipeline is sound. Research recommends extending it with: gomobile for mobile SDKs (single Go codebase for iOS+Android), TypeScript for web SDK and React analytics app (Vite 6, TanStack Router/Query, Recharts 3, shadcn/ui), templ+htmx for public pages (type-safe Go templates), Claude/Anthropic SDK for LLM text-to-SQL, JWT+API keys for authentication, Helm 4 for Kubernetes deployment, and OpenTelemetry for observability.

**Core technologies:**
- **gomobile:** iOS+Android SDK from single Go codebase -- matches team skillset, avoids dual Swift+Kotlin maintenance, but has type restrictions (no maps/slices) requiring JSON bridge pattern
- **React 19 + Vite 6 + TanStack:** Analytics SPA with type-safe routing, server state management, embedded in Go binary via go:embed -- modern, fast, avoids Next.js SSR complexity
- **templ + htmx 2.0:** Public/marketing pages server-rendered in Go -- keeps entire stack in Go, type-safe templates, minimal JS
- **Claude API (Anthropic SDK):** Text-to-SQL and agentic analysis -- official Go SDK, tool use support, 67-86% SQL accuracy with schema grounding
- **Helm 4 + Kubernetes:** Deployment orchestration -- umbrella chart pattern, HPA for gateway, StatefulSets for infrastructure
- **OpenTelemetry SDK:** Observability instrumentation -- vendor-neutral, export to Prometheus+Grafana, essential before adding features

### Expected Features

Research confirms table stakes for behavioral analytics and identifies the LLM differentiator.

**Must have (table stakes):**
- Mobile SDK (iOS+Android), Web SDK (JS/TS), Go backend SDK -- data collection mechanisms, users expect SDKs for all major platforms
- Event segmentation/trends, funnel analysis, retention analysis -- the "big three" chart types that every product analytics platform offers
- SQL editor with Trino access -- power users (engineers) expect raw query capability
- API key authentication (SDKs), session auth (web app), basic RBAC (admin/editor/viewer) -- zero auth currently exists
- Metric alerts with Slack/email/webhook delivery -- reaction engine backend exists, needs UI and notification channels
- Dashboard builder to save and compose charts

**Should have (competitive differentiators):**
- LLM-powered text-to-SQL queries -- "How many users signed up last week from iOS?" generates and executes Trino SQL, explains results in plain English
- Conversational follow-ups -- "Now break that down by platform" modifies previous query based on context
- Agentic multi-step analysis (v2) -- LLM autonomously runs multiple queries, investigates anomalies, composes reports
- Self-hosted with data sovereignty -- Parquet+S3, no vendor lock-in, Trino SQL access unlimited (competitors charge for SQL access)
- gomobile SDK architecture -- unique in the analytics space, single implementation for two platforms

**Defer (v2+):**
- Session replay, heatmaps, feature flags, A/B testing platform -- different product categories, massive scope creep
- Auto-capture (track everything) -- generates noise and data quality problems, explicit instrumentation via protobuf is better
- User paths/flows (Sankey diagrams) -- complex visualization, high effort, medium value
- Agentic LLM features -- requires stable text-to-SQL foundation first

### Architecture Approach

The architecture extends the existing event-driven pipeline (NATS hub with fan-out to warehouse sink and reaction engine) by adding three new layers: client SDKs feeding into the existing HTTP server, a unified API gateway serving both the React analytics SPA and templ+htmx public pages (single Go binary with path-based routing), and a separate LLM service for text-to-SQL (isolated due to unpredictable latency).

**Major components:**
1. **API Gateway (expanded cmd/server)** -- unified HTTP entry point serving event ingestion, analytics BFF API, LLM proxy, React SPA (go:embed), and templ+htmx pages
2. **React Analytics App** -- SPA with dashboards (trends, funnels, retention), SQL editor (CodeMirror 6), LLM natural language queries, rule management UI
3. **LLM Analysis Service (new cmd/llm-service)** -- text-to-SQL with schema introspection, query validation, result explanation, separate process for isolation and scaling
4. **gomobile SDK (new sdk/mobile/)** -- thin JSON bridge API exported via gomobile bind, internal Go batching, persistent queue (SQLite), device context auto-collection
5. **Web SDK (new sdk/web/)** -- TypeScript npm package, fetch-based transport, IndexedDB persistence, automatic page view tracking
6. **Observability Layer (new)** -- OpenTelemetry instrumentation in all services, Prometheus metrics, Grafana dashboards, essential before adding load

**Key patterns to follow:**
- **Unified gateway with path-based routing:** single Go binary handles all HTTP traffic (API, SPA, public pages), go:embed for React bundle
- **Backend-for-Frontend (BFF):** analytics API shaped for React app needs, not generic query API
- **LLM as isolated service:** prevents slow LLM calls (1-60s) from blocking event ingestion or dashboard queries
- **Schema-aware prompting for text-to-SQL:** inject Trino schema into every LLM prompt, validate generated SQL against schema
- **gomobile bind with JSON bridge:** accept JSON strings instead of Go maps/slices due to gomobile type limitations
- **Persistent event queue in SDKs:** SQLite (mobile) or IndexedDB (web), write before network send, critical for offline reliability

### Critical Pitfalls

Research identified 18 documented pitfalls. Top 5 that cause rewrites or data loss:

1. **Small files problem destroys query performance** -- streaming-to-batch creates thousands of tiny Parquet files, Trino performance degrades from seconds to minutes. Prevention: background compaction job merging small files to 128-256MB, or migrate to Apache Iceberg for native compaction. Must fix before dashboards are built on top.

2. **LLM-generated SQL hallucinates schema elements** -- LLM references non-existent tables/columns or uses wrong Trino dialect, returns plausible but incorrect results. Prevention: always inject exact schema into prompt, validate SQL against schema before execution, show generated SQL to users, use Trino-specific dialect tips. Design-time decision, not retrofit.

3. **LLM creates SQL injection attack surface** -- prompt injection tricks LLM into generating DROP/DELETE or unbounded SELECT. Prevention: read-only Trino user, parse and reject non-SELECT statements, query resource limits, rate limiting, audit logging. Must be architected into LLM feature from day one.

4. **Graceful shutdown data loss under Kubernetes** -- SIGTERM races with batch flush, events ACKed but not written to S3. Prevention: ACK after S3 upload (not before), flush all batches on shutdown, set terminationGracePeriodSeconds to 2x flush interval, add timeout to Stop() to prevent deadlock. Fix in pipeline hardening before adding SDKs.

5. **gomobile type restrictions force awkward SDK API** -- no maps, no slices of structs, no function callbacks, Go 1.22.5+ stack split bug with interfaces. Prevention: design JSON bridge API first (Init/Track/Flush accept JSON strings), keep exported interface minimal, test with exact Go toolchain+NDK versions. Irreversible architecture decision at SDK design time.

## Implications for Roadmap

Based on dependency analysis from ARCHITECTURE.md and feature priorities from FEATURES.md, the roadmap should follow this structure:

### Phase 1: Pipeline Hardening & Observability
**Rationale:** Every subsequent phase depends on zero data loss, auth working, and debuggability. Pipeline gaps (shutdown coordination, deduplication, compaction, authentication) must be fixed before increasing load with SDKs. Observability must exist before adding services that need monitoring.

**Delivers:**
- Graceful shutdown without data loss (ACK-after-write, signal handling)
- Event deduplication with client idempotency keys
- Parquet file compaction strategy or Iceberg migration
- API key authentication for event ingestion
- OpenTelemetry metrics and structured logging
- NATS consumer scaling (pull consumers with worker pools)
- Dead letter queue for failed events
- Indexed rule evaluation (by event type)
- Request body size limits and per-key rate limiting

**Addresses:** Zero test coverage (11%), documented shutdown bugs, no auth, no metrics, single consumer bottleneck

**Avoids:** Pitfalls 1, 4, 6, 9, 11, 13, 18

**Research flag:** Standard observability and stream processing patterns, no phase-specific research needed

### Phase 2: gomobile SDK (iOS + Android)
**Rationale:** Without SDKs, events come from curl scripts. Mobile SDK is highest leverage (covers both platforms from one codebase). Must come after auth (needs API keys) and observability (needs to track SDK-specific metrics). Can be built in parallel with web app since it feeds into existing pipeline.

**Delivers:**
- gomobile bind library (.xcframework for iOS, .aar for Android)
- JSON bridge API (Init, Track, Flush, SetUser, Reset)
- Persistent event queue (SQLite)
- Batch flushing (size-based + time-based)
- Offline retry with exponential backoff
- Device context auto-collection (OS, version, model, locale)
- Async queue architecture (non-blocking Track())
- Native Swift and Kotlin wrapper libraries

**Uses:** Go 1.24+, gomobile, SQLite (via gomobile), existing event proto definitions

**Implements:** SDK architecture component from ARCHITECTURE.md

**Avoids:** Pitfalls 5 (type restrictions), 8 (UI thread blocking), 14 (offline event loss)

**Research flag:** Standard patterns, but gomobile has known compatibility issues. Needs version-specific testing (Go toolchain + NDK). No deep research phase needed, but allocate testing time for gomobile build issues.

### Phase 3: Analytics Web Application (React SPA + Auth)
**Rationale:** Establishes the UI container for all analysis features. Requires auth from Phase 1. Proves the web app can query Trino and render results before building complex features (funnels, LLM). BFF API patterns established here are reused in Phase 4.

**Delivers:**
- API Gateway expansion (analytics BFF routes, session auth middleware, React SPA serving via go:embed)
- JWT session authentication (login, access token, refresh token)
- Basic RBAC (admin, editor, viewer roles)
- React app shell with routing (TanStack Router)
- Event segmentation / trends view (time-series charts with Recharts)
- Property filters and breakdowns
- Date range picker (relative + absolute)
- Real-time event stream (SSE)
- SQL editor (CodeMirror 6 with lang-sql)
- Query history and saved queries (PostgreSQL)
- Dashboard builder (save/compose charts)

**Uses:** React 19, Vite 6, TanStack Router/Query, Recharts 3, shadcn/ui, CodeMirror 6, Tailwind 4, golang-jwt/jwt/v5, Trino Go client

**Implements:** Gateway, React app, analytics API components from ARCHITECTURE.md

**Avoids:** Pitfall 10 (over-rendering -- use React Query, memoization, virtualization), Pitfall 16 (unbounded queries -- auto-LIMIT, resource groups)

**Research flag:** No phase-specific research needed. Standard React + Go patterns. CodeMirror SQL autocomplete may need schema integration research during implementation.

### Phase 4: LLM Text-to-SQL Analysis
**Rationale:** The primary differentiator. Requires the analytics API patterns from Phase 3 (Trino query execution, result formatting). React app must exist to provide the UI. Builds on proven text-to-SQL architecture (schema injection, validation, explanation).

**Delivers:**
- LLM service (new cmd/llm-service with internal HTTP API)
- Schema introspection and caching (INFORMATION_SCHEMA queries)
- Prompt construction with schema context and Trino dialect tips
- Claude API integration (Anthropic SDK v1.20.0+)
- SQL generation, validation, and safety checks (read-only user, query parsing)
- Query execution via Trino Go client
- Result explanation (LLM explains findings in plain English)
- Chart type suggestion (line, bar, funnel based on query)
- Natural language query UI in React app (AskAI page)
- Conversation history (PostgreSQL)
- LLM request logging and audit trail

**Uses:** anthropic-sdk-go, Trino Go client, PostgreSQL for conversation state, React app from Phase 3

**Implements:** LLM service component and text-to-SQL flow from ARCHITECTURE.md

**Avoids:** Pitfall 2 (schema hallucination -- schema injection, validation), Pitfall 3 (SQL injection -- read-only user, query parsing, resource limits)

**Research flag:** NEEDS RESEARCH. Text-to-SQL prompting strategies for Trino are domain-specific. Phase-specific research needed for: optimal prompt templates, schema representation format, few-shot examples, self-correction patterns, Trino dialect quirks. Allocate 2-3 days for /gsd:research-phase before implementation.

### Phase 5: Web SDK (JavaScript/TypeScript)
**Rationale:** After mobile SDK proves the pattern and web app exists to consume the data. Less urgent than mobile for initial use case. Can reuse event definitions and ingestion API from Phase 2.

**Delivers:**
- TypeScript npm package
- Dual build (ESM + UMD for CDN script tag)
- fetch-based transport with retry
- IndexedDB/localStorage persistent queue
- Automatic page view tracking
- requestIdleCallback batching
- Browser context auto-collection (user agent, screen size, timezone)

**Uses:** TypeScript 5.7+, Vite 6 for build, vitest for testing

**Implements:** Web SDK component from ARCHITECTURE.md

**Avoids:** gomobile WASM (5-10MB binary), framework dependencies

**Research flag:** Standard web SDK patterns, no research needed

### Phase 6: templ + htmx Public/Marketing Pages
**Rationale:** Public-facing layer is lower priority than analytics functionality. Can be built anytime after gateway exists. templ+htmx keeps the public layer in Go.

**Delivers:**
- templ templates for landing, docs, pricing pages
- htmx interactivity without JS framework
- Static asset serving (CSS, images)
- Routing coexistence with React SPA (templ owns /, /docs; React owns /app/*)

**Uses:** templ v0.3.x, htmx 2.0.x

**Implements:** Public pages component from ARCHITECTURE.md

**Avoids:** Pitfall 17 (routing conflict -- strict URL namespace, SPA fallback handler)

**Research flag:** Standard patterns, no research needed

### Phase 7: Advanced Analytics Views
**Rationale:** After event segmentation proves the query-to-visualization pattern, add the complex chart types that depend on multi-step SQL (funnels, retention) or specialized rendering (user paths).

**Delivers:**
- Funnel analysis (multi-step builder, conversion calculation, drop-off viz)
- Retention analysis (cohort-based grids, N-day/week/month return rates)
- User paths / flows (Sankey diagrams showing navigation sequences)
- User/identity resolution (anonymous-to-known, device-to-user mapping)
- User profiles and user-level analytics

**Uses:** Recharts (or Visx for custom Sankey), PostgreSQL schema extensions for identity graph

**Implements:** Advanced dashboard views from FEATURES.md

**Avoids:** Building these before simpler event segmentation is proven

**Research flag:** NEEDS RESEARCH for retention cohort SQL patterns and identity resolution algorithms. Standard funnel patterns, but identity stitching across devices is complex. Allocate 1-2 days for /gsd:research-phase.

### Phase 8: Kubernetes Deployment
**Rationale:** Requires all services to exist and be stable. K8s configuration informed by actual resource usage observed during Phases 1-7. Should not block feature development (Docker Compose works for early testing).

**Delivers:**
- Helm 4 umbrella chart
- Subcharts for gateway, warehouse-sink, reaction-engine, llm-service
- Dependency charts (NATS, PostgreSQL, MinIO, Trino)
- Proper liveness/readiness/startup probes
- Resource limits based on profiling (GOMEMLIMIT set)
- HPA for gateway (CPU + request rate)
- Secrets management (API keys, LLM provider key, DB credentials)
- Ingress with TLS
- CI/CD with image tagging (git SHA, not :latest)

**Uses:** Helm 4, Kubernetes 1.28+, official NATS/MinIO/Trino charts

**Implements:** K8s deployment architecture from ARCHITECTURE.md

**Avoids:** Pitfall 7 (liveness probe cascading failure), Pitfall 12 (resource limits wrong), Pitfall 15 (:latest tags)

**Research flag:** NATS HA setup and Trino coordinator+worker topology are K8s-specific. May need 1 day research for StatefulSet configuration and resource group tuning.

### Phase 9: Agentic LLM (Evolution)
**Rationale:** Requires stable text-to-SQL (Phase 4) and production data for testing. Evolves from single-query text-to-SQL to multi-step agentic analysis.

**Delivers:**
- Tool use with Claude tool calling API
- Multi-step analysis ("investigate why retention dropped" triggers funnel + cohort + breakdown queries)
- Agentic loop (think, act, observe, repeat)
- Autonomous anomaly investigation (when alert fires, LLM analyzes root cause)
- Report generation from multi-query analysis

**Uses:** Claude tool use, existing LLM service from Phase 4

**Implements:** Agentic text-to-SQL flow from ARCHITECTURE.md

**Avoids:** Building agentic behavior before basic text-to-SQL works reliably

**Research flag:** NEEDS RESEARCH. Agentic architectures for analytics are cutting-edge. Allocate 2-3 days for /gsd:research-phase on tool orchestration patterns, failure recovery, and result synthesis.

### Phase Ordering Rationale

- **Pipeline first:** SDKs, web app, and LLM all depend on zero data loss, auth, and observability. Pipeline hardening is the foundation.
- **SDKs in parallel with web app:** SDKs feed into existing pipeline (can start after Phase 1). Web app queries existing data (can start after Phase 1). These can overlap.
- **LLM after web app:** Text-to-SQL needs the Trino query execution and result rendering patterns established in the web app. Natural language UI lives in the React app.
- **K8s near the end:** Docker Compose supports development. K8s is for production scale, needs all services to exist first.
- **Agentic last:** Requires stable text-to-SQL foundation and production usage patterns.

### Research Flags

Phases needing deeper research during planning:
- **Phase 4 (LLM Text-to-SQL):** Prompt engineering for Trino, schema representation, self-correction patterns -- 2-3 days
- **Phase 7 (Advanced Analytics):** Identity resolution algorithms, retention cohort SQL -- 1-2 days
- **Phase 8 (Kubernetes):** NATS HA, Trino resource groups, StatefulSet configuration -- 1 day
- **Phase 9 (Agentic LLM):** Tool orchestration, multi-step analysis patterns -- 2-3 days

Phases with standard patterns (skip research-phase):
- **Phase 1 (Pipeline Hardening):** Well-documented stream processing patterns
- **Phase 2 (gomobile SDK):** Standard mobile SDK patterns (with gomobile quirks documented)
- **Phase 3 (Web App):** Standard React + Go patterns
- **Phase 5 (Web SDK):** Standard browser SDK patterns
- **Phase 6 (templ + htmx):** Standard server-rendered patterns

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All technologies verified with official docs, active maintenance confirmed, version compatibility checked. gomobile risks are documented. |
| Features | HIGH | Based on comprehensive survey of Amplitude, Mixpanel, PostHog, Plausible. Table stakes vs. differentiators clearly distinguished. |
| Architecture | HIGH | Existing codebase verified (.planning/codebase/ARCHITECTURE.md), new components grounded in official docs (Trino Go client, templ, NATS K8s). |
| Pitfalls | HIGH | Pipeline/K8s/SDK pitfalls grounded in codebase analysis (CONCERNS.md) + official docs. LLM/frontend pitfalls based on ecosystem research and academic papers. |

**Overall confidence:** HIGH

### Gaps to Address

Research was comprehensive, but several areas need validation during implementation:

- **gomobile build compatibility:** Go 1.24.12 + NDK version compatibility must be tested empirically. gomobile has history of breaking changes (golang/go#52470, #68760). Allocate 1-2 days at SDK phase start for build environment setup and testing.

- **Text-to-SQL accuracy on Causality schema:** Benchmarks show 67-86% accuracy on standard datasets, but actual accuracy depends on schema complexity and prompt quality. Needs iterative refinement with real queries. Plan for prompt tuning iteration during Phase 4.

- **Parquet compaction vs. Iceberg migration:** Research recommends compaction job as immediate fix, Iceberg as long-term solution. Decision should be data-driven: measure actual file counts and query latency in Phase 1, then choose strategy.

- **Trino resource groups for mixed workload:** Dashboard queries vs. ad-hoc SQL editor queries need different resource allocations. Trino resource group configuration is cluster-specific. Needs tuning during Phase 3 with real query patterns.

- **LLM cost at scale:** Claude API pricing is per-token. Text-to-SQL with schema context is 2-5K tokens per request. At 100 queries/day, cost is manageable. At 10K queries/day, needs caching strategy or local LLM fallback. Monitor during Phase 4 rollout.

## Sources

### Primary (HIGH confidence)
- **Existing codebase:** `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/CONCERNS.md`, `docs/architecture.md`, source code analysis
- **Official documentation:** Go gomobile (pkg.go.dev), Trino Go client (github.com/trinodb/trino-go-client), NATS JetStream (docs.nats.io), Helm 4 (helm.sh), OpenTelemetry (opentelemetry.io), Anthropic SDK (github.com/anthropics/anthropic-sdk-go), React/Vite/TanStack (official sites), templ (templ.guide)
- **Verified package versions:** npm registry, pkg.go.dev, GitHub releases for all recommended libraries

### Secondary (MEDIUM confidence)
- **Competitor analysis:** Amplitude docs (amplitude.com/docs), Mixpanel docs (docs.mixpanel.com), PostHog docs (posthog.com/docs) -- feature comparison and UX patterns
- **Text-to-SQL benchmarks:** research.aimultiple.com, academic papers (arXiv 2503.05445, 2308.01990) -- accuracy metrics and security research
- **Analytics architecture patterns:** AWS/Google Cloud blogs on text-to-SQL, RisingWave/Upsolver blogs on stream processing, Wren AI architecture (getwren.ai/oss)
- **Mobile SDK design patterns:** Segment, Amplitude, Mixpanel SDK documentation (API design, offline queue, batching)
- **Kubernetes best practices:** kubernetes.io/blog, CNCF blog (OpenTelemetry, monitoring stacks)

### Tertiary (LOW confidence, needs validation)
- **PostHog self-hosted limitations:** PostHog blog claims cloud is recommended above 100K events/month -- may be biased toward their cloud offering
- **gomobile SDK viability for analytics:** No known precedent of gomobile for analytics SDK at scale -- Causality-specific bet, needs production validation
- **Agentic text-to-SQL effectiveness:** Academic papers show promise, but production deployments are rare -- Phase 9 is exploratory

---
*Research completed: 2026-02-05*
*Ready for roadmap: yes*
