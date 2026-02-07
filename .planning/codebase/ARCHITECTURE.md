# Architecture

**Analysis Date:** 2026-02-05

## Pattern Overview

**Overall:** Multi-service event streaming pipeline with fan-out to parallel processors

**Key Characteristics:**
- Event-driven architecture with NATS JetStream as central hub
- Three independent microservices consuming from same event stream
- Stateless event processors with configurable batching/buffering
- PostgreSQL for persistent state (rules, webhooks, anomaly configs)
- Protocol Buffers for type-safe serialization across all boundaries

## Layers

**HTTP Gateway Layer:**
- Purpose: Accept incoming events from mobile/web clients and publish to NATS
- Location: `cmd/server` (main entry), `internal/gateway` (logic)
- Contains: HTTP server with middleware, request validation, event enrichment, NATS publisher
- Depends on: NATS JetStream, Protocol Buffers
- Used by: Client applications (iOS, Android, web)

**Event Stream Layer:**
- Purpose: Reliable event distribution and multiple consumer management
- Location: `internal/nats`
- Contains: NATS client wrapper, stream/consumer management, publisher, configuration
- Depends on: NATS JetStream library
- Used by: All three service entry points (server, warehouse-sink, reaction-engine)

**Warehouse/Analytics Layer:**
- Purpose: Batch events and persist to S3 in Parquet format with Hive partitioning
- Location: `cmd/warehouse-sink`, `internal/warehouse`
- Contains: Consumer, Parquet writer, S3 client, batch management with time/size triggers
- Depends on: NATS JetStream, MinIO/S3, Parquet library
- Used by: Trino, Redash for SQL querying

**Reaction/Rules Engine Layer:**
- Purpose: Evaluate events against user-defined rules and trigger webhooks/alerts
- Location: `cmd/reaction-engine`, `internal/reaction`
- Contains: Consumer, rule engine with JSONPath evaluation, webhook dispatcher, anomaly detector
- Depends on: NATS JetStream, PostgreSQL, HTTP client for webhooks
- Used by: External webhooks, NATS-based subscribers

**Shared Event Processing Layer:**
- Purpose: Common event categorization and data transformation
- Location: `internal/events`
- Contains: Event type/category extraction, subject name sanitization
- Depends on: Protocol Buffers
- Used by: All processors (warehouse, reaction)

**Persistent State Layer:**
- Purpose: Store and retrieve rules, webhooks, deliveries, and anomaly configs
- Location: `internal/reaction/db`
- Contains: Database client, repositories for rules/webhooks/deliveries/anomaly configs
- Depends on: PostgreSQL driver
- Used by: Reaction engine, dispatcher, anomaly detector

## Data Flow

**Event Ingestion Flow:**

1. Client sends HTTP POST to `{HOST}:8080/v1/events/ingest` or `/v1/events/batch`
2. `EventService.IngestEvent()` or `IngestEventBatch()` validates request
3. Envelope enriched with generated UUID v7 and server timestamp if missing
4. Event published to NATS JetStream stream via `Publisher.PublishEvent()`
5. Response returned to client (HTTP 200 with event_id)

**Warehouse Processing Flow:**

1. `warehouse.Consumer` fetches 100 messages at a time from JetStream with 5-second max wait
2. For each message, deserialize protobuf and batch in memory
3. Flush triggered by: batch reaching `Batch.MaxEvents` OR time interval `Batch.FlushInterval` expires
4. On flush: group events by partition key (app_id, year, month, day, hour)
5. For each partition: convert to `EventRow` structs, write to Parquet, upload to S3
6. ACK messages on successful processing; NAK on failure (automatic retry)
7. S3 path format: `s3://bucket/{app_id}/{year}/{month}/{day}/{hour}/batch-*.parquet`
8. Hive Metastore catalogs schema; Trino queries via Hive connector

**Reaction Processing Flow:**

1. `reaction.Consumer` fetches 100 messages at a time from JetStream
2. For each event, call `Engine.ProcessEvent(ctx, event)`
3. Engine loads cached rules (refreshed every `RuleRefreshInterval`)
4. For each rule: extract app_id, event category/type; evaluate filter match
5. For matching rules: evaluate all conditions using JSONPath extraction and operators (eq, ne, gt, contains, regex, in)
6. For matched rules: execute actions:
   - If webhook URLs: create delivery records in PostgreSQL with status=PENDING
   - If NATS subjects: publish enriched payload to subject (with app_id template substitution)
7. `Dispatcher` polls deliveries table in background, retries with exponential backoff
8. `AnomalyDetector` runs separate background task refreshing anomaly configs and detecting threshold/rate/count violations

**State Management:**

- Rules cached in memory with periodic refresh from PostgreSQL
- Webhooks and deliveries managed via database (source of truth)
- Anomaly configs cached with refresh interval
- No distributed state; each service maintains independent cache
- Graceful shutdown: close stopCh, wait for goroutines via doneCh, final flush/drain

## Key Abstractions

