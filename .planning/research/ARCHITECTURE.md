# Architecture Patterns

**Domain:** Self-hosted behavioral analytics platform
**Researched:** 2026-02-05
**Confidence:** HIGH (existing codebase verified, new components grounded in official docs and ecosystem research)

## Existing Architecture (As-Is)

The current system is a well-structured event streaming pipeline with three independent Go microservices sharing a NATS JetStream event bus.

```
Mobile/Web (curl)
      |
      v
┌─────────────┐
│ HTTP Server  │ :8080  (cmd/server)
│  + Gateway   │
└──────┬──────┘
       │ publish
       v
┌─────────────┐
│    NATS      │ JetStream
│  (EVENTS)   │
└──────┬──────┘
       │ fan-out to 2 consumers
       ├──────────────────────────────┐
       v                              v
┌──────────────┐              ┌───────────────┐
│ Warehouse    │              │  Reaction     │
│ Sink         │              │  Engine       │
└──────┬───────┘              └───────┬───────┘
       │                              │
       v                              v
┌──────────────┐              ┌───────────────┐
│ MinIO (S3)   │              │  PostgreSQL   │
│ Parquet      │              │  + Webhooks   │
└──────┬───────┘              └───────────────┘
       │
       v
┌──────────────┐
│ Hive Meta    │
│ store        │
└──────┬───────┘
       │
       v
┌──────────────┐
│   Trino      │ SQL engine
└──────┬───────┘
       │
       v
┌──────────────┐
│   Redash     │ (placeholder, to be replaced)
└──────────────┘
```

**Key characteristics of existing architecture:**
- Event-driven with NATS JetStream as the central hub
- All services are stateless Go binaries with graceful shutdown
- Protocol Buffers for type-safe event definitions (26+ event types)
- Hive-style partitioning (`app_id/year/month/day/hour`) for query performance
- PostgreSQL stores reaction engine state (rules, webhooks, anomaly configs)
- No authentication, no observability, 11% test coverage

## Recommended Architecture (To-Be)

### Full System Diagram

```
                    ┌─────────────────────────────────────────────────┐
                    │              Public Surface                     │
                    │                                                 │
 ┌────────────┐    │  ┌──────────────────────────────────────────┐   │
 │ iOS SDK    │────┼──│                                          │   │
 │ (gomobile) │    │  │          API Gateway / Server            │   │
 └────────────┘    │  │              (Go)                        │   │
                   │  │                                          │   │
 ┌────────────┐    │  │  /api/v1/events/*    Event ingestion     │   │
 │ Android SDK│────┼──│  /api/v1/analytics/* Analytics API (BFF) │   │
 │ (gomobile) │    │  │  /api/v1/llm/*       LLM analysis API   │   │
 └────────────┘    │  │  /api/v1/admin/*     Rules/config CRUD   │   │
                   │  │  /app/*              React SPA (embed)   │   │
 ┌────────────┐    │  │  /*                  templ+htmx pages    │   │
 │ Web SDK    │────┼──│                                          │   │
 │ (JS/TS)    │    │  └──────┬───────────┬──────────┬───────────┘   │
 └────────────┘    │         │           │          │               │
                   │         │           │          │               │
 ┌────────────┐    │         │           │          │               │
 │ Go SDK     │────┼─────────┘           │          │               │
 │ (backend)  │    │                     │          │               │
 └────────────┘    └─────────────────────┼──────────┼───────────────┘
                                         │          │
                   ┌─────────────────────┘          │
                   │                                │
                   v                                v
            ┌─────────────┐               ┌──────────────────┐
            │    NATS      │               │  LLM Analysis    │
            │  JetStream   │               │  Service (Go)    │
            └──────┬──────┘               └────────┬─────────┘
                   │                               │
        ┌──────────┴──────────┐                    │
        v                     v                    v
 ┌──────────────┐     ┌───────────────┐    ┌──────────────┐
 │ Warehouse    │     │  Reaction     │    │   Trino      │
 │ Sink         │     │  Engine       │    │              │
 └──────┬───────┘     └───────┬───────┘    └──────┬───────┘
        │                     │                    │
        v                     v                    v
 ┌──────────────┐     ┌───────────────┐    ┌──────────────┐
 │ MinIO (S3)   │     │  PostgreSQL   │    │ Hive Meta    │
 │ Parquet      │     │              │    │ store        │
 └──────────────┘     └───────────────┘    └──────────────┘
        │
        └──────────> Trino queries Parquet via Hive
```

