# Causality

## What This Is

Causality is a self-hosted behavioral analytics platform — Amplitude on mega steroids. It provides end-to-end ownership of the entire analytics stack: from mobile/web/backend SDKs that collect events, through a streaming pipeline that stores and processes them, to a custom web application where teams analyze behavior through dashboards, natural language queries powered by LLMs, and raw SQL. Built in Go, deployed on Kubernetes.

## Core Value

Anyone on the team can understand what users are doing in any app — by asking a question in plain English, browsing a dashboard, or writing SQL — without depending on any third-party analytics vendor.

## Requirements

### Validated

- ✓ HTTP event ingestion (single + batch endpoints) — existing
- ✓ Protocol Buffer event definitions (26+ event types: screenView, buttonTap, userLogin, purchaseComplete, etc.) — existing
- ✓ NATS JetStream event streaming with reliable delivery — existing
- ✓ Parquet batching with configurable size/time triggers — existing
- ✓ S3 storage with Hive-style partitioning (app_id/year/month/day/hour) — existing
- ✓ Trino SQL querying over Parquet data — existing
- ✓ Rule engine with JSONPath conditions and webhook actions — existing
- ✓ Anomaly detection (threshold, rate, count-based) — existing
- ✓ Webhook delivery with HMAC signing and exponential backoff retry — existing
- ✓ Multi-stage Docker builds (server, warehouse-sink, reaction-engine) — existing
- ✓ Docker Compose development environment — existing

### Active

- [ ] gomobile SDK for iOS and Android (single Go codebase compiled via gomobile)
- [ ] JavaScript/TypeScript web SDK for browser apps
- [ ] Go backend SDK for server-side event collection
- [ ] Custom web application (React + TypeScript) with three analysis modes: dashboards, natural language, SQL editor
- [ ] Pre-built dashboard views — funnels, retention curves, event flows, real-time activity
- [ ] LLM-powered natural language analysis — text-to-SQL evolving into agentic multi-step reasoning
- [ ] SQL editor for power users with schema browser and query history
- [ ] Public/marketing pages with templ + htmx served from Go
- [ ] API authentication (API keys for SDKs, session auth for web app)
- [ ] Kubernetes deployment manifests and Helm charts
- [ ] Harden pipeline — comprehensive test coverage, graceful shutdown, error handling, backpressure
- [ ] Observability — Prometheus metrics, structured logging, health dashboards
- [ ] Event deduplication and validation
- [ ] Rule and alert management UI in web app

### Out of Scope

- Multi-tenant SaaS hosting — internal tool first, external offering is a future concern
- Native iOS/Android SDKs (Swift/Kotlin) — gomobile provides cross-platform from Go
- Real-time streaming analytics (sub-second) — batch analytics with reasonable freshness is sufficient
- Custom visualization builder — dashboards are pre-built; SQL + LLM cover exploratory needs
- Data export/ETL — own the data in S3/Parquet, query it directly
- Mobile app for viewing analytics — web app only

## Context

**Existing codebase:** The Go backend has a working event pipeline (HTTP → NATS → Parquet → S3 → Trino) and a reaction engine (rules + anomaly detection + webhooks). However, this was produced in a brainstorming session and has significant gaps: 11% test coverage, no authentication, no observability, fragile shutdown paths, silent failures, and several known performance bottlenecks (see `.planning/codebase/CONCERNS.md`).

**The SDKs don't exist yet.** Currently events are sent via curl/test scripts. The gomobile SDK is the highest-leverage SDK — one Go codebase targeting both mobile platforms.

**The web application doesn't exist yet.** Redash is in the Docker Compose stack but is a placeholder. The vision is a custom-built analytics experience with LLM as a first-class analysis method.

**The LLM analysis is the differentiator.** No self-hosted analytics platform offers "ask your data anything" as a core feature. This starts as text-to-SQL (LLM generates Trino queries, executes, explains results) and evolves into an agentic system that can do multi-step analysis — drill down, compare time periods, identify anomalies, chart results.

**Target users:** Internal team first. Multiple apps and services instrumented with Causality SDKs. Team members range from engineers (who want SQL) to product people (who want dashboards and natural language).

## Constraints

- **Tech Stack (backend)**: Go 1.24+ — established, all new backend code continues in Go
- **Tech Stack (frontend)**: React + TypeScript for the analytics SPA; templ + htmx from Go servers for public/static pages
- **Tech Stack (mobile SDKs)**: gomobile — single Go codebase for iOS and Android
- **Streaming**: NATS JetStream — established, proven in current architecture
- **Storage**: MinIO/S3 + Parquet — established, enables Trino querying
- **Query Engine**: Trino — required for SQL analytics and LLM text-to-SQL
- **Deployment**: Kubernetes — all services containerized with K8s manifests
- **Protocol**: Protocol Buffers — established for event definitions and API contracts
- **Database**: PostgreSQL — established for reaction engine state, extend for web app state

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| gomobile for mobile SDKs | Single Go codebase for both platforms; team already in Go | — Pending |
| React+TS for analytics app | Rich interactivity needed for dashboards/charts/SQL editor; large ecosystem | — Pending |
| templ+htmx for public pages | SEO-friendly, server-rendered, keeps Go as the primary language | — Pending |
| LLM analysis in v1 | Core differentiator; without it, this is just another self-hosted analytics tool | — Pending |
| Kubernetes from day one | Deferred — Docker Compose sufficient for internal use; K8s adds premature complexity | — Deferred to v2.0 |
| Text-to-SQL as LLM starting point | Simpler than full agent; Trino SQL is well-documented; can iterate toward agentic later | — Pending |
| Go backend SDK bundled with Phase 1 | Thin HTTP client, simplest SDK — natural fit with pipeline hardening phase | — Pending |
| Full v1.0 scope (6 phases) | Pipeline hardening → gomobile SDK → Web app → LLM → Web SDK → Public pages | — Scoped |

## Milestones

### v1.0 — Complete Analytics Platform with LLM Differentiator
**Status:** Requirements defined, roadmap created, not started
**Phases:** 6 (see `.planning/ROADMAP.md`)
**Scope:** Pipeline hardening + Go SDK, gomobile SDK, React analytics web app, LLM text-to-SQL, web SDK, templ+htmx public pages

### v2.0 — Advanced Analytics & Production Deployment (future)
**Scope:** Funnel/retention/user paths, Kubernetes + Helm, agentic LLM

---
*Last updated: 2026-02-05 after requirements and roadmap definition*