**EventEnvelope (Protobuf):**
- Purpose: Universal wrapper for all events with metadata and type-safe payload
- Examples: `UserLogin`, `ScreenView`, `PurchaseComplete`, `CustomEvent`
- Pattern: oneof payload discriminator in protobuf for type safety

**EventRow:**
- Purpose: Flattened structure for Parquet storage optimized for SQL analytics
- Files: `internal/warehouse/parquet.go`
- Pattern: Convert protobuf to row struct with partition columns extracted from timestamp

**Rule (Database Model):**
- Purpose: User-defined conditions that trigger actions
- Files: `internal/reaction/db/rules.go`
- Pattern: Stores AppID, EventCategory, EventType filters + JSONPath conditions + action webhooks

**Consumer Pattern:**
- Purpose: Generic NATS consumer with graceful shutdown
- Examples: `warehouse.Consumer`, `reaction.Consumer`
- Pattern: Fetch loop with context/signal handling, batching, error handling (ACK/NAK), doneCh sync

**Dispatcher Pattern:**
- Purpose: Worker pool for async tasks with retry logic
- Examples: `reaction.Dispatcher` for webhooks
- Pattern: Worker goroutines polling database, HMAC-SHA256 signing, exponential backoff

## Entry Points

**HTTP Server (`cmd/server/main.go`):**
- Location: `cmd/server/main.go`
- Triggers: Environment variables (LOG_LEVEL, LOG_FORMAT, gateway config, NATS config)
- Responsibilities:
  - Load config via caarlos0/env
  - Connect to NATS, ensure stream/consumers exist
  - Create HTTP server with middleware chain (RequestID, Logging, Recovery, CORS, RateLimit, ContentType)
  - Register protobuf service handlers via sebuf
  - Listen on `GATEWAY_ADDR` (default 0.0.0.0:8080)
  - Graceful shutdown on SIGINT/SIGTERM

**Warehouse Sink (`cmd/warehouse-sink/main.go`):**
- Location: `cmd/warehouse-sink/main.go`
- Triggers: Environment variables (NATS config, S3 config, LOG_LEVEL)
- Responsibilities:
  - Connect to NATS and S3/MinIO
  - Create consumer for warehouse-sink
  - Start Consumer.Start(ctx) for event batching and flushing
  - Block on sigCh until SIGINT/SIGTERM
  - Graceful shutdown: Consumer.Stop() for final flush, NATS Drain()

**Reaction Engine (`cmd/reaction-engine/main.go`):**
- Location: `cmd/reaction-engine/main.go`
- Triggers: Environment variables (NATS config, PostgreSQL config, LOG_LEVEL)
- Responsibilities:
  - Connect to NATS and PostgreSQL
  - Create repositories for rules, webhooks, deliveries, anomaly configs
  - Create and start: Engine (rule refresh loop), Dispatcher (webhook workers), AnomalyDetector (config refresh + detection)
  - Create Consumer for event processing
  - Start all components in sequence
  - Block on sigCh until SIGINT/SIGTERM
  - Graceful shutdown: stop Consumer, AnomalyDetector, Dispatcher, Engine, then NATS Drain()

## Error Handling

**Strategy:** Sentinel errors with custom wrappers; logged and propagated upstream

**Patterns:**

- **Protobuf unmarshaling errors:** Logged, message NAK'd for retry in warehouse/reaction consumers
- **Database errors:** Returned as wrapped fmt.Errorf, cause service startup to fail or operations to be logged
- **NATS connection errors:** Connection retry handled by NATS client (reconnect handlers), errors logged but service continues
- **HTTP delivery failures:** Stored as delivery record with retry tracking; exponential backoff with max attempts
- **Rule evaluation errors:** Logged per-rule but don't stop processing other rules
- **Partition write errors:** Logged per-partition in warehouse but other partitions continue

**Gateway responses:**
- HTTP 400 if validation fails (missing required fields)
- HTTP 500 if event publication fails
- HTTP 503 if NATS not ready (readiness check)

## Cross-Cutting Concerns

**Logging:**
- Framework: `log/slog` (standard library structured logging)
- Pattern: Each component has dedicated logger with component name
- Levels: DEBUG, INFO, WARN, ERROR configured via LOG_LEVEL env var
- Format: JSON or text via LOG_FORMAT env var

**Validation:**
- Framework: buf.validate (protobuf validation rules)
- Pattern: IngestEventRequest requires event; batch requires 1-1000 events
- Application: Enforced at gateway layer via EventService

**Authentication:**
- Approach: Header-based app_id for multi-tenancy (no per-request auth)
- Webhook signing: HMAC-SHA256 with shared secret stored in database
- Pattern: Client includes app_id in event; server isolates data by app_id in rules/anomaly configs

**Concurrency:**
- NATS consumer loop processes messages sequentially per consumer (default 1)
- Warehouse flushes synchronized via mutex on batch
- Reaction engine caches rules with RWMutex for thread-safe reads
- Dispatcher uses worker pool (configurable Workers count) for concurrent webhook delivery
- All main services use context.Context for cancellation and signal handling

---

*Architecture analysis: 2026-02-05*
