# Codebase Concerns

**Analysis Date:** 2026-02-05

## Tech Debt

**Incomplete Event Type Conversion in Anomaly Detector:**
- Issue: The `eventToJSON()` method in `internal/reaction/anomaly.go` only converts a subset of event types (ProductView, AddToCart, PurchaseComplete, CustomEvent) compared to the full Engine implementation which handles 26+ types.
- Files: `internal/reaction/anomaly.go:463-505`
- Impact: Anomaly detection for event types like ScreenView, ButtonTap, UserLogin, etc. will have incomplete event data in detection logic, potentially limiting anomaly rules that depend on these fields.
- Fix approach: Implement full event type conversion matching `internal/reaction/engine.go:500-559` for all event types in AnomalyDetector, or extract to shared utility function to prevent drift.

**Silent JSON Marshaling Errors in Anomaly Detector:**
- Issue: Line 371 in `internal/reaction/anomaly.go` ignores JSON marshaling errors: `eventDataJSON, _ = json.Marshal(eventJSON)`
- Files: `internal/reaction/anomaly.go:371`
- Impact: If JSON marshaling fails (e.g., due to circular references or non-serializable types), the event data is silently dropped from anomaly event records, losing debugging information.
- Fix approach: Check error and log it, or handle the error case explicitly rather than silencing.

**Deferred Close Errors Silenced Across Database Operations:**
- Issue: Multiple database and I/O operations use `defer func() { _ = resource.Close() }()` pattern throughout the codebase.
- Files:
  - `internal/reaction/db/webhooks.go:115, 163, 268`
  - `internal/reaction/db/rules.go:142, 164, 286`
  - `internal/reaction/db/client.go:78`
  - `internal/reaction/db/deliveries.go:85, 95, 133, 297, 354`
  - `internal/reaction/db/anomaly_configs.go:145, 166, 268, 307`
  - `internal/reaction/dispatcher.go:194`
- Impact: Resource cleanup failures are undetected. Connection pool exhaustion or transaction rollback failures will go unnoticed, potentially leading to resource leaks under error conditions.
- Fix approach: Log close errors at debug level, or handle specific errors like transaction rollback failures at error level.

## Test Coverage Gaps

**Minimal Test Coverage:**
- Issue: Only 3 test files exist for 27+ internal Go files (11% coverage by file count). No tests for critical components.
- Files with no tests:
  - `internal/reaction/engine.go` (581 lines - core rule evaluation logic)
  - `internal/reaction/anomaly.go` (535 lines - anomaly detection)
  - `internal/reaction/dispatcher.go` (264 lines - webhook delivery)
  - `internal/reaction/consumer.go` (170 lines - NATS consumption)
  - `internal/warehouse/consumer.go` (299 lines - event batching and S3 upload)
  - `internal/warehouse/s3.go` (164 lines - S3 operations)
  - `internal/gateway/server.go` (123 lines - HTTP server setup)
  - `internal/gateway/middleware.go` (192 lines - request handling)
  - All database repositories (5 files, ~1300 lines)
  - All NATS client code (publisher, stream manager)
- Impact: Critical business logic lacks automated validation. Rule evaluation, anomaly detection, and data warehouse operations are unverified by tests. Regressions will only be caught in production.
- Priority: High
- Fix approach: Add unit tests for core logic paths (rule matching, condition evaluation, anomaly detection), integration tests for consumer patterns, and tests for database operations using test databases.

## Fragile Areas

**Rule and Anomaly Config Cache with Stale Reload:**
- Files: `internal/reaction/engine.go:31-32`, `internal/reaction/anomaly.go:46-47`
- Why fragile: In-memory cache with background refresh. If refresh fails, stale rules remain cached with no immediate indication. Multiple concurrent readers can access cache without versioning or atomic updates.
- Safe modification: Add version tracking to cached rules, log failures prominently, consider adding metrics for staleness. Use atomic swap or mutex-protected versioning.
- Test coverage: Zero tests for cache consistency during concurrent operations.

**JSONPath Extraction Implementation:**
- Files: `internal/reaction/engine.go:241-262`, `internal/reaction/anomaly.go:442-461`
- Why fragile: Simple string-split based parsing with no validation. Duplicate implementations across Engine and AnomalyDetector. No support for array indexing, wildcard matching, or complex JSONPath features. Malformed paths fail silently returning false.
- Safe modification: Extract to shared utility, add path validation, consider using a JSONPath library (e.g., github.com/PaesslerAG/jsonpath).
- Test coverage: Zero tests for edge cases (empty paths, nested objects, missing fields).

**Silent Failures in Message Processing:**
- Files: `internal/warehouse/consumer.go:97-103`, `internal/reaction/consumer.go:86-93`
- Why fragile: When message fetch fails, the loop continues with no backoff. Infinite busy-wait on persistent errors burns CPU. NAK and ACK errors are logged but don't stop processing loop.
- Safe modification: Add exponential backoff on fetch errors, track consecutive failures and break/reconnect after threshold.
- Test coverage: Zero tests for error handling paths.

