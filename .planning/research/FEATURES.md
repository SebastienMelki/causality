# Feature Landscape

**Domain:** Self-hosted behavioral analytics platform
**Researched:** 2026-02-05
**Overall confidence:** HIGH (based on comprehensive survey of Amplitude, Mixpanel, PostHog, Plausible, and broader ecosystem)

## What Causality Already Has

Before mapping what to build, here is what the existing codebase provides:

| Capability | Status | Location |
|------------|--------|----------|
| HTTP event ingestion (single + batch) | Working | `internal/gateway/` |
| 26+ typed event definitions (protobuf) | Working | `proto/causality/v1/events.proto` |
| NATS JetStream streaming | Working | `internal/nats/` |
| Parquet batching and S3 upload | Working | `internal/warehouse/` |
| Hive-style partitioning (app_id/year/month/day/hour) | Working | `internal/warehouse/consumer.go` |
| Trino SQL querying | Working (infra) | `docker/trino/` |
| Rule engine (JSONPath conditions, webhook actions) | Working | `internal/reaction/engine.go` |
| Anomaly detection (threshold, rate, count) | Working | `internal/reaction/anomaly.go` |
| Webhook delivery (HMAC, exponential backoff) | Working | `internal/reaction/dispatcher.go` |
| Docker Compose dev environment | Working | `docker-compose.yml` |

**Key gap:** No SDKs, no web application, no dashboards, no LLM analysis, no authentication. Events are sent via curl/scripts.

---

## Table Stakes

Features users expect from any behavioral analytics platform. Missing any of these means the product feels incomplete or unusable for the target audience (engineers + product people analyzing app behavior).

### 1. Client SDKs

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Mobile SDK (iOS + Android)** | Cannot collect mobile app events without one; the primary data source | High | Planned as gomobile (single Go codebase). Must handle offline queuing, batching, retry, device context auto-collection. |
| **Web/Browser SDK (JS/TS)** | Web apps are a primary instrumentation target | Medium | Must handle page lifecycle, batching, retry, automatic page view tracking. |
| **Backend SDK (Go)** | Server-side events (API calls, background jobs, auth events) need collection | Low | Thin client -- just HTTP POST with batching. Go is the team's language. |
| **Offline event queuing** | Mobile devices lose connectivity; events must not be lost | Medium | SDK must persist events to disk and flush when connectivity returns. Every serious mobile SDK does this. |
| **Automatic context collection** | Device model, OS version, app version, locale, timezone -- users expect this without manual setup | Low | Already defined in DeviceContext proto. SDKs must auto-populate. |
| **Event batching in SDK** | Sending individual HTTP requests per event would kill battery and bandwidth | Medium | Configurable batch size + flush interval. Standard practice. |
| **Session tracking** | Users expect session-level grouping (session start, duration, events per session) | Medium | SDK generates session IDs, detects session boundaries (background > 30s = new session). |

**Confidence:** HIGH. Every competitor (Amplitude, Mixpanel, PostHog) ships SDKs for all major platforms. This is literally the data collection mechanism.

### 2. Core Analysis Views (Dashboards)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Event segmentation / trends** | "How many times did X happen over time?" -- the most basic analytics question | Medium | Time-series charts with event counts, broken down by properties. Amplitude's most-used chart type. |
| **Funnel analysis** | "What percentage of users who did A also did B then C?" -- conversion measurement | High | Multi-step ordered event sequences with conversion rates, drop-off visualization, configurable windows. Amplitude, Mixpanel, PostHog all have this. |
| **Retention analysis** | "Of users who signed up in week 1, how many came back in week 2, 3, ...?" -- cohort retention curves | High | Cohort-based retention grids showing N-day/week/month return rates. Standard in Amplitude and Mixpanel. |
| **User paths / flows** | "What do users do after screen X?" -- navigation flow visualization | High | Sankey-style diagrams showing actual user paths through the app. Amplitude calls this Pathfinder, PostHog calls it Paths. |
| **Real-time event stream** | "What is happening right now?" -- live activity view | Low | Live tail of incoming events. Simple but gives immediate confidence that instrumentation is working. |
| **Dashboard builder** | Users need to save charts and compose them into dashboards for daily monitoring | Medium | Save individual charts, arrange them on a dashboard, set time range globally. |

**Confidence:** HIGH. Funnel, retention, and event segmentation are the big three that every product analytics platform offers. Missing any one of them makes Causality feel like a toy.

### 3. Authentication and Access Control

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **API key authentication for SDKs** | SDKs need a credential to send events; must identify which app is sending | Low | API keys per app_id. Already implicit in the event model (app_id field). |
| **Session authentication for web app** | Users log into the analytics web app | Medium | Email/password or SSO. JWT or session cookies. |
| **Basic RBAC** | Admins vs. viewers at minimum; control who can modify rules/alerts vs. who can only view dashboards | Medium | Roles: admin, editor, viewer. Not complex -- internal tool, small team. |

