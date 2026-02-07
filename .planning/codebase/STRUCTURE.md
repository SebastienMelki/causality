# Codebase Structure

**Analysis Date:** 2026-02-05

## Directory Layout

```
causality/
├── cmd/                      # Service entry points (binaries)
│   ├── server/               # HTTP event ingestion gateway
│   ├── warehouse-sink/       # NATS→S3 batch processor
│   └── reaction-engine/      # Rule evaluation and anomaly detection
├── internal/                 # Private application packages
│   ├── events/               # Shared event categorization
│   ├── gateway/              # HTTP server and handlers
│   ├── nats/                 # NATS client and stream management
│   ├── warehouse/            # Parquet writing and S3 upload
│   └── reaction/             # Rule engine, dispatcher, anomaly detector
│       └── db/               # PostgreSQL persistence layer
├── pkg/proto/                # Generated protobuf code
│   └── causality/v1/         # Generated Go structs from .proto
├── proto/                    # Protocol buffer definitions
│   └── causality/v1/         # .proto source files
├── docker/                   # Containerization and compose configs
│   ├── hive/                 # Hive Metastore initialization
│   ├── postgres/             # PostgreSQL initialization scripts
│   ├── redash/               # Redash auto-setup scripts
│   └── trino/                # Trino catalog and configuration
├── sql/                      # Database initialization
│   └── hive/                 # Hive table creation (deprecated, in docker/hive/)
├── docs/                     # Documentation
├── api/openapi/              # OpenAPI specification
├── Dockerfile                # Multi-stage production image
├── docker-compose.yml        # Development environment definition
├── Makefile                  # Build, test, and dev commands
├── go.mod, go.sum            # Go module dependencies
├── buf.yaml, buf.lock        # Protocol buffer build config
└── CLAUDE.md                 # Project guidelines for Claude

```

## Directory Purposes