### Component Boundaries

| Component | Responsibility | Talks To | New/Existing |
|-----------|---------------|----------|--------------|
| **API Gateway (Go)** | Unified HTTP entry point. Event ingestion, analytics BFF API, auth, static file serving | NATS, Trino, PostgreSQL, LLM Service | **Modified** (expand existing `cmd/server`) |
| **React Analytics App** | SPA for dashboards, SQL editor, NL queries, rule management | API Gateway (HTTP) | **New** |
| **templ+htmx Pages** | Public/marketing/docs pages, server-rendered | Rendered by API Gateway | **New** |
| **LLM Analysis Service** | Text-to-SQL, schema introspection, query explanation, agentic analysis | Trino, LLM Provider (OpenAI/Ollama), PostgreSQL | **New** (`cmd/llm-service`) |
| **Warehouse Sink** | NATS consumer, Parquet batching, S3 upload | NATS, MinIO | **Existing** |
| **Reaction Engine** | Rule evaluation, anomaly detection, webhook dispatch | NATS, PostgreSQL | **Existing** |
| **iOS/Android SDK** | Event collection, batching, offline queue, retry | API Gateway (HTTP) | **New** (`sdk/mobile/`) |
| **Web SDK (JS/TS)** | Browser event collection, auto-tracking | API Gateway (HTTP) | **New** (`sdk/web/`) |
| **Go Backend SDK** | Server-side event collection | API Gateway (HTTP) | **New** (`sdk/go/`) |
| **NATS JetStream** | Event streaming hub | All event producers/consumers | **Existing** |
| **MinIO / S3** | Parquet file storage | Warehouse Sink, Trino | **Existing** |
| **Trino** | SQL query engine over Parquet | LLM Service, API Gateway (for analytics queries) | **Existing** |
| **PostgreSQL** | State store (rules, webhooks, sessions, query history, LLM conversations) | Reaction Engine, API Gateway, LLM Service | **Existing** (new schemas) |
| **Observability Stack** | Metrics, logging, tracing | All services (emit), Prometheus/Grafana (collect) | **New** |

## Detailed Component Architecture

### 1. API Gateway / Server (Modified)

**Current:** HTTP server with event ingestion endpoints only (`/v1/events/ingest`, `/v1/events/batch`).

**Expanded to serve as the unified entry point:**

```
API Gateway (single Go binary)
├── /api/v1/events/ingest       [existing] Event ingestion
├── /api/v1/events/batch        [existing] Batch ingestion
├── /api/v1/analytics/          [new] BFF for React app
│   ├── /dashboards             Dashboard data queries
│   ├── /queries                Execute/save SQL queries
│   ├── /funnels                Funnel analysis
│   ├── /retention              Retention curves
│   └── /events/stream          SSE for live event feed
├── /api/v1/llm/                [new] LLM analysis proxy
│   ├── /ask                    Natural language query
│   ├── /conversations          Conversation history
│   └── /explain                Explain query results
├── /api/v1/admin/              [new] Config management
│   ├── /rules                  Rule CRUD
│   ├── /webhooks               Webhook CRUD
│   ├── /anomalies              Anomaly config CRUD
│   └── /apps                   App key management
├── /app/*                      [new] Serve React SPA
├── /*                          [new] templ+htmx public pages
├── /health                     [existing] Health check
└── /ready                      [existing] Readiness check
```

**Why a single gateway (not separate services):**
- Reduces deployment complexity: one binary handles all HTTP traffic
- The `cmd/server` already has middleware (CORS, rate limiting, logging) that all routes need
- Auth middleware applies uniformly across all endpoints
- Static file serving (React SPA via `go:embed`, templ pages) is trivial in Go
- BFF pattern: the analytics API is tightly coupled to the React app's needs, not a general-purpose API

**Recommendation:** Expand `cmd/server` into `cmd/gateway` that encompasses all HTTP concerns. Alternatively, keep `cmd/server` as the name and grow it. Do NOT create separate HTTP servers for different frontends.