**Confidence:** HIGH. No analytics platform ships without auth. Currently zero auth exists in Causality.

### 4. Data Exploration

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **SQL editor** | Power users (engineers) want raw SQL access to the data | Medium | Monaco-based SQL editor with autocomplete, schema browser, query history, results table. Queries go to Trino. |
| **Property filters and breakdowns** | Every chart must support "filter by X" and "break down by Y" | Medium | Filter events by any property (device, OS, app version, custom params). Break down into separate lines/bars. |
| **Date range picker** | Universal time range selection across all views | Low | Relative ranges (last 7 days, last 30 days) + absolute date picker. Applied globally. |
| **Export results** | Download query results as CSV/JSON | Low | Standard feature. Users need to get data out for reports, slides, further analysis. |

**Confidence:** HIGH. These are fundamental interactions that underpin all analytics work.

### 5. Alerting and Monitoring

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Metric alerts** | "Alert me when daily active users drops below X" or "when error rate exceeds Y" | Medium | Already partially built in reaction engine (threshold, rate, count). Needs UI for configuration + notification channels. |
| **Notification channels** | Alerts must reach people -- Slack, email, webhook | Medium | Webhook already exists. Add Slack integration and email. Slack is the most important channel for internal teams. |
| **Alert management UI** | Configure, enable/disable, view history of alerts from the web app | Medium | CRUD interface for the existing rule engine and anomaly detection configs. |

**Confidence:** HIGH. Mixpanel and Amplitude both offer alerts. PostHog has alerts. The reaction engine already handles the backend; the gap is the UI and notification delivery.

---

## Differentiators

Features that set Causality apart from competitors. These are not expected but create significant competitive advantage.

### 1. LLM-Powered Natural Language Analysis (Primary Differentiator)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Text-to-SQL queries** | "How many users signed up last week from iOS?" -- anyone on the team can query data without knowing SQL | High | LLM generates Trino SQL from natural language. Schema-aware prompting. Self-correcting (if query fails, retry with error context). Mixpanel has Spark AI, PostHog has Max AI -- but neither is available self-hosted. |
| **Result explanation** | LLM explains what the query found in plain English, with key takeaways | Medium | After executing the SQL, pass results back to LLM for narrative summary. |
| **Conversational follow-ups** | "Now break that down by platform" or "Compare to the previous month" | High | Maintain conversation context. LLM modifies the previous query based on follow-up. This is where it becomes agentic. |
| **Agentic multi-step analysis** | LLM autonomously runs multiple queries, drills into anomalies, composes a report | Very High | Multi-step reasoning: "Investigate why retention dropped this week" triggers funnel check, cohort comparison, property breakdown. This is the mega steroids differentiator. |
| **Chart generation from NL** | LLM not only runs SQL but also selects the right visualization (line chart, bar chart, funnel, retention grid) | High | LLM outputs both the query and the chart type/configuration. The frontend renders it. |

**Confidence:** MEDIUM (for the technical approach). Text-to-SQL accuracy with Trino is achievable (current LLMs get 67-86% accuracy on standard benchmarks; with schema context and self-correction, higher). The agentic layer is harder and less proven. Start with text-to-SQL, iterate toward agentic.

**Why this is THE differentiator:** No self-hosted analytics platform offers LLM-powered analysis. Mixpanel Spark AI and PostHog Max AI are cloud-only. By offering this self-hosted, Causality fills a gap nobody else occupies.

### 2. Full Stack Ownership

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Self-hosted, no vendor dependency** | Complete data sovereignty; no data leaves your infrastructure | Low (already true) | PostHog moved away from self-hosted for serious workloads (they recommend cloud above 100K events/month). Causality leans INTO self-hosting. |
| **Single vendor for SDKs + pipeline + analytics** | No gluing together Segment + Amplitude + custom dashboards | Low (architectural) | The value is in the integration -- one team owns everything from the SDK to the dashboard. |
| **Data in open formats (Parquet + S3)** | No vendor lock-in; data is always accessible via standard tools | Low (already true) | Already using Parquet on S3. Any tool that reads Parquet (DuckDB, Spark, pandas) can access the data. |

**Confidence:** HIGH. This is an architectural choice already made. The competitive advantage is real.

