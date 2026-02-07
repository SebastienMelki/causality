# Technology Stack

**Project:** Causality -- Self-Hosted Behavioral Analytics Platform
**Researched:** 2026-02-05
**Mode:** Ecosystem (subsequent milestone -- additions to existing Go backend)

## Existing Stack (Established -- Not Re-Researched)

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.24.7 (toolchain 1.24.12) | All backend services |
| NATS JetStream | 2.10 | Event streaming |
| Protocol Buffers (proto3) | v1.36.11 | Event definitions, API contracts |
| Parquet | parquet-go v0.25.0 | Columnar storage |
| MinIO/S3 | AWS SDK v2 | Object storage |
| Trino | 444 | SQL query engine |
| PostgreSQL | 16 | Reaction engine state |
| Docker | Multi-stage builds | Container images |

## Recommended New Stack

### 1. Mobile SDKs -- gomobile

| Technology | Version | Purpose | Confidence |
|------------|---------|---------|------------|
| golang.org/x/mobile/cmd/gomobile | latest | Cross-compile Go to iOS/Android | HIGH |
| golang.org/x/mobile/cmd/gobind | latest | Generate Swift/Kotlin bindings | HIGH |

Why gomobile: Single Go codebase for both platforms. The team is Go-focused. Alternative native Swift + Kotlin SDKs would double maintenance.

Critical type restrictions (verified):
- Supported: signed integers, floats, string, bool, byte slice, exported structs, interfaces, error
- NOT supported: slices of structs, maps, channels, unsigned integers (except byte), generics
- SDK public API must use JSON strings or flat primitives. Internally use full Go types.

Architecture: Thin API surface (Init, Track, Flush, SetUser, Reset) accepting JSON strings. Internal Go types for event construction, batching, HTTP posting.

Risk: Small community, no official roadmap. Stable but no new features. Acceptable for library binding use case.

Do NOT use: Flutter/React Native (unnecessary runtime), native Swift/Kotlin (doubles maintenance), cgo approaches (gomobile handles this).

### 2. Web/JS SDK

| Technology | Version | Purpose | Confidence |
|------------|---------|---------|------------|
| TypeScript | ~5.7+ | Type-safe browser SDK | HIGH |
| Vite | 6.x | Build/bundle for npm | HIGH |
| vitest | 3.x | Unit testing | HIGH |

Architecture: npm package with fetch-based transport, automatic batching via requestIdleCallback/visibilitychange. No framework dependencies.

Do NOT use: gomobile WASM (5-10MB binary), Segment (defeats self-hosted purpose).

### 3. Go Backend SDK

| Technology | Version | Purpose | Confidence |
|------------|---------|---------|------------|
| Go standard library | 1.24+ | Server-side event tracking | HIGH |

Thin Go package wrapping HTTP calls. No special dependencies needed beyond net/http, encoding/json, and existing protobuf types.

### 4. React + TypeScript Analytics Web Application

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| React | 19.x | UI framework | Team requirement, ecosystem | HIGH |
| TypeScript | ~5.7+ | Type safety | Industry standard | HIGH |
| Vite | 6.x | Build tooling | 5x faster than webpack, Vite 6 stable | HIGH |
| TanStack Router | 1.x (~1.151) | Routing | Type-safe routes, params, loaders | HIGH |
| TanStack Query | 5.x (~5.90) | Server state | Caching, refetching, suspense | HIGH |
| Recharts | 3.x (~3.7) | Charts | SVG, composable, rewrote state in v3 | HIGH |
| shadcn/ui | latest | UI components | Copy-paste, Radix + Tailwind, 104k stars | HIGH |
| Tailwind CSS | 4.x | CSS | 5x faster builds, CSS-first config | HIGH |
| CodeMirror 6 | lang-sql 6.x | SQL editor | 150KB vs Monaco 3-5MB | MEDIUM |
| @uiw/react-codemirror | 4.x | CM6 React wrapper | Official binding | MEDIUM |
| Zustand | 5.x | Client state | Minimal, works with TanStack Query | MEDIUM |
| Vitest | 3.x | Testing | Vite-native, ES modules | HIGH |
| React Testing Library | latest | Component tests | Standard | HIGH |

Recharts over Visx: 3x less code per chart. Standard chart types with good defaults. Recharts 3 rewrote internals. Add Visx later if custom visualizations needed.

TanStack Router over React Router: Superior TypeScript integration. Compile-time route error checking. Daily releases.

shadcn/ui over MUI/Ant Design: Own the code, no dependency lock-in. Radix primitives. Tailwind styling. MUI is heavy and opinionated.