### 2. React Analytics App (New)

**Architecture:** Single-page application embedded in the Go binary via `go:embed`.

```
web/app/                        React+TS source
├── src/
│   ├── pages/
│   │   ├── Dashboard.tsx       Pre-built dashboard views
│   │   ├── SQLEditor.tsx       SQL query editor
│   │   ├── AskAI.tsx           Natural language interface
│   │   ├── EventStream.tsx     Real-time event feed
│   │   ├── RulesManager.tsx    Rule/alert management
│   │   └── Settings.tsx        App config
│   ├── components/
│   │   ├── charts/             Recharts/D3 chart components
│   │   ├── query/              SQL editor components
│   │   └── common/             Shared UI components
│   ├── hooks/                  React hooks for API calls
│   ├── api/                    Typed API client (generated from OpenAPI)
│   └── store/                  State management (Zustand or React Query)
├── dist/                       Build output (go:embed target)
├── package.json
└── tsconfig.json
```

**Key decisions:**
- **Build output embedded in Go binary** using `//go:embed web/app/dist/*`. This eliminates the need for a separate web server, CDN, or NGINX. The Go gateway serves the SPA directly.
- **Client-side routing with catch-all**: The Go server handles SPA routing by serving `index.html` for any `/app/*` path that does not match a static file.
- **API communication**: All API calls go to the same origin (the gateway), eliminating CORS complexity.
- **No SSR**: The analytics app does not need SEO. Pure client-side rendering is sufficient.

### 3. templ+htmx Public Pages (New)

**Architecture:** Server-side rendered pages using Go's templ templating engine and htmx for interactivity.

```
web/public/                     templ source
├── templates/
│   ├── layout.templ            Base layout with nav, footer
│   ├── home.templ              Landing page
│   ├── docs.templ              Documentation pages
│   ├── pricing.templ           Pricing (if applicable)
│   └── components/
│       ├── header.templ
│       └── footer.templ
├── static/
│   ├── css/
│   └── images/
```

**Routing strategy (coexistence with React SPA):**
```
/ (root)                        templ+htmx: landing page
/docs/*                         templ+htmx: documentation
/app/*                          React SPA: analytics application
/api/*                          Go handlers: API endpoints
```

The Go router dispatches based on path prefix. templ handlers own `/` and top-level marketing paths. The React SPA owns `/app/*`. API routes own `/api/*`. This is clean separation with no routing conflicts.

### 4. LLM Analysis Service (New)

**Architecture:** Separate Go service that handles all LLM interactions.

```
cmd/llm-service/
├── main.go                     Service entry point
internal/llm/
├── service.go                  Core service orchestrating LLM operations
├── schema.go                   Trino schema introspection and caching
├── prompt.go                   Prompt templates and construction
├── trino.go                    Trino query execution via Go client
├── conversation.go             Conversation state management
├── agent.go                    Agentic multi-step orchestration (v2)
└── providers/
    ├── provider.go             LLM provider interface
    ├── openai.go               OpenAI API client
    └── ollama.go               Ollama local LLM client
```

**Why a separate service (not embedded in gateway):**
- LLM calls are long-running (seconds to minutes for agentic flows). Isolating them prevents slow requests from affecting event ingestion or dashboard API latency.
- Different scaling characteristics: LLM service may need GPU nodes or specific hardware. Event ingestion needs low-latency CPU nodes.
- Independent deployment: LLM prompts and models change frequently; redeploy without touching the data pipeline.
- Different failure modes: LLM provider outages should not affect event collection.

**Communication with gateway:** Internal HTTP/gRPC API. The gateway proxies `/api/v1/llm/*` requests to the LLM service. In Kubernetes, this is service-to-service communication within the cluster.