**Shutdown Coordination Gaps:**
- Files: `internal/warehouse/consumer.go:282-299`, `internal/reaction/consumer.go:157-170`
- Why fragile: Stop signal closes stopCh and waits for doneCh, but if processing goroutine panics before closing doneCh, Stop() will deadlock. No timeout protection in Stop().
- Safe modification: Use a timeout in Stop() context, add panic recovery in goroutines, or use WaitGroup with timeout.
- Test coverage: Zero tests for graceful shutdown sequences.

## Performance Bottlenecks

**No Request Body Size Limiting:**
- Problem: HTTP server sets MaxHeaderBytes (1MB) but doesn't limit request body size.
- Files: `internal/gateway/config.go:23`, `internal/gateway/server.go:63-70`
- Cause: Using standard net/http with no explicit body size limits. A client can send arbitrarily large JSON payloads.
- Impact: Malicious or buggy clients can exhaust memory with large batch requests. Batch endpoint at `/v1/events/batch` accepts arbitrary number of events with no validation.
- Improvement path: Add `http.MaxBytesHandler()` wrapper or explicit Content-Length validation in handlers. Add MaxEvents constraint to batch requests.

**Synchronous Rule Evaluation on Hot Path:**
- Problem: Each event undergoes full rule evaluation against all active rules with no caching or early termination.
- Files: `internal/reaction/engine.go:139-147`, `internal/reaction/engine.go:173-189`
- Cause: Linear scan through all rules for each event. No indexing by app_id/event_type.
- Impact: Latency increases linearly with number of rules. At scale (1000+ rules), each event faces O(n) matching.
- Improvement path: Index rules by (app_id, event_category, event_type) tuple instead of flat list scan.

**Anomaly Detection State Increment Without Batching:**
- Problem: `IncrementStateCount()` in anomaly detector performs individual database operations per event.
- Files: `internal/reaction/anomaly.go:288, 320`
- Cause: Each event triggers a separate DB write to increment counters.
- Impact: At high event volumes, DB write throughput becomes bottleneck. No batching of state increments.
- Improvement path: Use Redis or in-process rate limit counter with periodic flush, or batch state updates.

**Full Event-to-JSON Conversion on Every Processing:**
- Problem: Both rule engine and anomaly detector independently convert protobuf events to JSON.
- Files: `internal/reaction/engine.go:467`, `internal/reaction/anomaly.go:464`
- Cause: No shared conversion or caching.
- Impact: Repeated JSON marshaling for same event across multiple processing pipelines.
- Improvement path: Convert once in consumer, pass JSON through pipeline.

**Blocking Consumer Fetch with No Parallelism:**
- Problem: Single blocking fetch per consumer with 100 message batch and 5s timeout.
- Files: `internal/warehouse/consumer.go:97`, `internal/reaction/consumer.go:87`
- Cause: Sequential message processing within single goroutine per consumer.
- Impact: Even with multiple consumers, message throughput is limited. If processing latency is high, fewer messages are fetched.
- Improvement path: Use worker pool pattern within consumer, parallel processing of message batches.

## Scaling Limits

**In-Memory Rule and Config Cache:**
- Current capacity: All enabled rules/configs loaded into memory on startup and periodically refreshed.
- Limit: Linear memory growth with rule count. At 10,000+ rules, memory usage becomes significant. Cache refresh blocks reader access due to mutex lock.
- Scaling path: Implement distributed rule cache (Redis), partition rules by tenant/app, implement read-through caching with TTL.

**Single Consumer Instance per Stream:**
- Current capacity: One warehouse consumer and one reaction consumer instance per Causality deployment.
- Limit: Throughput capped by single-instance processing latency. No horizontal scaling.
- Scaling path: Partition stream by app_id, run multiple consumer instances with consumer group semantics, coordinate partitioned state management.

**PostgreSQL Direct Connection for Reaction State:**
- Current capacity: Fixed connection pool (default 25 open, 5 idle) for all database operations.
- Limit: Database becomes bottleneck under high delivery/anomaly detection load. No read replicas.
- Scaling path: Connection pooling service (pgBouncer), read replicas for webhook history queries, consider time-series database for anomaly state.

**File-Based Batch Flush Strategy:**
- Current capacity: Events batched in memory until max_events or flush_interval threshold.
- Limit: Memory usage per consumer = batch_size * avg_event_size. Loss of in-flight batch on crash.
- Scaling path: Implement durability with batch checkpointing, batch spilling to local disk, or NATS consumer offset management.

## Scaling Limits

**S3 Upload Latency on Data Path:**
- Problem: Warehouse consumer waits synchronously for S3 upload before ACKing messages.
- Files: `internal/warehouse/consumer.go:266-270`
- Cause: Flush operation is synchronous; upload blocks event consumption.
- Impact: If S3 latency spikes (network issues, MinIO overload), event ingestion throughput drops.
- Scaling path: Decouple upload from consumption via local queue, implement async upload with retry queue.

## Known Issues & Behaviors

