# Domain Pitfalls

**Domain:** Self-hosted behavioral analytics platform (Go pipeline, gomobile SDKs, LLM text-to-SQL, React analytics frontend, Kubernetes deployment)
**Researched:** 2026-02-05

---

## Critical Pitfalls

Mistakes that cause rewrites, data loss, or fundamental architecture failures.

---

### Pitfall 1: Small Files Problem Destroys Query Performance

**What goes wrong:** The warehouse sink writes a Parquet file per flush interval per partition. With Hive-style partitioning by `app_id/year/month/day/hour`, a modest deployment (5 apps, 24 hours) creates 120 partitions per day. At a 30-second flush interval, that is 2,880 tiny Parquet files per partition per day. Trino must open, read metadata from, and close each file individually. Query performance degrades from seconds to minutes as file count grows. S3 API costs scale linearly with file count (charged per 1,000 requests).

**Why it happens:** Streaming-to-batch architectures inherently create many small files. The current warehouse sink has no compaction strategy. Developers focus on ingestion throughput and discover the query problem weeks later when dashboards become unusable.

**Consequences:** Dashboard queries that should take 2 seconds take 60+ seconds. Trino worker memory exhausted by file metadata. S3 GET request costs become unexpectedly high. Users lose trust in the analytics platform.

**Prevention:**
- Implement a background compaction job that merges small Parquet files within each partition into larger files (target 128MB-256MB per file)
- Increase batch size and flush interval to reduce file count at the source (trade latency for fewer files)
- Consider migrating from Hive tables to Apache Iceberg table format, which handles compaction natively and supports hidden partitioning
- Add monitoring for file count per partition and average file size

**Detection:** Query latency increasing over time. S3 `ListObjects` calls returning thousands of objects per partition. Trino query plans showing excessive split counts.

**Confidence:** HIGH -- well-documented problem in analytics architectures. Multiple sources confirm this is the primary performance killer for Parquet-on-S3 query engines.

**Phase relevance:** Must be addressed in pipeline hardening phase, before the web app and dashboards are built. If dashboards are built on top of a query layer that degrades over time, users will blame the frontend.