**Text-to-SQL flow (v1):**
```
User question
    │
    v
[1. Schema Retrieval]
    Query Trino INFORMATION_SCHEMA for table/column metadata
    Cache schema with TTL (refresh every N minutes)
    │
    v
[2. Prompt Construction]
    System prompt: "You are an analytics SQL expert..."
    Schema context: Table definitions, column descriptions, sample values
    User question: "How many users logged in this week?"
    Few-shot examples: Common query patterns for this domain
    │
    v
[3. LLM Call]
    Send prompt to OpenAI/Ollama
    Parse SQL from response
    │
    v
[4. SQL Validation]
    Syntax check (basic parse)
    Table/column existence verification
    Safety check (no DDL, no mutations)
    │
    v
[5. Query Execution]
    Execute SQL against Trino via Go client (trinodb/trino-go-client)
    Set query timeout (e.g., 30 seconds)
    │
    v
[6. Result Formatting]
    Format results as table/chart data
    Generate natural language explanation
    Return to user via gateway
```

**Agentic flow (v2, future):**
```
User question
    │
    v
[Agent Loop]
    ├── Think: What do I need to find out?
    ├── Act: Generate SQL, execute query
    ├── Observe: Analyze results
    ├── Think: Do I need to drill deeper?
    ├── Act: Generate follow-up SQL
    ├── Observe: Combine findings
    └── Respond: Synthesize multi-step analysis
```

**Trino client integration:** Use the official `trinodb/trino-go-client` package which implements Go's `database/sql/driver` interface. DSN format: `http://user@trino:8080?catalog=hive&schema=causality`. This gives standard `sql.DB` semantics (connection pooling, timeouts, context cancellation).

### 5. SDK Architecture

#### gomobile SDK (iOS + Android)

```
sdk/mobile/
├── causality.go                Public API surface (gomobile bind exports)
├── client.go                   HTTP client with retry/backoff
├── queue.go                    Persistent event queue (SQLite)
├── batch.go                    Event batching logic
├── config.go                   SDK configuration
├── device.go                   Device info collection
└── internal/
    ├── storage/                SQLite-backed persistent queue
    └── network/                Reachability detection
```

**Key architectural decisions:**
- **gomobile bind** generates `.xcframework` (iOS) and `.aar` (Android). Native apps call Go functions through generated Objective-C/Swift and Java/Kotlin bindings.
- **Exposed API surface must use gomobile-compatible types**: `string`, `int`, `float64`, `bool`, `[]byte`, and structs containing only these types. No maps, no slices of non-primitive types, no channels.
- **Persistent queue**: Events are written to SQLite (available on both platforms) before sending. On network failure, events accumulate and are flushed when connectivity returns.
- **Batching**: Timer-based and size-based batching. Accumulate events and send as batch every N seconds or when batch reaches M events.
- **Device context**: Collected once at SDK init (platform, OS version, device model, screen size). Attached to every event envelope.

**Type restriction pattern:**
```go
// gomobile-compatible: exported struct with simple types
type Event struct {
    AppID      string
    DeviceID   string
    EventType  string
    EventName  string
    PayloadJSON string  // JSON string instead of map[string]interface{}
}

// SDK public API
func Init(config string) error          // JSON config
func TrackScreen(screenName string)     // Convenience method
func TrackEvent(eventJSON string)       // Generic event
func Flush()                            // Force send
func SetUserID(userID string)           // Set user context
```

#### Web SDK (JavaScript/TypeScript)

```
sdk/web/
├── src/
│   ├── index.ts                Entry point, Causality class
│   ├── client.ts               HTTP client with retry
│   ├── queue.ts                localStorage/IndexedDB queue
│   ├── batch.ts                Event batching
│   ├── auto-track.ts           Automatic page view, click tracking
│   ├── types.ts                TypeScript type definitions
│   └── utils/
│       ├── device.ts           Browser/device detection
│       └── storage.ts          Persistent storage abstraction
├── package.json
├── tsconfig.json
└── rollup.config.js            Bundle for CDN + npm
```

**Distribution:** npm package + CDN script tag. Dual build: ESM for bundlers, UMD/IIFE for script tag inclusion.

#### Go Backend SDK

```
sdk/go/
├── causality.go                Client struct, public API
├── options.go                  Functional options pattern
├── batch.go                    In-memory batching with flush
├── transport.go                HTTP transport with retry
└── causality_test.go           Tests
```

**Distribution:** Standard Go module. `go get github.com/SebastienMelki/causality/sdk/go`

### 6. Observability Layer (New)