CodeMirror over Monaco: 1/20th the bundle size. Excellent SQL support via @codemirror/lang-sql. Schema-aware completion.

Do NOT use: Next.js (SSR unnecessary for authenticated SPA), Redux (TanStack Query + Zustand covers it), Monaco (too large), Chart.js (canvas-based), MUI/Ant Design (too heavy).

### 5. templ + htmx -- Public/Marketing Pages

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| templ (a-h/templ) | v0.3.x (v0.3.977) | Go HTML templating | Type-safe, compiled to Go | HIGH |
| htmx | 2.0.x | Hypermedia interactivity | Minimal JS, server-rendered | HIGH |

templ generates type-safe Go code from .templ files. No runtime template parsing. htmx adds interactivity without JS framework. Keeps public-facing layer in Go.

htmx 4.0 is in development (mid-2026) but 2.0 is stable and supported indefinitely. Use 2.0.x.

Do NOT use: Go html/template (no type safety), second React app (over-engineering), Hugo (separate build system).

### 6. LLM-Powered Natural Language Analysis

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| anthropic-sdk-go | v1.20.0+ | Claude API client | Official, tool use, streaming, Go 1.22+ | HIGH |
| Claude Sonnet | API | Text-to-SQL and analysis | Intelligence/speed/cost balance | MEDIUM |

Two-Phase Architecture:

Phase 1 - Text-to-SQL: User question -> schema context + Claude -> Trino SQL -> execute -> Claude explains results -> display in UI.

Phase 2 - Agentic: Claude uses tool calling for multi-step analysis. Execute SQL, request follow-ups, compare periods, generate charts. Go backend as tool executor.

Why Claude: Official Go SDK (v1.20.0), well-maintained, type-safe, native tool use. Claude excels at structured output and complex instructions. Sonnet-class sufficient for text-to-SQL.

Why not local LLM: Commercial models achieve 80-90%+ on text-to-SQL benchmarks. Open models lag substantially. Core differentiator needs best quality.

LLM abstraction: Build internal/llm package with Analyzer interface. Enables provider swapping, mocking, rate limiting, cost tracking, caching.

Schema context: Pre-compute schema description with table names, column types, sample values. Cache it. Include partition column docs for Trino-aware queries.

Do NOT use: LangChain/LangGraph (Python-centric, no mature Go version), Vanna.ai (Python-only), fine-tuned models (over-engineering).

### 7. API Authentication

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| golang-jwt/jwt/v5 | v5.3.0 | JWT tokens | Standard Go JWT, EdDSA/ECDSA/RSA/HMAC | HIGH |
| API Keys (custom) | -- | SDK auth | Stateless, hashed in PostgreSQL | HIGH |
| golang.org/x/crypto/bcrypt | latest | Password hashing | Standard Go extended stdlib | HIGH |

Two auth mechanisms:

1. API Keys for SDKs: X-API-Key header. Keys generated per-app, stored hashed in PostgreSQL. Stateless validation with caching. Matches Amplitude/Mixpanel pattern.

2. JWT sessions for web app: Email/password login. Access token (15 min) + refresh token (7 days, httpOnly cookie). User ID and roles in JWT. Middleware validates on every request.

Do NOT use: dgrijalva/jwt-go (deprecated), Keycloak (over-engineering), OAuth2/OIDC (unnecessary complexity for internal tool).

### 8. Kubernetes Deployment

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Helm | 4.x (4.1.0+) | K8s package management | Released Nov 2025, active branch | HIGH |
| Kubernetes | 1.28+ | Container orchestration | Multi-service architecture | HIGH |
| Kustomize | built-in | Environment overlays | Dev/staging/prod variance | MEDIUM |

Chart structure: Umbrella chart (causality) with sub-charts:
- causality-server (HPA on CPU/request rate)
- warehouse-sink (fixed replicas)
- reaction-engine (fixed replicas)
- analytics-api (new: React API + LLM endpoints)
- web (new: templ+htmx public pages + React static)

Infrastructure (NATS, PostgreSQL, MinIO, Trino) via official Helm charts or managed separately.

Helm 4 over Helm 3: Active development. Helm 3 bug fixes until Jul 2026, security until Nov 2026. Backward-compatible with v3 charts.

Do NOT use: Docker Compose for production (no scaling), Pulumi/Terraform (infrastructure not app deployment), Nomad (smaller ecosystem).