**cmd/**
- Purpose: Standalone service entry points - each cmd is a separate binary
- Contains: main.go files with config loading, initialization, signal handling
- Key files:
  - `cmd/server/main.go` - HTTP gateway startup
  - `cmd/warehouse-sink/main.go` - Warehouse consumer startup
  - `cmd/reaction-engine/main.go` - Reaction engine startup

**internal/events/**
- Purpose: Shared event type/category extraction and manipulation logic
- Contains: Event categorization enum constants, GetCategoryAndType() function
- Key files: `internal/events/category.go`

**internal/gateway/**
- Purpose: HTTP server, request handlers, middleware, event validation
- Contains: HTTP server setup, EventService (business logic), middleware chain
- Key files:
  - `internal/gateway/server.go` - HTTP server and handler registration
  - `internal/gateway/service.go` - EventService with IngestEvent/IngestEventBatch
  - `internal/gateway/middleware.go` - RequestID, Logging, Recovery, CORS, RateLimit, ContentType
  - `internal/gateway/config.go` - Configuration struct with environment binding
  - `internal/gateway/errors.go` - Sentinel errors (ErrEventRequired, ErrAtLeastOneEvent)

**internal/nats/**
- Purpose: NATS client abstraction, stream management, event publishing
- Contains: Client wrapper with JetStream, StreamManager, Publisher
- Key files:
  - `internal/nats/client.go` - NATS connection and JetStream wrapper
  - `internal/nats/stream.go` - StreamManager for ensuring stream and consumers exist
  - `internal/nats/publisher.go` - Publisher for sending events to stream
  - `internal/nats/config.go` - Configuration with URL, auth, timeouts
  - `internal/nats/errors.go` - Sentinel errors (ErrStreamNotFound, etc.)

**internal/warehouse/**
- Purpose: Event batching, Parquet serialization, S3 upload with Hive partitioning
- Contains: Consumer for NATS messages, Parquet writer, S3 client
- Key files:
  - `internal/warehouse/consumer.go` - Consumes NATS messages, batches, flushes on time/size
  - `internal/warehouse/parquet.go` - EventRow struct definition, proto→parquet conversion, Parquet writer
  - `internal/warehouse/s3.go` - S3 client for bucket operations and uploads
  - `internal/warehouse/config.go` - Batch config (MaxEvents, FlushInterval), S3 config
  - `internal/warehouse/errors.go` - Sentinel errors

**internal/reaction/**
- Purpose: Rule evaluation engine, webhook dispatch, anomaly detection
- Contains: Engine (rule matching), Dispatcher (webhook retry), AnomalyDetector (threshold/rate/count), Consumer
- Key files:
  - `internal/reaction/engine.go` - Rule matching with JSONPath evaluation, webhook queuing
  - `internal/reaction/consumer.go` - Consumes NATS events, processes through engine and anomaly detector
  - `internal/reaction/dispatcher.go` - Worker pool for webhook delivery with HMAC signing and retry backoff
  - `internal/reaction/anomaly.go` - Threshold/rate/count anomaly detection with background refresh
  - `internal/reaction/config.go` - Engine, Dispatcher, Anomaly configs
  - `internal/reaction/errors.go` - Sentinel errors

**internal/reaction/db/**
- Purpose: PostgreSQL persistence for rules, webhooks, deliveries, anomaly configs
- Contains: Database client, repositories with CRUD operations
- Key files:
  - `internal/reaction/db/client.go` - Database connection and initialization
  - `internal/reaction/db/rules.go` - Rule repository with GetEnabled(), Save(), Update()
  - `internal/reaction/db/webhooks.go` - Webhook repository (CRUD)
  - `internal/reaction/db/deliveries.go` - WebhookDelivery repository with status tracking
  - `internal/reaction/db/anomaly_configs.go` - AnomalyConfig repository with GetEnabled()

**pkg/proto/causality/v1/**
- Purpose: Generated Go code from .proto definitions (auto-generated by buf generate)
- Contains: Generated structs, message marshaling, service interfaces
- Note: DO NOT edit; regenerate via `make buf-generate`

**proto/causality/v1/**
- Purpose: Protocol buffer definitions for all messages and services
- Contains: EventEnvelope, service definitions (EventService), validation rules
- Key files: events.proto (EventEnvelope, DeviceContext, event types), service.proto (EventService RPC)

**docker/**
- Purpose: Service configurations and initialization scripts for development environment
- Contains:
  - `docker/hive/` - core-site.xml, init-schema.sql for Hive Metastore
  - `docker/postgres/` - init-reaction-engine.sql, init-redash.sql for PostgreSQL
  - `docker/redash/` - entrypoint.sh, init-admin.sh for auto-setup
  - `docker/trino/` - catalog configs, etc/, init-tables.sql

**sql/hive/**
- Purpose: SQL table definitions (deprecated; moved to docker/hive/)
- Contains: create_tables.sql for Hive schema

**docs/**
- Purpose: Additional documentation
- Contains: architecture.md

## Key File Locations

**Entry Points:**
- `cmd/server/main.go` - HTTP gateway entry point with config loading and graceful shutdown
- `cmd/warehouse-sink/main.go` - Warehouse consumer entry point
- `cmd/reaction-engine/main.go` - Reaction engine entry point with all component initialization

**Configuration:**
- `internal/gateway/config.go` - Gateway struct with Addr, ReadTimeout, WriteTimeout, CORS, RateLimit
- `internal/nats/config.go` - NATS struct with URL, Name, ReconnectWait, Stream config
- `internal/warehouse/config.go` - Warehouse struct with Batch, S3, Parquet configs
- `internal/reaction/config.go` - Reaction struct with Engine, Dispatcher, Anomaly, Database configs

**Core Logic:**
- `internal/gateway/service.go` - EventService: IngestEvent, IngestEventBatch, enrichment
- `internal/warehouse/consumer.go` - Consumer: fetch, batch, flush, partition, write to S3
- `internal/reaction/engine.go` - Engine: rule caching, matching, condition evaluation, webhook queueing
- `internal/reaction/dispatcher.go` - Dispatcher: worker pool, retry logic, HMAC signing

**Testing:**
- `internal/gateway/service_test.go` - Tests for EventService
- `internal/warehouse/parquet_test.go` - Tests for Parquet writing
- `internal/nats/publisher_test.go` - Tests for NATS publishing

## Naming Conventions

**Files:**
- Lowercase with underscores: `config.go`, `consumer.go`, `service.go`
- Main entry point: `main.go` in each cmd subdirectory
- Test files: `{module}_test.go` (e.g., `service_test.go`)

**Directories:**
- Lowercase: `cmd`, `internal`, `proto`, `docker`
- Package path matches Go package name (no dashes, lowercase)

**Functions:**
- Exported (public): PascalCase (e.g., `NewConsumer()`, `ProcessEvent()`)
- Unexported (private): camelCase (e.g., `processMessage()`, `flushTimer()`)

**Interfaces/Types:**
- PascalCase: `Consumer`, `Engine`, `Dispatcher`, `EventRow`, `Rule`

**Variables/Constants:**
- Exported: PascalCase (e.g., `CategoryUser`, `CategoryScreen`)
- Unexported: camelCase (e.g., `batch`, `stopCh`, `doneCh`)
- Config env var prefixes: ALL_CAPS with underscores (e.g., `LOG_LEVEL`, `GATEWAY_ADDR`, `NATS_URL`)

## Where to Add New Code

**New Feature (e.g., new event type or processor):**
- Add event type to `proto/causality/v1/events.proto`
- Update category extraction in `internal/events/category.go`
- If warehouse: update `internal/warehouse/parquet.go` EventRow struct
- If reaction: update rule matching in `internal/reaction/engine.go`
- Regenerate code: `make buf-generate`
- Write tests in corresponding `*_test.go`

**New Component/Module (e.g., new external integration):**
- Create new package under `internal/` (e.g., `internal/notification`)
- Follow pattern: Config struct + NewXXX constructor + Start/Stop lifecycle methods
- Integrate into appropriate cmd main.go (if new service) or existing service
- Add environment config variables in cmd main.go Config struct
- Test with `make test`

**Utilities (e.g., helper functions):**
- For shared logic: add to existing package (e.g., `events.SanitizeSubjectName()` in `internal/events`)
- For single-service logic: keep in service package
- Never export unless needed by multiple packages (prefer unexported + interfaces)

**Tests:**
- Co-located with source files: `{module}_test.go` in same directory
- Use table-driven tests for multiple cases
- Mock external dependencies (NATS, PostgreSQL) where practical

## Special Directories

**bin/**
- Purpose: Build output directory for compiled binaries
- Generated: Yes (by `make build`)
- Committed: No (gitignore)

**pkg/proto/**
- Purpose: Generated protobuf code (DO NOT edit manually)
- Generated: Yes (by `make buf-generate` from proto/)
- Committed: Yes (for distribution)

**docker-compose.yml**
- Purpose: Development environment specification with all services
- Services: NATS, MinIO, Trino, Hive Metastore, PostgreSQL, Redash
- Usage: `make docker-up` to start; `make docker-down` to stop

**Makefile**
- Purpose: Development command shortcuts
- Key targets:
  - `make dev` - Clean rebuild and start full environment
  - `make build` - Compile all services
  - `make test` - Run test suite
  - `make lint` - Run golangci-lint
  - `make fmt` - Format code
  - `make buf-generate` - Regenerate protobuf code
  - `make trino-cli` - Open Trino query interface

---

*Structure analysis: 2026-02-05*