**Architecture:** OpenTelemetry instrumentation in all Go services, exporting to Prometheus (metrics) and optionally Jaeger/Tempo (traces).

```
All Go Services
    │
    ├── [Metrics] ──> Prometheus ──> Grafana
    │   - events_ingested_total
    │   - events_processed_total
    │   - batch_flush_duration_seconds
    │   - trino_query_duration_seconds
    │   - llm_request_duration_seconds
    │   - nats_messages_pending
    │   - http_request_duration_seconds
    │
    ├── [Logs] ──> stdout (JSON) ──> Loki ──> Grafana
    │   - Already using slog with JSON format
    │   - Add trace_id, span_id fields
    │
    └── [Traces] ──> OTel Collector ──> Jaeger/Tempo ──> Grafana
        - HTTP request traces
        - NATS publish/consume traces
        - Trino query traces
        - LLM API call traces
```

**Recommendation:** Use OpenTelemetry SDK for Go (`go.opentelemetry.io/otel`) as the instrumentation layer. Export metrics in Prometheus format (each service exposes `/metrics`). This is the industry standard and avoids vendor lock-in.

### 7. Kubernetes Deployment Architecture

```
Kubernetes Cluster
├── Namespace: causality
│   ├── Deployment: gateway              (2+ replicas)
│   ├── Deployment: warehouse-sink       (2+ replicas, consumer group)
│   ├── Deployment: reaction-engine      (2+ replicas, consumer group)
│   ├── Deployment: llm-service          (1-2 replicas)
│   ├── StatefulSet: nats                (3 replicas, JetStream cluster)
│   ├── StatefulSet: postgresql           (1 primary + read replica)
│   ├── StatefulSet: minio               (4 nodes distributed)
│   ├── Deployment: trino-coordinator    (1 replica)
│   ├── Deployment: trino-worker         (2+ replicas)
│   ├── StatefulSet: hive-metastore      (1 replica)
│   ├── ConfigMap: trino-config
│   ├── ConfigMap: hive-config
│   ├── Secret: db-credentials
│   ├── Secret: s3-credentials
│   ├── Secret: llm-api-key
│   ├── Ingress: causality-ingress       (gateway + docs)
│   └── Service: internal services       (ClusterIP)
│
├── Namespace: monitoring
│   ├── Deployment: prometheus
│   ├── Deployment: grafana
│   ├── DaemonSet: promtail/fluentbit    (log collection)
│   └── Deployment: loki                  (log storage)
```

**Helm chart structure:**
```
deploy/helm/causality/
├── Chart.yaml
├── values.yaml
├── templates/
│   ├── gateway-deployment.yaml
│   ├── warehouse-sink-deployment.yaml
│   ├── reaction-engine-deployment.yaml
│   ├── llm-service-deployment.yaml
│   ├── ingress.yaml
│   ├── services.yaml
│   ├── configmaps.yaml
│   └── secrets.yaml
├── charts/                              # Subcharts for dependencies
│   ├── nats/                            # Official NATS Helm chart
│   ├── postgresql/                      # Bitnami PostgreSQL chart
│   ├── minio/                           # Official MinIO chart
│   └── trino/                           # Official Trino chart
```

**Key K8s decisions:**
- **NATS:** Use official NATS Helm chart with JetStream enabled, 3-node StatefulSet for HA.
- **MinIO:** Use official MinIO operator/chart with distributed mode (minimum 4 nodes for erasure coding).
- **Trino:** Coordinator + worker topology. Coordinator is a single pod; workers scale horizontally.
- **PostgreSQL:** Single instance with persistent volume is sufficient initially. Add read replica when needed.
- **Gateway:** Deployment with HPA (horizontal pod autoscaler) based on CPU/request rate. Behind Ingress with TLS.
- **Custom services (warehouse-sink, reaction-engine, llm-service):** Deployments with resource limits. Not exposed via Ingress (internal only, except LLM service proxied through gateway).

## Data Flow (Complete)