### 3. Developer-First Experience

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **gomobile SDK (single Go codebase for iOS + Android)** | Unique approach: one implementation, two platforms. Team already writes Go. | High | No competitor does this. It is a risk (gomobile has limitations) but also a differentiator if it works well. |
| **Protobuf-first event schema** | Type-safe event definitions, auto-generated code, schema evolution | Low (already done) | Most competitors use untyped JSON. Protobuf gives compile-time safety in the SDK. |
| **Direct Trino SQL access** | Engineers can bypass the UI entirely and query the data lake | Low (already available) | Power move for technical teams. Amplitude and Mixpanel charge extra for SQL access. |
| **API-first design** | Every UI action maps to an API call; enables scripting and automation | Medium | Not yet built, but the architecture supports it. |

**Confidence:** HIGH. These are architectural decisions that naturally flow from the current design.

### 4. Integrated Anomaly Detection

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Real-time rule evaluation** | Already built. Events are evaluated against rules as they stream through NATS. | Low (exists) | Competitors offer anomaly detection as a premium feature or separate product. Causality has it built into the pipeline. |
| **LLM-powered anomaly investigation** | When an anomaly fires, the LLM agent can automatically investigate | Very High | This is a future differentiator. Combines alerting with agentic analysis. |

**Confidence:** MEDIUM for the LLM investigation layer. HIGH for the rule engine itself (already working).

---

## Anti-Features

Features to explicitly NOT build. These are common in competitors but wrong for Causality's position.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Session replay / screen recording** | Massive data volume, complex mobile SDKs, privacy nightmare, completely different technical domain. PostHog built an entire team around this. | Focus on behavioral event analysis. If users need session replay, they can use a dedicated tool (FullStory, Hotjar) alongside Causality. |
| **Heatmaps** | Requires DOM/view hierarchy capture, rendering engine, and is a UX research tool, not a behavioral analytics tool. Different audience. | Causality tracks events (what happened), not visual interaction patterns (where on screen). |
| **Feature flags / experimentation** | Building a feature flag system is a separate product (LaunchDarkly, Statsig, Unleash). Bundling it dilutes focus. | Integrate with existing feature flag tools via event properties (e.g., flag variant as an event property). |
| **A/B testing platform** | Statistical significance engines, variant assignment, experiment management -- this is its own domain. | Users can analyze experiment results via SQL/LLM queries if variants are included in event properties. |
| **Surveys / in-app messaging** | Different product category entirely. PostHog bundles it; it makes their product sprawling. | Out of scope. Use dedicated tools. |
| **Customer Data Platform (CDP)** | Syncing data to 40+ destinations is a massive integration effort. Segment, Hightouch, and Census own this space. | Data is in Parquet/S3 -- users can build their own ETL if needed. Open format means no lock-in. |
| **Auto-capture (track everything automatically)** | Generates massive noise, creates data governance problems, and makes the data lake unreliable. Heap and PostHog do this; teams often regret the data quality. | Explicit instrumentation via typed protobuf events. Quality over quantity. The tracking plan is the schema. |
| **Multi-tenant SaaS platform** | Hosting for external customers requires billing, isolation, compliance, support -- completely different business. | Internal tool first. Multi-tenancy is a future concern if Causality becomes a product. |
| **Custom visualization builder** | Drag-and-drop chart builder is a huge UI project (Redash, Metabase territory). Diminishing returns for an internal tool. | Pre-built dashboard views (funnel, retention, trends) + SQL editor + LLM queries cover 95% of needs. |
| **Native iOS/Android SDKs (Swift/Kotlin)** | Maintaining two native SDKs doubles the work for the SDK team. The whole point of gomobile is avoiding this. | gomobile SDK provides cross-platform coverage from Go. |
| **Real-time streaming analytics** | Sub-second analytics requires a completely different architecture (Flink, Kafka Streams). The batch pipeline (Parquet + Trino) is optimized for analytical queries, not real-time. | Near-real-time (minutes of freshness) via small batch intervals. Live event stream for immediate debugging. Reaction engine handles truly real-time alerting via NATS. |

---

## Feature Dependencies

Features build on each other. This dependency graph informs phase ordering.

```
SDK (mobile/web/backend)
  |
  v
Event Ingestion (already exists) --> Authentication (API keys)
  |
  v
Data Pipeline (already exists)
  |
  +---> Web Application Shell
  |       |
  |       +---> Session Auth + RBAC
  |       |
  |       +---> Event Segmentation / Trends view
  |       |       |
  |       |       +---> Property filters and breakdowns
  |       |       |
  |       |       +---> Funnel Analysis view
  |       |       |       |
  |       |       |       +---> Retention Analysis view
  |       |       |       |
  |       |       |       +---> User Paths view
  |       |       |
  |       |       +---> Dashboard Builder (save + compose charts)
  |       |
  |       +---> SQL Editor
  |       |       |
  |       |       +---> Text-to-SQL (LLM)
  |       |               |
  |       |               +---> Conversational follow-ups
  |       |               |
  |       |               +---> Agentic multi-step analysis
  |       |
  |       +---> Alert Management UI
  |               |
  |               +---> Slack/email notification channels
  |
  +---> User/Identity Resolution (anonymous -> known)
          |
          +---> User profiles + user-level analytics
```