**Silent Errors in Reaction Processing:**
- Behavior: When rule or anomaly processing fails, error is logged but message is still ACKed successfully.
- Files: `internal/reaction/consumer.go:136-141, 146-150`
- Impact: Failed rule evaluations or anomaly detections are not retried. Webhooks for those events will not be queued.
- Mitigation: Currently by design (continue on error), but no metrics to detect systematic failures.

**Graceful Shutdown Data Loss Risk:**
- Problem: On shutdown, in-flight batches in warehouse consumer may be lost if flush is interrupted.
- Files: `internal/warehouse/consumer.go:282-299`
- Cause: Final flush happens after consumer loop is stopped, but if context times out during flush, events are lost.
- Impact: Events in batch buffer at shutdown time are not written to S3.
- Mitigation: Set adequate shutdown timeout and flush interval to minimize batch size.

**Webhook Delivery Without Guaranteed Ordering:**
- Problem: Multiple webhooks from different rules for same event are delivered concurrently without ordering guarantees.
- Files: `internal/reaction/dispatcher.go` concurrent worker pool
- Impact: Webhooks may arrive out of order. Systems expecting event ordering will see violations.
- Mitigation: Configure single dispatcher worker if ordering required, but impacts throughput.

**Missing Request Validation:**
- Problem: HTTP batch endpoint accepts events with missing required fields (app_id, device_id).
- Files: `internal/gateway/service.go:63-117`
- Impact: Invalid events are published to NATS and processed by consumers, wasting resources.
- Fix approach: Add explicit validation in IngestEventBatch() for required fields before publishing.

## Security Considerations

**Credentials Exposed in Default Environment Variables:**
- Risk: Database password, NATS credentials, S3 credentials set via environment variables with defaults visible in logs.
- Files: `internal/reaction/db/client.go:21-46`, NATS config, S3 config
- Current mitigation: Code logs non-sensitive values ("hive" is default user, but password printed in server logs).
- Recommendations: Never log credentials, use secrets management system, remove default passwords, validate that credentials are changed in production.

**Rate Limiting Bypasses:**
- Risk: Global rate limiter applies to all requests, but can be bypassed if multiple instances are deployed. No per-IP limiting.
- Files: `internal/gateway/middleware.go:166-184`
- Current mitigation: Single instance typical, but distributed setup has no coordination.
- Recommendations: Add distributed rate limiting (Redis-backed), per-IP limiting, API key based limits.

**CORS Configuration Too Permissive by Default:**
- Risk: Default CORS allows "*" origin which enables CSRF attacks.
- Files: `internal/gateway/config.go:38`
- Current mitigation: Environment-configurable, but default is unsafe.
- Recommendations: Change default to specific allowed origins, document secure configuration.

**No Request Authentication:**
- Risk: Event ingestion endpoint is unauthenticated. Any client can submit events.
- Files: `internal/gateway/server.go` (no auth middleware)
- Impact: Malicious actors can inject false events into the warehouse.
- Recommendations: Implement API key validation, client certificate validation, or HMAC signing on requests.

**Webhook Delivery Without HTTPS Validation:**
- Risk: Webhooks are sent to arbitrary URLs without certificate validation or HTTPS enforcement.
- Files: `internal/reaction/dispatcher.go:145` (HTTP client has no TLS config)
- Impact: Webhook data can be intercepted or delivered to wrong endpoints via DNS spoofing.
- Recommendations: Enforce HTTPS for webhook URLs, validate certificates, implement webhook signing with HMAC.

## Missing Critical Features

**No Event Deduplication:**
- Problem: No detection of duplicate events. If clients retry, duplicates are accepted.
- Blocks: Accurate event counting in analytics. Business logic depending on unique event sets.
- Suggested: Implement idempotency keys or deduplicate on consumer side based on ID + timestamp window.

**No Event Ordering Guarantees:**
- Problem: NATS JetStream provides delivery ordering per partition but no global ordering. Multi-partition streams have no ordering.
- Blocks: Time-sensitive event sequences cannot be guaranteed in order.
- Suggested: Enforce single partition or implement client-side reordering if ordering required.

**No Event Schema Validation:**
- Problem: Payload fields are not validated against expected schema beyond protobuf structure.
- Blocks: Cannot enforce application-specific field requirements (e.g., product_id must be present in ProductView).
- Suggested: Add JSONSchema validation on custom event payloads, or extend protobuf definitions.

**No Message Dead Letter Queue:**
- Problem: Failed messages are retried indefinitely or lost on NAK. No DLQ for poison messages.
- Blocks: Debugging of corrupt or unparseable events.
- Suggested: Implement DLQ consumer, log unparseable messages with full context.

**No Event Audit Trail:**
- Problem: No immutable log of all rule/anomaly decisions. Decisions are made but not recorded for audit.
- Blocks: Compliance, debugging, rule effectiveness analysis.
- Suggested: Write all rule matches and anomaly detections to separate audit stream.

**No Metrics or Observability:**
- Problem: No Prometheus metrics, traces, or dashboards for operational visibility.
- Blocks: Cannot monitor system health, identify bottlenecks, or debug issues in production.
- Suggested: Add Prometheus metrics (events/sec, latency, errors), distributed tracing, health check dashboards.

---

*Concerns audit: 2026-02-05*