**Sources:**
- [Dealing with Small Files Issues on S3 (Upsolver)](https://www.upsolver.com/blog/small-file-problem-s3)
- [When Small Parquet Files Become a Big Problem](https://www.datobra.com/when-small-parquet-files-become-a-big-problem-and-how-i-ended-up-writing-a-compactor-in-pyarrow/)
- [Compaction Strategies and the Small File Problem (Uplatz)](https://uplatz.com/blog/compaction-strategies-and-the-small-file-problem-in-object-storage-a-comprehensive-analysis-of-query-performance-optimization/)

---

### Pitfall 2: LLM-Generated SQL Hallucinates Schema Elements

**What goes wrong:** The LLM generates SQL that references tables, columns, or functions that do not exist in the Trino schema. The query either fails with a cryptic Trino error or -- worse -- executes against similarly-named columns and returns plausible but incorrect results. Users cannot distinguish between correct and hallucinated results, eroding trust in the entire "ask your data anything" feature.

**Why it happens:** LLMs are trained on general SQL corpora and "fill in" schema elements that sound plausible. Trino's SQL dialect has specific syntax differences from PostgreSQL/MySQL that LLMs commonly confuse (e.g., `BETWEEN` with VARCHAR vs DATE, `UNNEST` syntax, `TRY_CAST` instead of `CAST`). The Causality schema uses a single denormalized `events` table with protobuf-derived column names that may not match natural language concepts (e.g., `custom_params` JSON vs. "user properties").

**Consequences:** Users get wrong answers and don't know it. Trust in LLM analysis collapses after a few bad results. Engineering time wasted debugging "why did the AI say X" instead of building features. In the worst case, business decisions are made on hallucinated data.

**Prevention:**
- Always inject the exact Trino schema (table names, column names, column types) into the LLM prompt. Never rely on the LLM's training knowledge of your schema
- Validate generated SQL against the schema before execution: parse the SQL AST and verify every table/column reference exists
- Use Trino's `EXPLAIN` to validate query plans before running queries against real data
- Include Trino-specific SQL dialect tips in the system prompt (e.g., "Use `date_parse()` not `STR_TO_DATE()`, use `TRY_CAST` for safe casting, `BETWEEN` requires DATE type not VARCHAR")
- Implement a query result sanity check: if COUNT returns 0 or an astronomically large number, flag for review
- Show users the generated SQL alongside results so they can validate

**Detection:** Queries failing with "Column not found" or "Table not found" errors. Results that contradict known metrics. User feedback reporting "the AI is wrong."

**Confidence:** HIGH -- this is the most extensively documented failure mode in text-to-SQL literature. Research shows schema hallucination rates of 23% without grounding, reducible to 1% with proper schema injection and validation.

**Phase relevance:** Must be addressed at LLM feature design time, not retrofitted. The schema injection, validation, and dialect-specific prompting are architectural decisions, not incremental fixes.

**Sources:**
- [Why LLMs Struggle with Text-to-SQL (Select Star)](https://www.selectstar.com/resources/text-to-sql-llm)
- [LLM Text-to-SQL: Top Challenges and Tips (K2View)](https://www.k2view.com/blog/llm-text-to-sql/)
- [Why 90% Accuracy in Text-to-SQL is 100% Useless (TDS)](https://towardsdatascience.com/why-90-accuracy-in-text-to-sql-is-100-useless/)
- [Techniques for Improving Text-to-SQL (Google Cloud)](https://cloud.google.com/blog/products/databases/techniques-for-improving-text-to-sql)

---

### Pitfall 3: LLM Text-to-SQL Creates a SQL Injection Attack Surface

**What goes wrong:** A user types a natural language prompt that tricks the LLM into generating destructive or data-exfiltrating SQL. Examples: "Show me all users; DROP TABLE events; --" or more sophisticated prompt injection that manipulates the LLM into ignoring its system prompt and generating `DELETE`, `UPDATE`, or `INSERT` statements. Even without malicious intent, a confused LLM might generate `SELECT *` on a multi-billion row table, consuming all Trino resources.

**Why it happens:** Text-to-SQL is fundamentally prompt-to-SQL-injection (P2SQL). The LLM is an untrusted SQL generator controlled by user input. Standard SQL injection defenses (parameterized queries) do not apply because the entire query structure is dynamic.

**Consequences:** Data destruction if write operations are allowed. Resource exhaustion from runaway queries. Data exfiltration if the Trino user has access to tables beyond the analytics schema. Complete platform compromise if the Trino connection has admin privileges.

**Prevention:**
- Execute all LLM-generated SQL with a read-only Trino user that has access only to the `causality` schema (no `INFORMATION_SCHEMA` access beyond what's needed for schema introspection)
- Parse the generated SQL and reject any statement that is not a `SELECT` (block `DROP`, `DELETE`, `UPDATE`, `INSERT`, `CREATE`, `ALTER`, `GRANT`)
- Implement query resource limits: `SET SESSION query_max_execution_time` and row limit via `LIMIT` injection
- Add a SQL allowlist/blocklist filter: reject queries containing `UNION SELECT` on system tables, `SLEEP`, `WAITFOR`, or other suspicious patterns
- Rate-limit LLM query execution per user/session
- Log all generated and executed SQL for audit

**Detection:** Trino audit logs showing unexpected query patterns. Queries accessing system tables. Resource usage spikes from unbounded queries.

**Confidence:** HIGH -- published academic research (2025) demonstrates SQL injection via backdoor attacks on text-to-SQL models. This is an active area of security research.

**Phase relevance:** Must be designed into the LLM feature from day one. Cannot be bolted on after the text-to-SQL pipeline is built. The read-only user and query validation are prerequisites for any LLM query execution.

**Sources:**
- [Are Your LLM-based Text-to-SQL Models Secure? (arXiv 2025)](https://arxiv.org/abs/2503.05445)
- [From Prompt Injections to SQL Injection Attacks (arXiv)](https://arxiv.org/pdf/2308.01990)
- [Text-to-SQL LLM Applications: Prompt Injections (Medium/TDS)](https://medium.com/data-science/text-to-sql-llm-applications-prompt-injections-ebee495d0c16)
- [Exploiting AI-Agents: Database Query-Based Prompt Injection (Keysight)](https://www.keysight.com/blogs/en/tech/nwvs/2025/07/31/db-query-based-prompt-injection)

---

### Pitfall 4: Graceful Shutdown Data Loss Under Kubernetes

**What goes wrong:** Kubernetes sends SIGTERM, the warehouse sink begins shutdown, but in-flight event batches are lost because: (a) the flush-on-shutdown races with the context timeout, (b) new events arrive during the drain period and are ACKed but never flushed, or (c) the Go process exits before the final S3 upload completes. The current codebase has documented shutdown coordination gaps (CONCERNS.md): if a goroutine panics before closing `doneCh`, `Stop()` deadlocks indefinitely.

**Why it happens:** Go's `http.Server.Shutdown()` only waits for in-flight HTTP requests, not background goroutines. Kubernetes has a default 30-second `terminationGracePeriodSeconds`, but if a preStop hook consumes part of that budget, the actual drain time is shorter than expected. The warehouse sink's batch buffer is in-memory with no durability.

**Consequences:** Events that were ACKed to NATS (and thus won't be redelivered) are permanently lost. Analytics data has gaps that are invisible to users. The more frequently pods restart (rolling updates, scaling events, node drains), the more data is lost.

**Prevention:**
- Add a timeout to `Stop()` to prevent deadlock (context with deadline, not infinite wait on `doneCh`)
- Add panic recovery in all goroutines that close `doneCh`
- On SIGTERM: stop accepting new messages immediately, flush all buffered batches, wait for S3 uploads to complete, then exit
- Use NATS consumer `AckWait` properly: do not ACK messages until they are durably written to S3. If the process dies, unacked messages will be redelivered
- Set Kubernetes `terminationGracePeriodSeconds` to at least 2x the maximum flush interval plus S3 upload time
- Consider writing batch checkpoints to local disk so incomplete batches survive restarts
- Add a preStop hook with a short sleep (5-10s) to allow kube-proxy to stop routing traffic before shutdown begins

**Detection:** Missing time ranges in Trino queries after pod restarts. Gaps in hourly partition file counts. Events in NATS that were ACKed but never appear in S3.

**Confidence:** HIGH -- this is directly observable in the current codebase (CONCERNS.md documents the exact deadlock and data loss paths). Kubernetes graceful shutdown is extensively documented with specific Go patterns.

**Phase relevance:** Must be the first thing fixed in pipeline hardening. Every subsequent phase (SDKs sending more events, dashboards displaying data) depends on zero data loss in the pipeline.

**Sources:**
- [Terminating Elegantly: Guide to Graceful Shutdowns (packagemain)](https://packagemain.tech/p/graceful-shutdowns-k8s-go)
- [How to Implement Graceful Shutdown in Go for Kubernetes (OneUptime)](https://oneuptime.com/blog/post/2026-01-07-go-graceful-shutdown-kubernetes/view)
- [Kubernetes Pod Graceful Shutdown with SIGTERM and preStop Hooks](https://devopscube.com/kubernetes-pod-graceful-shutdown/)
- Project CONCERNS.md: Shutdown Coordination Gaps, Graceful Shutdown Data Loss Risk

---

### Pitfall 5: gomobile Type System Restrictions Force Awkward SDK API Design

**What goes wrong:** gomobile bind only supports a narrow subset of Go types: signed integers, floats, strings, booleans, byte slices, and interfaces with methods using only those types. No slices of structs, no maps, no function callbacks, no generics, no error types beyond `error`. The team designs a clean Go SDK API using idiomatic Go types, then discovers it cannot be exported through gomobile. The result is either: (a) a complete API redesign at the gomobile boundary, or (b) serializing everything to JSON strings and passing those across the bridge, losing type safety entirely.

**Why it happens:** gomobile generates Objective-C and Java bindings via cgo, and the bridge layer supports only primitive types. This is a fundamental architectural constraint, not a bug that will be fixed. The Go issue tracker has open issues about this dating back to 2015 (golang/go#13445, #14614).

**Consequences:** The SDK's public API becomes either unidiomatic (everything is a string or byte slice) or split into two layers: a clean internal Go API and an ugly exported gomobile wrapper. Callbacks from native code to Go (e.g., "SDK is ready", "event was delivered") require an interface-based delegate pattern that is fragile across Go version upgrades (Go 1.22.5+ introduced a fatal stack split error with interface callbacks). Binary size is 5-10MB minimum, which is significant for mobile apps.

**Prevention:**
- Design the gomobile API boundary first, before writing internal SDK logic. The exported surface should be as thin as possible: `Init(configJSON string)`, `Track(eventJSON string)`, `Flush()`, `Shutdown()`
- Use JSON serialization at the boundary as the intentional API pattern, not a workaround. Document it. The native SDK wrappers (Swift/Kotlin) parse JSON into native types
- Keep the callback interface minimal: one interface with one or two methods. Avoid chaining Go-to-native-to-Go calls (the Go 1.22.5+ stack split bug)
- Test with the specific Go toolchain version and NDK version before committing. gomobile has known incompatibilities with NDK 24+ (golang/go#52470)
- Budget for 5-10MB binary size in the app size analysis. Use `-target=ios/arm64` (not fat binaries) to minimize size
- Write thin Swift and Kotlin wrapper libraries that provide idiomatic APIs on top of the gomobile bridge

**Detection:** Build failures when upgrading Go versions. Crashes on iOS/Android with stack traces pointing to cgo bridge code. App store rejection for binary size.

**Confidence:** HIGH -- gomobile type restrictions are documented in official Go packages documentation. Binary size issues confirmed in multiple GitHub issues (#15223, #14465). Go 1.22.5 stack split bug confirmed in #68760.

**Phase relevance:** Must be addressed at SDK design phase. The API boundary design is an irreversible architectural decision. Changing it after native wrapper libraries and app integrations exist is extremely expensive.

**Sources:**
- [Supported Go types for gomobile bind (pliutau.com)](https://pliutau.com/gomobile-bind-types/)
- [gomobile command documentation (pkg.go.dev)](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)
- [golang/go#52470: Gomobile incompatible with modern Android SDKs](https://github.com/golang/go/issues/52470)
- [golang/go#68760: bind with go1.22.5 stack split fatal error](https://github.com/golang/go/issues/68760)
- [golang/go#13445: support slices of supported structs](https://github.com/golang/go/issues/13445)

---

### Pitfall 6: No Event Deduplication Corrupts Analytics

**What goes wrong:** Mobile SDKs retry event delivery when the network is unreliable. The HTTP server returns a timeout, but the event was actually published to NATS successfully. The SDK retries, creating a duplicate. The warehouse sink writes both copies to Parquet. Analytics queries double-count events. Funnels show impossible conversion rates. Purchase totals are inflated.

**Why it happens:** The current pipeline has no deduplication at any layer (documented in CONCERNS.md). HTTP event ingestion generates a UUID v7 server-side, so retried events get different IDs. There is no client-side idempotency key. NATS JetStream provides at-least-once delivery, not exactly-once.

**Consequences:** All analytics metrics are unreliable. Business decisions based on inflated numbers. Engineers spend time investigating "why don't the numbers match" instead of building features. The problem gets worse as mobile SDK usage increases (more retries from spotty networks).

**Prevention:**
- Require client-side idempotency keys on all events (SDK generates a UUID before sending, includes it in the event payload)
- Implement server-side deduplication using a sliding window: maintain a set of recently-seen idempotency keys (Redis with TTL or in-memory with size limit) and reject duplicates at the HTTP layer with 200 OK (not an error -- the client already sent this)
- As a defense-in-depth measure, deduplicate in the warehouse sink before writing to Parquet: if the same event ID appears in the current batch, drop the duplicate
- For historical data cleanup, implement a periodic compaction job that deduplicates within Parquet files based on event ID

**Detection:** Duplicate event IDs in Trino queries. Event counts that don't match expected volumes. Funnel conversion rates exceeding 100%.

**Confidence:** HIGH -- this is directly documented in the codebase CONCERNS.md as a known missing feature. Deduplication is a standard requirement for at-least-once messaging systems.

**Phase relevance:** Must be addressed in pipeline hardening, before SDKs ship. Once SDKs are in production and sending duplicate events, the corrupted data is permanent in the warehouse.

**Sources:**
- [Usage Deduplication is Challenging (OpenMeter)](https://openmeter.io/blog/usage-deduplication)
- [Effective Deduplication of Events in Batch and Stream Processing (RisingWave)](https://risingwave.com/blog/effective-deduplication-of-events-in-batch-and-stream-processing/)
- [Handling Duplicate Data in Streaming Pipeline (Google Cloud)](https://cloud.google.com/blog/products/data-analytics/handling-duplicate-data-in-streaming-pipeline-using-pubsub-dataflow)
- Project CONCERNS.md: No Event Deduplication

---

## Moderate Pitfalls

Mistakes that cause delays, rework, or significant technical debt.

---

### Pitfall 7: Liveness Probes That Check External Dependencies Cause Cascading Restarts

**What goes wrong:** The Go services expose health endpoints that check NATS connectivity and PostgreSQL connections. When deployed to Kubernetes with liveness probes pointing at these endpoints, a temporary NATS outage causes all pods to fail liveness checks simultaneously. Kubernetes restarts all pods, which then all try to reconnect to NATS at the same time, creating a thundering herd. The system enters a restart loop.

**Why it happens:** Developers conflate "is this process alive?" (liveness) with "can this process do useful work?" (readiness). The current `/health` and `/ready` endpoints likely check NATS connectivity. Kubernetes liveness probes should only check whether the process is stuck (deadlock, infinite loop), not whether external services are available.

**Prevention:**
- Liveness probe (`/livez`): Only check that the HTTP server can respond. No external dependency checks. A simple 200 OK.
- Readiness probe (`/readyz`): Check NATS connectivity, PostgreSQL connectivity, and other external dependencies. Failing readiness removes the pod from service endpoints but does NOT restart it.
- Startup probe: Give services time to connect to NATS/PostgreSQL on cold start before liveness begins checking
- Never include database queries, NATS ping, or S3 connectivity in liveness probes
- Set `failureThreshold` on liveness probes high enough to tolerate transient issues (e.g., 5 failures at 10s intervals = 50s before restart)

**Detection:** Pods in CrashLoopBackOff during infrastructure outages. Multiple simultaneous pod restarts in logs. Services restarting when the problem is in NATS or PostgreSQL, not in the Go process itself.

**Confidence:** HIGH -- this is documented on the official Kubernetes blog as one of the seven common Kubernetes pitfalls.

**Phase relevance:** Address when creating Kubernetes manifests. Must be correct before production deployment.

**Sources:**
- [7 Common Kubernetes Pitfalls (kubernetes.io official blog)](https://kubernetes.io/blog/2025/10/20/seven-kubernetes-pitfalls-and-how-to-avoid/)
- [Kubernetes Liveness and Readiness Probes: How to Avoid Shooting Yourself in the Foot](https://blog.colinbreck.com/kubernetes-liveness-and-readiness-probes-how-to-avoid-shooting-yourself-in-the-foot/)
- [Kubernetes Liveness Probes: Examples and Common Pitfalls (vcluster)](https://www.vcluster.com/blog/kubernetes-liveness-probes-examples-and-common-pitfalls)

---

### Pitfall 8: Mobile SDK Blocks the UI Thread

**What goes wrong:** The gomobile-generated SDK performs network I/O, disk I/O (event queue persistence), or JSON serialization on the main/UI thread. On iOS, this triggers ANR (Application Not Responding) watchdog kills after 5 seconds. On Android, it triggers ANR dialogs and StrictMode violations. App store reviewers reject the app or users see persistent jank.

**Why it happens:** gomobile's generated bindings are synchronous by default. When a Swift/Kotlin app calls `CausalitySDK.Track(eventJSON)`, if the Go code does any blocking work (writing to disk, HTTP POST, JSON marshaling of large payloads), it blocks the calling thread. Developers test with small payloads and fast networks and don't notice until production.

**Consequences:** Apps using the SDK get poor App Store ratings. SDK gets blamed and removed. User event data stops flowing. The entire analytics platform becomes useless without instrumented apps.

**Prevention:**
- The gomobile exported API must be completely non-blocking. `Track()` should only append to an in-memory queue and return immediately
- All I/O (disk persistence, network upload, batch flushing) must happen on Go goroutines spawned internally, never on the calling thread
- The native wrapper libraries (Swift/Kotlin) should additionally dispatch SDK calls to a background queue/thread as defense-in-depth
- Implement a bounded in-memory queue with disk overflow. If the queue is full, drop the oldest low-priority events rather than blocking
- Add SDK integration tests that measure `Track()` call latency -- must be under 1ms

**Detection:** ANR reports in Google Play Console or Xcode Organizer. `Track()` call duration exceeding 1ms in profiling. User complaints about app freezing.

**Confidence:** HIGH -- extensively documented in mobile analytics SDK design literature. Segment, Amplitude, and Mixpanel SDKs all use async queue patterns for exactly this reason.

**Phase relevance:** Must be designed into the SDK from the start. The queue architecture is the SDK's core data structure. Retrofitting async behavior into a synchronous API is a breaking change.

**Sources:**
- [The Hidden Performance Cost of Excessive Mobile Analytics (droidcon)](https://www.droidcon.com/2025/07/04/the-hidden-performance-cost-of-excessive-mobile-analytics/)
- [How Mobile Event Tracking SDKs Work: A Deep Dive (Medium)](https://amit-engineer.medium.com/how-an-event-tracking-sdk-works-on-android-d8c19b50eea3)
- [Android System Design: Building a Reliable Analytics Pipeline (Medium)](https://medium.com/@prabhat.rai1707/android-system-design-building-a-reliable-analytics-pipeline-eafa4412a3a8)

---

### Pitfall 9: NATS Consumer Scaling Bottleneck

**What goes wrong:** The current architecture uses a single consumer instance per stream (documented in CONCERNS.md). As event volume grows, the single warehouse-sink consumer cannot keep up. The NATS stream accumulates a backlog. Event freshness in the warehouse degrades from minutes to hours. Dashboards show stale data.

**Why it happens:** The current consumer uses a sequential fetch loop processing one batch at a time. NATS JetStream pull consumers support horizontal scaling, but the consumer must be configured as a durable pull consumer with `MaxAckPending` set appropriately. The current code fetches 100 messages with a 5-second timeout, processes them sequentially, then fetches again. There is no parallelism within the consumer and no support for multiple consumer instances.

**Prevention:**
- Convert to NATS JetStream pull consumers (which are designed for horizontal scaling)
- Use consumer groups: multiple instances of warehouse-sink can share a durable consumer, and NATS will distribute messages across instances
- Within each consumer instance, use a worker pool pattern: fetch messages in one goroutine, process in N worker goroutines
- Set `MaxAckPending` appropriately -- too low throttles throughput, too high risks message redelivery storms on crash
- Set the JetStream buffer size to at least 65536 to avoid dropped messages under load
- Monitor consumer lag (pending message count) as the primary capacity metric
- Never set `PullMaxMessagesWithBytesLimit` lower than the maximum expected message size (this causes consumer stalls)

**Detection:** Growing `NumPending` on NATS consumer. Increasing latency between event ingestion timestamp and warehouse write timestamp. Consumer CPU at 100% on a single core while other cores are idle.

**Confidence:** HIGH -- NATS JetStream pull consumer scaling is well-documented. Consumer stall bug with byte limits confirmed in NATS GitHub discussions.

**Phase relevance:** Address in pipeline hardening phase. Must be resolved before SDKs increase event volume by 10-100x.

**Sources:**
- [NATS JetStream Consumers documentation](https://docs.nats.io/nats-concepts/jetstream/consumers)
- [NATS slow consumer discussion (nats.go #888)](https://github.com/nats-io/nats.go/discussions/888)
- [Scaling JetStream replicas issues (nats-server #6838)](https://github.com/nats-io/nats-server/issues/6838)
- Project CONCERNS.md: Single Consumer Instance per Stream, Blocking Consumer Fetch with No Parallelism

---

### Pitfall 10: React Dashboard Over-Rendering Kills Performance with High-Volume Data

**What goes wrong:** The analytics dashboard renders charts, tables, and real-time counters. Each chart component subscribes to query results. When multiple queries return simultaneously (page load, filter change), React re-renders the entire dashboard. Charts with thousands of data points (30 days of hourly event counts = 720 points per series, times 10 event types = 7,200 points) cause jank. Users perceive the dashboard as "slow" even though the backend is fast.

**Why it happens:** Developers treat dashboard components like form inputs -- every state change triggers a full re-render. Data fetching, state management, and rendering are tightly coupled. Large datasets are passed as props without memoization. Charts re-animate on every render.

**Consequences:** Dashboard feels sluggish. Users switch back to Redash or raw SQL. The "beautiful custom dashboard" provides worse UX than the generic tool it replaced.

**Prevention:**
- Separate data fetching from rendering: use React Query (TanStack Query) for server state, keeping query results cached and deduplicated
- Use URL state (search params) for filter state, not component state -- this enables shareable dashboard URLs and reduces unnecessary re-renders
- Batch state updates: when multiple filters change, defer re-rendering until all changes are applied (React 18+ automatic batching helps, but be deliberate)
- Virtualize large tables: never render more than the visible rows. Use TanStack Table with virtualization
- For charts with large datasets: downsample data server-side (Trino can do time-bucketed aggregations) rather than sending 100K points to the browser
- Memoize chart components with `React.memo` and stable keys to prevent re-animation
- Use a SQL editor component (Monaco) only when the SQL tab is active -- lazy load it as it is heavy (~2MB)

**Detection:** React DevTools Profiler showing unnecessary re-renders. Lighthouse performance score below 50. User-reported "lag" when changing filters. Browser memory usage exceeding 500MB.

**Confidence:** MEDIUM -- based on general React dashboard best practices and common patterns in analytics frontends. Specific to Causality's data volumes and chart types.

**Phase relevance:** Address during web app architecture design. Component boundaries and state management patterns are hard to change after dozens of components are built.

**Sources:**
- [Common Mistakes in React Admin Dashboards (dev.to)](https://dev.to/vaibhavg/common-mistakes-in-react-admin-dashboards-and-how-to-avoid-them-1i70)
- [ReactJS Development for Real-Time Analytics Dashboards (Makers' Den)](https://makersden.io/blog/reactjs-dev-for-real-time-analytics-dashboards)
- [Dashboard Design Principles for 2025 (UXPin)](https://www.uxpin.com/studio/blog/dashboard-design-principles/)

---

### Pitfall 11: Silent ACK-Before-Process Loses Events Permanently

**What goes wrong:** The reaction engine consumer ACKs messages even when rule evaluation or anomaly detection fails (documented in CONCERNS.md: "error is logged but message is still ACKed successfully"). If a systemic failure occurs (database down, malformed event type), every affected event is ACKed and lost. There is no DLQ. There are no metrics to detect the failure rate.

**Why it happens:** The "continue on error" pattern was a reasonable prototype choice to prevent a single bad event from blocking the stream. But without metrics, alerting, or a dead letter queue, the failure rate is invisible. The system silently degrades.

**Consequences:** Webhooks are never triggered for events that hit processing errors. Anomaly detection misses patterns because events are dropped. No one knows until someone asks "why didn't we get an alert for X?"

**Prevention:**
- Implement a dead letter queue (DLQ): events that fail processing N times go to a separate NATS stream for manual inspection
- Add Prometheus metrics: `events_processed_total`, `events_failed_total`, `events_dlq_total` with labels for error type
- Do not ACK messages that fail processing -- use NATS NAK with delay for retryable errors, and move to DLQ for non-retryable errors (malformed events, unknown event types)
- Add alerting on error rate: if `events_failed_total / events_processed_total > 1%`, fire an alert
- Log failed events with full context (event ID, error, raw payload) for debugging

**Detection:** Currently undetectable without log analysis. After fix: DLQ depth growing, error rate metrics exceeding threshold.

**Confidence:** HIGH -- directly documented in codebase CONCERNS.md. Standard pattern for message processing systems.

**Phase relevance:** Fix in pipeline hardening phase, before increasing event volume with SDKs.

**Sources:**
- Project CONCERNS.md: Silent Errors in Reaction Processing, Missing Message Dead Letter Queue
- [Backpressure in Streaming Systems (RisingWave)](https://risingwave.com/blog/backpressure-in-streaming-systems/)

---

### Pitfall 12: Kubernetes Resource Limits Set Wrong Cause OOMKills or CPU Throttling

**What goes wrong:** Go services are deployed to Kubernetes without accurate resource limits. Two failure modes: (a) Limits too low: the warehouse sink OOMKills during large batch flushes because Parquet serialization temporarily doubles memory usage. The pod restarts, losing the in-flight batch. (b) Limits too high: resource overprovisioning wastes cloud budget. Kubernetes cannot efficiently bin-pack pods, leading to node underutilization.

**Why it happens:** Developers guess resource limits or copy defaults. Go's garbage collector behavior (memory is released lazily) makes actual memory usage hard to predict. The warehouse sink's memory usage scales with `batch_size * avg_event_size`, but this is not obvious from the code. The reaction engine's memory usage scales with rule count.

**Prevention:**
- Profile each service under realistic load before setting limits. Use `runtime.MemStats` or Prometheus Go metrics to measure actual peak memory
- Set Go's `GOMEMLIMIT` environment variable to 80-90% of the container memory limit to prevent OOMKills (Go 1.19+ feature). This tells the GC to be more aggressive before hitting the container limit
- Set CPU requests (not limits) for Go services -- CPU limits cause throttling that is hard to diagnose, and Go's goroutine scheduler works better with full CPU access
- Start with generous limits in staging, measure actual usage over 48 hours, then right-size
- Use Kubernetes VPA (Vertical Pod Autoscaler) in recommendation mode to suggest limits based on actual usage
- For the warehouse sink: memory limit = 2 * batch_size * max_event_size + overhead

**Detection:** OOMKilled events in pod status. High CPU throttling in container metrics (`container_cpu_cfs_throttled_periods_total`). Memory usage flatlined at the limit (indicating GC pressure).

**Confidence:** HIGH -- well-documented Kubernetes operational pattern. `GOMEMLIMIT` is a standard Go-on-Kubernetes best practice since Go 1.19.

**Phase relevance:** Address during Kubernetes deployment phase. Must be tuned before production traffic.

**Sources:**
- [2025's Top 8 Kubernetes Deployment Mistakes to Avoid (Cpluz)](https://cpluz.com/blog/2025s-top-8-kubernetes-deployment-mistakes-to-avoid-in-2026/)
- [7 Common Kubernetes Pitfalls (kubernetes.io)](https://kubernetes.io/blog/2025/10/20/seven-kubernetes-pitfalls-and-how-to-avoid/)
- [Kubernetes Best Practices 2025 (KodeKloud)](https://kodekloud.com/blog/kubernetes-best-practices-2025/)

---

### Pitfall 13: No Request Authentication Allows Event Injection

**What goes wrong:** The HTTP event ingestion endpoint is unauthenticated (documented in CONCERNS.md). Anyone who discovers the endpoint URL can submit fake events. A malicious actor injects thousands of `purchaseComplete` events, making revenue analytics unreliable. Or they inject events with another app's `app_id`, polluting that app's data.

**Why it happens:** Authentication was deferred during the prototype phase. The `app_id` is sent in the event payload by the client, not validated server-side. There is no API key system.

**Consequences:** All analytics become untrustworthy. Cannot distinguish real events from injected ones. No way to revoke access for a compromised client. No rate limiting per tenant.

**Prevention:**
- Implement API key authentication: each app gets an API key that must be included in the `Authorization` header
- API keys should be scoped: an API key for app "X" can only submit events with `app_id: "X"`. Server validates this.
- Add request body size limiting (`http.MaxBytesHandler`) to prevent memory exhaustion from oversized payloads
- Add per-API-key rate limiting (the current global rate limiter is insufficient for multi-tenant use)
- For the web app, use session-based auth (JWT or cookie) separate from the SDK API key auth

**Detection:** Events from unknown IP ranges. Event volumes that don't correlate with app usage. Events with app_ids that don't correspond to registered apps.

**Confidence:** HIGH -- directly documented in CONCERNS.md. Authentication is table stakes for any API accepting external input.

**Phase relevance:** Must be in pipeline hardening, before SDKs ship. Shipping an SDK with a hardcoded API key to an unauthenticated endpoint is a security incident waiting to happen.

**Sources:**
- Project CONCERNS.md: No Request Authentication, Rate Limiting Bypasses, No Request Body Size Limiting

---

## Minor Pitfalls

Mistakes that cause annoyance, rework, or minor technical debt.

---

### Pitfall 14: Mobile SDK Offline Queue Without Disk Persistence Loses Events

**What goes wrong:** The gomobile SDK uses an in-memory event queue. When the app is killed by the OS (backgrounded too long, memory pressure), all queued events are lost. Users who have poor connectivity lose the most events -- exactly the users whose behavior is hardest to observe.

**Prevention:**
- Implement disk-based event persistence using a lightweight format (SQLite via gomobile, or a simple append-only file)
- Events are written to disk before `Track()` returns, and removed from disk after successful upload
- Cap disk storage (e.g., 5MB) and drop oldest events when full
- Priority-based eviction: low-priority events (screenView) are dropped before high-priority events (purchaseComplete)
- Follow Segment's model: persist up to 1,000 events on disk, never expire them, retry on next app launch

**Confidence:** HIGH -- standard pattern in mobile analytics SDKs. Segment, Amplitude, and Mixpanel all use disk persistence.

**Phase relevance:** SDK development phase. Must be included in the initial SDK architecture.

**Sources:**
- [Persistent Storage for an Event Framework (Keen.io)](https://keen.io/blog/persistent-storage-for-an-event-framework/)
- [Analytics-Android Documentation (Segment)](https://segment.com/docs/connections/sources/catalog/libraries/mobile/android/)

---

### Pitfall 15: Using `latest` Image Tag in Kubernetes Manifests

**What goes wrong:** Kubernetes manifests reference images as `causality-server:latest`. Deployments cannot be rolled back because the previous `latest` has been overwritten. Two pods in the same deployment run different code if they were pulled at different times. `imagePullPolicy: Always` adds latency to every pod start.

**Prevention:**
- Tag all images with the git commit SHA or semantic version
- Set `imagePullPolicy: IfNotPresent` (default for non-latest tags)
- Use Helm chart values or Kustomize overlays to inject the image tag from CI/CD
- Never use `:latest` in any manifest checked into version control

**Confidence:** HIGH -- one of the most frequently cited Kubernetes mistakes.

**Phase relevance:** Kubernetes deployment phase. Set up the CI/CD image tagging pipeline before writing manifests.

**Sources:**
- [2025's Top 8 Kubernetes Deployment Mistakes (Cpluz)](https://cpluz.com/blog/2025s-top-8-kubernetes-deployment-mistakes-to-avoid-in-2026/)
- [Top 10 Kubernetes Deployment Errors (The New Stack)](https://thenewstack.io/top-10-kubernetes-deployment-errors-causes-and-fixes-and-tips/)

---

### Pitfall 16: Trino Connection from Web App Creates Unbounded Resource Consumption

**What goes wrong:** The React web app's SQL editor allows users to execute arbitrary Trino queries. A user runs `SELECT * FROM events` (no LIMIT, no WHERE) on a table with millions of rows. Trino allocates all available memory and CPU to this query, starving other users' dashboard queries. The entire analytics platform becomes unresponsive.

**Prevention:**
- Automatically inject `LIMIT 10000` into all user-submitted queries (can be overridden up to a maximum)
- Set Trino resource groups: dashboard queries get priority over ad-hoc SQL editor queries
- Set per-query resource limits in Trino: `query.max-execution-time`, `query.max-memory`, `query.max-total-memory-per-node`
- Add a query cost estimator: use `EXPLAIN (TYPE ESTIMATED)` to estimate row count before execution and warn users about expensive queries
- Implement query cancellation: users should be able to cancel running queries, and the backend should cancel the Trino query (not just stop reading results)

**Confidence:** MEDIUM -- standard concern for analytics platforms with ad-hoc query capabilities. Specific Trino resource group configuration needs phase-specific research.

**Phase relevance:** Web app phase, when the SQL editor is built. Must be designed into the query execution layer.

---

### Pitfall 17: templ + htmx Public Pages and React SPA Routing Conflict

**What goes wrong:** The Go server serves templ+htmx pages at `/`, `/pricing`, `/docs`, etc. The React SPA is served at `/app/*`. Routing conflicts arise: the Go server's catch-all handler interferes with the SPA's client-side routing. Deep links to `/app/dashboard/funnel` result in 404 because the Go server doesn't know to serve the React SPA's `index.html` for all `/app/*` paths.

**Prevention:**
- Define a strict URL namespace: `/app/*` serves the React SPA (single `index.html` for all sub-paths), everything else is templ+htmx
- Configure the Go HTTP server to serve `index.html` for any `/app/*` path that doesn't match a static asset (standard SPA fallback pattern)
- Use a reverse proxy (nginx, Caddy, or the Go server itself) that routes by path prefix
- Test deep linking and browser refresh on all `/app/*` paths during development

**Confidence:** MEDIUM -- standard multi-frontend architecture concern. Well-understood solution patterns.

**Phase relevance:** Address when scaffolding the web app serving layer. Must be decided before building either the templ pages or the React SPA.

---

### Pitfall 18: Synchronous Rule Evaluation Scales Linearly with Rule Count

**What goes wrong:** Each incoming event is evaluated against all active rules in a linear scan (O(n) per event). At 100 rules, this is fine. At 1,000 rules, latency per event becomes noticeable. At 10,000 rules, the reaction engine becomes a bottleneck and events back up in NATS.

**Prevention:**
- Index rules by `(app_id, event_category, event_type)` tuple. On each event, look up only matching rules instead of scanning all rules
- This is a simple HashMap lookup that reduces evaluation from O(n) to O(m) where m is the number of rules matching this specific event type (typically 1-10, not 1,000+)
- Consider a two-phase evaluation: fast filter (indexed lookup) then slow evaluation (JSONPath conditions) only on candidates

**Confidence:** HIGH -- directly documented in CONCERNS.md as a known performance bottleneck.

**Phase relevance:** Pipeline hardening phase. Should be fixed before the web app adds a rule management UI that makes it easy to create hundreds of rules.

**Sources:**
- Project CONCERNS.md: Synchronous Rule Evaluation on Hot Path

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Severity | Mitigation |
|---|---|---|---|
| Pipeline Hardening | Data loss on shutdown (Pitfall 4) | CRITICAL | Fix shutdown coordination, ACK-after-write, preStop hook |
| Pipeline Hardening | No deduplication (Pitfall 6) | CRITICAL | Add idempotency keys before SDKs ship |
| Pipeline Hardening | Silent ACK-before-process (Pitfall 11) | MODERATE | Add DLQ, metrics, conditional ACK |
| Pipeline Hardening | No authentication (Pitfall 13) | CRITICAL | Add API key auth before SDKs ship |
| Pipeline Hardening | Small files problem (Pitfall 1) | CRITICAL | Add compaction job or migrate to Iceberg |
| Pipeline Hardening | Linear rule scan (Pitfall 18) | MODERATE | Index rules by event type tuple |
| Pipeline Hardening | Consumer scaling bottleneck (Pitfall 9) | MODERATE | Convert to pull consumers with worker pool |
| SDK Development | gomobile type restrictions (Pitfall 5) | CRITICAL | Design thin JSON bridge API first |
| SDK Development | UI thread blocking (Pitfall 8) | CRITICAL | Async queue architecture from day one |
| SDK Development | Offline event loss (Pitfall 14) | MODERATE | Disk persistence with priority eviction |
| LLM Analysis | Schema hallucination (Pitfall 2) | CRITICAL | Schema injection, SQL validation, dialect tips |
| LLM Analysis | SQL injection via LLM (Pitfall 3) | CRITICAL | Read-only user, query parsing, resource limits |
| Web App | Dashboard over-rendering (Pitfall 10) | MODERATE | Server state management, virtualization, memoization |
| Web App | Unbounded Trino queries (Pitfall 16) | MODERATE | Auto-LIMIT, resource groups, query cancellation |
| Web App | SPA/SSR routing conflict (Pitfall 17) | MINOR | Strict URL namespace, SPA fallback handler |
| Kubernetes | Liveness probe cascading failure (Pitfall 7) | MODERATE | Separate livez/readyz, no external deps in liveness |
| Kubernetes | Resource limits misconfiguration (Pitfall 12) | MODERATE | Profile under load, use GOMEMLIMIT, avoid CPU limits |
| Kubernetes | Latest image tag (Pitfall 15) | MINOR | Git SHA tagging, CI/CD pipeline |

---

## Cross-Cutting Warnings

These apply across multiple phases and are easy to miss.

### Observability Must Come Before Features

The codebase has zero metrics, no distributed tracing, and structured logging that is not connected to any aggregation system. Every subsequent phase (SDKs, web app, LLM, K8s) will generate issues that are impossible to debug without observability. Observability is not a "nice to have" -- it is a prerequisite for operating the system at any scale.

**Warning sign:** "We'll add monitoring later" is the most common phrase spoken right before a production incident that takes 4 hours to diagnose because there are no metrics.

**Recommendation:** Add Prometheus metrics and structured log aggregation in the pipeline hardening phase, before adding any new features. Every subsequent component should emit metrics from day one.

### Test Coverage Debt Compounds

At 11% test coverage, every new feature added without tests increases the cost of eventually adding tests. The reaction engine (581 lines, zero tests) and anomaly detector (535 lines, zero tests) are the highest-risk components. If these are modified during hardening without tests, regressions are inevitable.

**Warning sign:** "I changed this function and something else broke" is how you discover insufficient test coverage.

**Recommendation:** Write tests for existing code in the hardening phase, before modifying it. Test-then-refactor, not refactor-then-test.

---

*Pitfalls research: 2026-02-05*
*Overall confidence: HIGH for pipeline/K8s/SDK pitfalls (grounded in codebase analysis + official docs), MEDIUM for LLM/frontend pitfalls (based on ecosystem research, not codebase-specific validation)*