### Event Ingestion Flow
```
SDK (mobile/web/backend)
  │
  │ POST /api/v1/events/ingest (or /batch)
  │ Header: X-API-Key: <key>
  │ Body: { event: EventEnvelope }
  v
Gateway (auth middleware validates API key)
  │
  │ Publish to NATS stream "CAUSALITY_EVENTS"
  │ Subject: events.<app_id>.<category>.<type>
  v
NATS JetStream ─────────────────────────────┐
  │                                          │
  │ Consumer: "warehouse-sink"               │ Consumer: "analysis-engine"
  v                                          v
Warehouse Sink                         Reaction Engine
  │ Batch events (size/time trigger)    │ Evaluate rules
  │ Write Parquet                       │ Detect anomalies
  │ Upload to S3                        │ Queue webhooks
  v                                     v
MinIO (S3)                             PostgreSQL (deliveries)
  │                                     │
  │ Hive Metastore catalogs schema     │ Dispatcher sends webhooks
  v                                     v
Trino (queries Parquet)                External Systems
```

### Analytics Query Flow
```
React App (browser)
  │
  │ GET /api/v1/analytics/dashboards
  v
Gateway (session auth validates user)
  │
  │ Query Trino via Go client
  │ (trinodb/trino-go-client, database/sql)
  v
Trino
  │
  │ Reads Parquet from S3 via Hive connector
  v
MinIO (S3)
  │
  │ Returns query results
  v
Gateway formats response
  │
  │ JSON response
  v
React App renders dashboard
```

### LLM Analysis Flow
```
React App (browser)
  │
  │ POST /api/v1/llm/ask
  │ Body: { question: "How many users logged in this week?" }
  v
Gateway (session auth validates user)
  │
  │ Proxy to LLM Service (internal HTTP)
  v
LLM Service
  │
  ├── [1] Get cached Trino schema
  │       (INFORMATION_SCHEMA or cached metadata)
  │
  ├── [2] Construct prompt with schema + question
  │
  ├── [3] Call LLM provider (OpenAI API / Ollama)
  │       Receive generated SQL
  │
  ├── [4] Validate SQL (parse, check tables/columns, safety)
  │
  ├── [5] Execute SQL against Trino
  │       (trinodb/trino-go-client)
  │
  ├── [6] Format results + generate explanation
  │
  │ Return { sql, results, explanation, chart_suggestion }
  v
Gateway returns to React App
  │
  v
React App renders results with chart
```

## Patterns to Follow

### Pattern 1: Unified Gateway with Path-Based Routing
**What:** A single Go HTTP server handles all external HTTP traffic, dispatching to different handlers based on URL prefix.
**When:** When you have multiple frontend concerns (SPA, server-rendered pages, API) but want single-binary deployment simplicity.
**Why:** Go's `http.ServeMux` supports pattern-based routing natively. Adding middleware (auth, logging, CORS) applies uniformly. `go:embed` enables the React SPA to live inside the same binary.
**Example:**
```go
mux := http.NewServeMux()

// API routes
mux.Handle("/api/v1/events/", eventHandler)
mux.Handle("/api/v1/analytics/", analyticsHandler)
mux.Handle("/api/v1/llm/", llmProxy)
mux.Handle("/api/v1/admin/", adminHandler)

// React SPA (catch-all for /app/*)
mux.Handle("/app/", spaHandler(embeddedFS))

// templ+htmx pages (everything else)
mux.Handle("/", publicHandler)
```

### Pattern 2: Backend-for-Frontend (BFF)
**What:** The analytics API is designed specifically for the React app's data needs, not as a generic query API.
**When:** The frontend needs pre-aggregated data, specific response shapes, and server-side query optimization.
**Why:** Prevents the frontend from needing to understand Trino SQL or raw data structures. The gateway can cache common dashboard queries, pre-compute aggregations, and return exactly what each React component needs.

### Pattern 3: LLM as Isolated Service
**What:** LLM operations run in a separate process/service from the main gateway.
**When:** LLM calls are unpredictable in latency (1-60+ seconds) and may require specialized hardware.
**Why:** Prevents slow LLM operations from blocking event ingestion or dashboard API responses. Allows independent scaling and deployment of LLM capabilities.