### 9. Observability

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| OpenTelemetry Go SDK | v1.35.0+ | Instrumentation | Vendor-neutral, metrics+traces+logs | HIGH |
| Prometheus | 3.x | Metrics collection | Industry standard, K8s native | HIGH |
| OTel Prometheus Exporter | latest | OTel to Prometheus | Bridge instrumentation to collection | HIGH |
| Grafana | 11.x | Dashboards | Pairs with Prometheus | HIGH |

OTel over raw Prometheus client: Vendor-neutral API. Instrument once, export to Prometheus now, switch providers later. OTel Go SDK stable at v1.35.0.

Key metrics: Event ingestion rate, processing latency (p50/p95/p99), batch flush size, Trino query duration, LLM latency and token usage, HTTP duration by endpoint, error rates.

Do NOT use: Raw Prometheus client (vendor lock-in), Jaeger standalone (tracing-only), ELK stack (heavy).

## Complete Installation Reference

### Go Backend (new dependencies)

```bash
go get github.com/anthropics/anthropic-sdk-go@latest
go get github.com/golang-jwt/jwt/v5@latest
go get golang.org/x/crypto@latest
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/exporters/prometheus@latest
go install github.com/a-h/templ/cmd/templ@latest
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

### React Analytics App

```bash
npm create vite@latest analytics-app -- --template react-ts
npm install react@19 react-dom@19
npm install @tanstack/react-router @tanstack/react-query
npm install recharts@3 zustand tailwindcss@4
npx shadcn@latest init
npm install @uiw/react-codemirror @codemirror/lang-sql
npm install -D typescript@5.7 vite@6 vitest
npm install -D @testing-library/react @testing-library/jest-dom
npm install -D @tanstack/router-devtools @tanstack/react-query-devtools
```

### Web/JS SDK

```bash
npm create vite@latest web-sdk -- --template vanilla-ts
npm install -D typescript@5.7 vite@6 vitest tsup
```

### Kubernetes

```bash
brew install helm
helm create deploy/helm/causality
```

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Mobile SDK | gomobile bind | Native Swift + Kotlin | Doubles maintenance |
| Mobile SDK | gomobile bind | React Native bridge | Unnecessary JS runtime |
| Browser SDK | Custom TS package | Segment analytics.js | Defeats self-hosted purpose |
| Frontend | React 19 + Vite 6 | Next.js | SSR unnecessary for authenticated SPA |
| Router | TanStack Router | React Router v7 | TanStack has superior TS integration |
| Charts | Recharts 3 | Visx (Airbnb) | 3x more code per chart |
| Charts | Recharts 3 | Chart.js | Canvas-based, less React-native |
| UI components | shadcn/ui | MUI | Heavy, opinionated styling |
| SQL editor | CodeMirror 6 | Monaco Editor | Monaco 3-5MB vs CM6 ~150KB |
| Public pages | templ + htmx | Go html/template | No type safety |
| LLM | Anthropic Claude SDK | OpenAI Go SDK | Claude tool use better designed |
| LLM | Anthropic Claude SDK | Local LLM (Ollama) | Quality too low for text-to-SQL |
| LLM framework | Direct SDK | LangChain Go port | No mature Go version |
| Auth | JWT + API keys | OAuth2/OIDC | Over-engineering for internal tool |
| Auth | golang-jwt v5 | dgrijalva/jwt-go | Deprecated, security issues |
| K8s deploy | Helm 4 | Docker Compose | No scaling, no rolling updates |
| Observability | OTel SDK | Raw Prometheus client | Vendor lock-in |

## Version Summary

| Package | Version | Verified |
|---------|---------|----------|
| Go | 1.24+ | YES (existing) |
| gomobile | golang.org/x/mobile@latest | YES |
| anthropic-sdk-go | v1.20.0 | YES (GitHub) |
| golang-jwt/jwt | v5.3.0 | YES (pkg.go.dev) |
| OTel Go SDK | v1.35.0 | YES (GitHub) |
| templ | v0.3.977 | YES (GitHub) |
| htmx | 2.0.x | YES (htmx.org) |
| React | 19.x | YES (npm) |
| Vite | 6.x | YES (vite.dev) |
| TanStack Router | 1.x (~1.151) | YES (npm) |
| TanStack Query | 5.x (~5.90) | YES (npm) |
| Recharts | 3.x (~3.7) | YES (npm) |
| Tailwind CSS | 4.x | YES (tailwindcss.com) |
| CodeMirror lang-sql | 6.x (~6.10) | YES (npm) |
| Helm | 4.x (~4.1) | YES (helm.sh) |
| Prometheus | 3.x | YES |
| Grafana | 11.x | MEDIUM |

---

*Stack research: 2026-02-05*