**Key insight:** The web application is the critical path. Without it, none of the analysis features are usable by non-engineers. The SQL editor and LLM analysis both depend on the web app existing.

**The SDKs can be built in parallel** with the web application since they feed data into the already-working pipeline.

---

## MVP Recommendation

For MVP, prioritize:

1. **gomobile SDK (iOS + Android)** -- Without data flowing in from real apps, everything else is theoretical. This is the highest-leverage SDK since it covers both mobile platforms.
2. **Web application shell** with authentication -- The container for all analysis features. Session auth, basic RBAC, navigation.
3. **Event segmentation / trends view** -- The simplest and most-used chart type. Proves the web app can query Trino and render results.
4. **SQL editor** -- Unblocks power users immediately with minimal frontend complexity (Monaco editor + results table).
5. **Text-to-SQL (LLM)** -- The differentiator. Even a basic version demonstrates the vision.
6. **API key authentication** -- SDKs need credentials. Simple per-app API keys.

Defer to post-MVP:

- **Funnel analysis**: Complex UI (multi-step builder, conversion calculation logic). Build after event segmentation proves the data pipeline end-to-end.
- **Retention analysis**: Complex cohort calculation. Needs user identification working well first.
- **User paths**: Sankey diagram rendering is specialized. High effort, medium value.
- **Dashboard builder**: Requires multiple chart types to exist first. Save/compose comes after individual views work.
- **Web SDK (JS/TS)**: Less urgent than mobile SDK for the initial use case. Can be done after gomobile SDK proves the pattern.
- **Agentic LLM analysis**: Requires text-to-SQL to be working reliably first. Iterate toward agentic behavior after basic NL queries work.
- **Notification channels (Slack/email)**: Webhook delivery already works. Slack integration is a nice-to-have for v1.
- **User/identity resolution**: Requires schema changes (user_id mapping, device-to-user graph). Important for retention analysis but not for MVP.

---

## Competitive Positioning Matrix

How Causality compares to competitors on key dimensions:

| Dimension | Amplitude | Mixpanel | PostHog | Causality |
|-----------|-----------|----------|---------|-----------|
| Self-hosted | No | No | Discouraged at scale | Yes (core value) |
| LLM analysis | Limited | Spark AI (cloud) | Max AI (cloud) | First-class, self-hosted |
| SDKs | 10+ platforms | 10+ platforms | 10+ platforms | 3 (Go mobile, JS, Go backend) |
| Funnel analysis | Excellent | Excellent | Good | To build |
| Retention analysis | Excellent | Excellent | Good | To build |
| Session replay | Yes | Yes | Yes | No (anti-feature) |
| Feature flags | Yes | Yes | Yes | No (anti-feature) |
| Data format | Proprietary | Proprietary | ClickHouse | Open (Parquet + S3) |
| SQL access | Paid tier | Paid tier | HogQL | Trino (unlimited) |
| Pricing | Per event (expensive) | Per event | Per event (open core) | Free (self-hosted) |
| Target user | Product teams | Product teams | Engineers | Engineers + Product |

---

## Sources

Research based on the following (confidence levels noted):

**HIGH confidence (official documentation, multiple sources):**
- Amplitude: https://amplitude.com/amplitude-analytics, https://amplitude.com/docs/analytics/charts
- Mixpanel: https://mixpanel.com/features/, https://mixpanel.com/spark-ai/, https://docs.mixpanel.com/docs/features/alerts
- PostHog: https://posthog.com/product-analytics, https://posthog.com/max, https://github.com/PostHog/posthog
- Plausible: https://plausible.io/

**MEDIUM confidence (multiple credible sources):**
- Text-to-SQL accuracy benchmarks: https://research.aimultiple.com/text-to-sql/
- Agentic text-to-SQL: https://aws.amazon.com/blogs/machine-learning/build-a-robust-text-to-sql-solution-generating-complex-queries-self-correcting-and-querying-diverse-data-sources/
- Identity resolution: https://docs.mixpanel.com/docs/tracking-methods/id-management/identifying-users-simplified
- SDK design patterns: https://uxcam.com/blog/top-analytics-sdks/

**LOW confidence (single source, unverified):**
- PostHog self-hosted limitations at scale (their own blog -- may be biased toward cloud)
- gomobile SDK viability for analytics (no known precedent; Causality-specific bet)