### Pattern 4: Schema-Aware Prompting for Text-to-SQL
**What:** The LLM service maintains a cached representation of the Trino schema (tables, columns, types, descriptions) and injects it into every prompt.
**When:** Building text-to-SQL that needs to generate valid SQL for a specific database schema.
**Why:** Without schema context, LLMs hallucinate table and column names. With accurate schema, accuracy improves dramatically. The Causality events table is relatively simple (one main table with clear column descriptions), which is favorable for text-to-SQL accuracy.

### Pattern 5: gomobile bind with JSON Bridge
**What:** The gomobile SDK uses JSON strings for complex data structures instead of native Go types.
**When:** gomobile's type system is limited (no maps, no slices of structs, no generics).
**Why:** `map[string]interface{}` cannot cross the gomobile boundary. Instead, accept `string` parameters containing JSON and deserialize inside Go. This is a well-known pattern in gomobile library development.

### Pattern 6: Persistent Event Queue in SDKs
**What:** All SDKs write events to a persistent local queue before attempting network transmission.
**When:** Mobile and web clients operate in unreliable network conditions.
**Why:** Events must not be lost when the device is offline, the app is killed, or the network is intermittent. SQLite (mobile) or IndexedDB (web) provides durability. Events are dequeued and sent when connectivity is available.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Separate HTTP Server per Frontend Concern
**What:** Creating separate Go binaries for the API, the SPA serving, the templ pages, and the LLM proxy.
**Why bad:** Multiplies deployment complexity, requires separate health checks, duplicates middleware, and complicates auth token sharing. In Kubernetes, this means 4+ deployments instead of 1 for what is logically a single HTTP surface.
**Instead:** Use a single gateway binary with path-based routing. The LLM service is the only exception because of its fundamentally different latency and resource characteristics.

### Anti-Pattern 2: React App Querying Trino Directly
**What:** Having the React frontend call Trino's REST API directly or through a thin proxy.
**Why bad:** Exposes query engine to the internet. No query validation, no access control on what SQL can be executed, no result caching. The Trino REST API is not designed for browser clients.
**Instead:** All Trino queries go through the gateway's analytics API, which validates, authorizes, and optimizes queries.

### Anti-Pattern 3: Embedding LLM Logic in the Gateway
**What:** Running LLM API calls, prompt construction, and agentic loops inside the gateway process.
**Why bad:** A single slow LLM request (30+ seconds) ties up a gateway goroutine. Under load, this can exhaust connection pools and affect event ingestion latency. LLM provider outages would degrade the entire gateway.
**Instead:** Separate LLM service with the gateway acting as a proxy. The gateway can enforce timeouts and circuit-break if the LLM service is slow.

### Anti-Pattern 4: gomobile Full App (instead of bind)
**What:** Using `gomobile build` to create full Go mobile applications.
**Why bad:** No access to native UI frameworks (UIKit, Jetpack Compose). The app feels foreign on both platforms. Limited access to platform APIs (push notifications, deep links, background modes).
**Instead:** Use `gomobile bind` to create a library. Native apps use their platform's UI framework and call Go for business logic (event collection, batching, networking).

### Anti-Pattern 5: Direct PostgreSQL Access from Multiple Services for Shared State
**What:** Having the gateway, LLM service, and reaction engine all directly read/write the same PostgreSQL tables.
**Why bad:** Schema changes require coordinating multiple services. No clear ownership of data. Risk of conflicting writes.
**Instead:** Each service owns specific tables. The gateway owns `sessions`, `query_history`, `api_keys`. The reaction engine owns `rules`, `webhooks`, `deliveries`, `anomaly_*`. The LLM service owns `conversations`, `llm_queries`. Cross-service data access goes through APIs.

## Scalability Considerations

| Concern | At 100 users | At 10K users | At 1M events/day |
|---------|--------------|--------------|-------------------|
| **Event ingestion** | Single gateway replica | 2-3 gateway replicas with HPA | 5+ replicas, consider dedicated ingestion path |
| **Analytics queries** | Trino single-node | Trino coordinator + 2 workers | Trino + 4+ workers, query result caching |
| **LLM queries** | Single LLM service | LLM service with queue/rate limiting | Multiple LLM services, request priority queue |
| **Storage** | Single MinIO | MinIO distributed (4+ nodes) | MinIO cluster or migrate to cloud S3 |
| **NATS** | Single NATS | 3-node JetStream cluster | NATS super-cluster, partition by app_id |
| **PostgreSQL** | Single instance | Connection pooling (pgBouncer) | Read replicas, partition large tables |

## Suggested Build Order (Dependencies)

The build order is driven by dependency chains between components.

### Phase 1: Foundation (Must Come First)
**Components:** Authentication, observability, pipeline hardening
**Why first:** Every subsequent component depends on auth (SDKs need API keys, React app needs sessions). Observability must exist before adding new services. Pipeline hardening fixes known fragile areas before adding more load.
**Dependency:** Nothing depends on new features; these are infrastructure.

### Phase 2: Analytics API + React App
**Components:** Gateway analytics API (BFF), React SPA with dashboards, SQL editor
**Why second:** This replaces Redash (the placeholder). Requires auth from Phase 1. Establishes the patterns for how frontends consume data from Trino.
**Dependency:** Requires auth middleware, Trino client integration, PostgreSQL schema for query history.

### Phase 3: LLM Analysis Service
**Components:** LLM service, text-to-SQL, schema introspection, NL query UI in React app
**Why third:** The differentiating feature. Requires the analytics API patterns from Phase 2 (query execution, result formatting). The React app must exist to provide the UI.
**Dependency:** Requires Trino client (Phase 2), React app (Phase 2), LLM provider config.

### Phase 4: SDKs
**Components:** gomobile SDK, Web SDK, Go backend SDK
**Why fourth:** SDKs consume the event ingestion API which already exists. Auth (API keys from Phase 1) is required. SDKs benefit from observability (tracking SDK-specific metrics).
**Dependency:** Requires auth (API keys), event ingestion (existing), observability (Phase 1).

### Phase 5: Kubernetes Deployment
**Components:** Helm charts, K8s manifests, CI/CD
**Why fifth:** Requires all services to exist and be stable. K8s configuration is informed by actual resource requirements observed during Phases 1-4.
**Dependency:** All services must be containerized and tested.

### Phase 6: Agentic LLM (Evolution)
**Components:** Multi-step analysis, agent loop, tool use
**Why last:** Evolves the text-to-SQL from Phase 3. Requires mature schema understanding and query patterns.
**Dependency:** Requires stable text-to-SQL (Phase 3), production data for testing.

## Sources

- [templ documentation: Using React with templ](https://templ.guide/syntax-and-usage/using-react-with-templ/)
- [Trino Go client (official)](https://github.com/trinodb/trino-go-client)
- [NATS Kubernetes Helm Charts](https://github.com/nats-io/k8s)
- [NATS and Kubernetes documentation](https://docs.nats.io/running-a-nats-service/nats-kubernetes)
- [MinIO and Trino Kubernetes deployment](https://blog.min.io/minio-trino-kubernetes/)
- [Wren AI text-to-SQL architecture](https://www.getwren.ai/oss)
- [LLM Text-to-SQL Architecture Patterns](https://github.com/arunpshankar/LLM-Text-to-SQL-Architectures)
- [Text-to-SQL accuracy comparison 2026](https://research.aimultiple.com/text-to-sql/)
- [Trino AI functions documentation](https://trino.io/docs/current/functions/ai.html)
- [gomobile documentation](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)
- [Go mobile wiki](https://go.dev/wiki/Mobile)
- [Embed React in Go binary](https://bindplane.com/blog/embed-react-in-golang)
- [Serve SPA from Go](https://hackandsla.sh/posts/2021-11-06-serve-spa-from-go/)
- [OpenTelemetry unified observability](https://www.cncf.io/blog/2025/11/27/from-chaos-to-clarity-how-opentelemetry-unified-observability-across-clouds/)
- [Kubernetes monitoring stack 2026](https://www.spectrocloud.com/blog/choosing-the-right-kubernetes-monitoring-stack)
- [Self-hosted LLMs for text-to-SQL](https://blog.nilenso.com/blog/2025/05/27/experimenting-with-self-hosted-llms-for-text-to-sql/)
- [Agentic LLM pipelines for text-to-SQL](https://arxiv.org/html/2510.25997)
- Existing codebase: `docs/architecture.md`, `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/CONCERNS.md`

---

*Architecture research: 2026-02-05*
